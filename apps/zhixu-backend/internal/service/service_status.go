package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"zhixu-backend/internal/config"
)

const (
	serviceProbeTimeout  = 3 * time.Second
	serviceStatusTTL     = 10 * time.Second
	serviceResponseLimit = 64 * 1024
)

type ServiceStatus struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	LatencyMS *int64 `json:"latency_ms"`
	Message   string `json:"message"`
}

type ServiceStatusView struct {
	CheckedAt string          `json:"checked_at"`
	Services  []ServiceStatus `json:"services"`
}

type statusFlight struct {
	done    chan struct{}
	cancel  context.CancelFunc
	waiters int
	result  ServiceStatusView
}

// ServiceStatusService owns an instance-local cache and immutable deployment targets.
// No user settings, database, model invocation or caller-provided URL are involved.
type ServiceStatusService struct {
	aiURL, secret, qdrantURL, apiKey string
	client                           *http.Client
	probeTimeout, cacheTTL           time.Duration
	mu                               sync.Mutex
	cached                           ServiceStatusView
	expires                          time.Time
	flight                           *statusFlight
}

func NewServiceStatusService(cfg *config.Config) *ServiceStatusService {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DisableCompression = true
	transport.MaxResponseHeaderBytes = 16 * 1024
	return &ServiceStatusService{
		aiURL: cfg.ChatAIURL, secret: cfg.InternalAPISecret,
		qdrantURL: cfg.QdrantURL, apiKey: cfg.QdrantAPIKey,
		probeTimeout: serviceProbeTimeout, cacheTTL: serviceStatusTTL,
		client: &http.Client{Transport: transport, Timeout: serviceProbeTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
	}
}

func (s *ServiceStatusService) Get(ctx context.Context) (ServiceStatusView, error) {
	if err := ctx.Err(); err != nil {
		return ServiceStatusView{}, err
	}
	s.mu.Lock()
	if time.Now().Before(s.expires) {
		result := copyStatusView(s.cached)
		s.mu.Unlock()
		return result, nil
	}
	f := s.flight
	if f == nil {
		// Sharing must not tie the probe to the first caller's cancellation.
		probeCtx, cancel := context.WithCancel(context.Background())
		f = &statusFlight{done: make(chan struct{}), cancel: cancel}
		s.flight = f
		go s.run(probeCtx, f)
	}
	f.waiters++
	s.mu.Unlock()
	defer s.leave(f)
	select {
	case <-ctx.Done():
		return ServiceStatusView{}, ctx.Err()
	case <-f.done:
		if err := ctx.Err(); err != nil {
			return ServiceStatusView{}, err
		}
		return copyStatusView(f.result), nil
	}
}

func (s *ServiceStatusService) leave(f *statusFlight) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f.waiters--
	if f.waiters == 0 && s.flight == f {
		s.flight = nil
		f.cancel()
	}
}

func (s *ServiceStatusService) run(ctx context.Context, f *statusFlight) {
	defer f.cancel()
	results := make([]ServiceStatus, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); results[0] = s.probe(ctx, "mastra", s.aiURL, s.secret) }()
	go func() { defer wg.Done(); results[1] = s.probe(ctx, "qdrant", s.qdrantURL, s.apiKey) }()
	wg.Wait()
	s.mu.Lock()
	defer s.mu.Unlock()
	f.result = ServiceStatusView{CheckedAt: time.Now().UTC().Format(time.RFC3339), Services: results}
	if s.flight == f {
		s.flight = nil
		if ctx.Err() == nil {
			s.cached = copyStatusView(f.result)
			s.expires = time.Now().Add(s.cacheTTL)
		}
	}
	close(f.done)
}

func copyStatusView(v ServiceStatusView) ServiceStatusView {
	out := ServiceStatusView{CheckedAt: v.CheckedAt, Services: append([]ServiceStatus(nil), v.Services...)}
	for i := range out.Services {
		if out.Services[i].LatencyMS != nil {
			n := *out.Services[i].LatencyMS
			out.Services[i].LatencyMS = &n
		}
	}
	return out
}

func statusTarget(raw string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" ||
		u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
		return "", false
	}
	return strings.TrimRight(u.String(), "/"), true
}

func (s *ServiceStatusService) probe(parent context.Context, id, rawURL, credential string) (result ServiceStatus) {
	result = ServiceStatus{ID: id, Status: "unconfigured", Message: "服务地址或凭据尚未正确配置"}
	target, ok := statusTarget(rawURL)
	if !ok || strings.ContainsAny(credential, "\r\n") || (id == "mastra" && strings.TrimSpace(credential) == "") {
		return
	}
	start := time.Now()
	defer func() { elapsed := time.Since(start).Milliseconds(); result.LatencyMS = &elapsed }()
	ctx, cancel := context.WithTimeout(parent, s.probeTimeout)
	defer cancel()
	paths := []string{"/internal/health"}
	if id == "qdrant" {
		paths = []string{"/readyz", "/collections"}
	}
	for _, path := range paths {
		data, status, err := s.request(ctx, target+path, id, credential)
		if err != nil {
			var netErr net.Error
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
				result.Status, result.Message = "timeout", "服务检查超时，请稍后重试"
			} else if status != 0 {
				result.Status, result.Message = "unhealthy", "服务响应异常，请联系部署管理员"
			} else {
				result.Status, result.Message = "unreachable", "无法连接服务，请检查部署状态"
			}
			return
		}
		if status == http.StatusUnauthorized || status == http.StatusForbidden {
			result.Status, result.Message = "unauthorized", "服务凭据验证失败，请联系部署管理员"
			return
		}
		if status != http.StatusOK {
			result.Status, result.Message = "unhealthy", "服务尚未就绪或响应异常"
			return
		}
		if id == "mastra" {
			var health struct {
				Status string `json:"status"`
			}
			if json.Unmarshal(data, &health) != nil || health.Status != "ok" {
				result.Status, result.Message = "unhealthy", "服务健康响应无效"
				return
			}
		} else if path == "/readyz" && strings.TrimSpace(string(data)) != "all shards are ready" {
			result.Status, result.Message = "unhealthy", "服务就绪响应无效"
			return
		} else if path == "/collections" && !validCollections(data) {
			result.Status, result.Message = "unhealthy", "服务凭据检查响应无效"
			return
		}
	}
	result.Status, result.Message = "healthy", "服务运行正常"
	return
}

func validCollections(data []byte) bool {
	var response struct {
		Status string `json:"status"`
		Result *struct {
			Collections *[]struct {
				Name string `json:"name"`
			} `json:"collections"`
		} `json:"result"`
	}
	if json.Unmarshal(data, &response) != nil || response.Status != "ok" || response.Result == nil || response.Result.Collections == nil {
		return false
	}
	for _, collection := range *response.Result.Collections {
		if strings.TrimSpace(collection.Name) == "" {
			return false
		}
	}
	return true
}

func (s *ServiceStatusService) request(ctx context.Context, target, id, credential string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json, text/plain")
	if id == "mastra" {
		req.Header.Set("Authorization", "Bearer "+credential)
	} else if credential != "" {
		req.Header.Set("api-key", credential)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	// Never parse or expose remote error bodies (which may contain secrets).
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, serviceResponseLimit+1))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if len(data) > serviceResponseLimit {
		return nil, resp.StatusCode, errors.New("service response exceeds limit")
	}
	return data, resp.StatusCode, nil
}
