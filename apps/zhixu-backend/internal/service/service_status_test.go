package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"zhixu-backend/internal/config"
)

func statusService(aiURL, qdrantURL string) *ServiceStatusService {
	return NewServiceStatusService(&config.Config{ChatAIURL: aiURL, InternalAPISecret: "internal-test-secret", QdrantURL: qdrantURL, QdrantAPIKey: "qdrant-test-key"})
}

func TestServiceStatusHealthyReadOnly(t *testing.T) {
	var mu sync.Mutex
	paths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.RawQuery != "" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		switch r.URL.Path {
		case "/internal/health":
			if r.Header.Get("Authorization") != "Bearer internal-test-secret" || r.Header.Get("api-key") != "" {
				t.Error("invalid AI credentials")
			}
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/readyz", "/collections":
			if r.Header.Get("api-key") != "qdrant-test-key" || r.Header.Get("Authorization") != "" {
				t.Error("invalid Qdrant credentials")
			}
			if r.URL.Path == "/readyz" {
				_, _ = w.Write([]byte("all shards are ready"))
			} else {
				_, _ = w.Write([]byte(`{"status":"ok","result":{"collections":[{"name":"private-collection"}]}}`))
			}
		default:
			t.Error("unexpected path")
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	s := statusService(server.URL, server.URL)
	defer s.client.CloseIdleConnections()
	view, err := s.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := time.Parse(time.RFC3339, view.CheckedAt); err != nil {
		t.Fatal(err)
	}
	if len(view.Services) != 2 || view.Services[0].ID != "mastra" || view.Services[1].ID != "qdrant" {
		t.Fatalf("invalid services %+v", view)
	}
	for _, item := range view.Services {
		if item.Status != "healthy" || item.LatencyMS == nil || *item.LatencyMS < 0 {
			t.Fatalf("invalid result %+v", item)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(paths) != 3 {
		t.Fatalf("unexpected probes %v", paths)
	}
}

func TestServiceStatusUnconfigured(t *testing.T) {
	for _, raw := range []string{"", "ftp://localhost", "http://user:password@localhost", "http://localhost?url=secret", "http://localhost#secret", ":invalid"} {
		t.Run(raw, func(t *testing.T) {
			s := statusService(raw, raw)
			defer s.client.CloseIdleConnections()
			view, err := s.Get(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			for _, item := range view.Services {
				if item.Status != "unconfigured" || item.LatencyMS != nil {
					t.Fatalf("unexpected %+v", item)
				}
			}
		})
	}
	s := statusService("http://localhost:1", "")
	s.secret = ""
	view, _ := s.Get(context.Background())
	if view.Services[0].Status != "unconfigured" {
		t.Fatal(view)
	}
	s.client.CloseIdleConnections()
}

func TestServiceStatusProbeErrorsAreSafe(t *testing.T) {
	cases := []struct {
		name, id, path, body, want string
		code                       int
	}{
		{"ai401", "mastra", "/internal/health", "internal-test-secret", "unauthorized", 401},
		{"ai403", "mastra", "/internal/health", "internal-test-secret", "unauthorized", 403},
		{"ai404", "mastra", "/internal/health", "secret upstream", "unhealthy", 404},
		{"ai500", "mastra", "/internal/health", "secret upstream", "unhealthy", 500},
		{"aiHTML", "mastra", "/internal/health", "<html>secret</html>", "unhealthy", 200},
		{"aiEmpty", "mastra", "/internal/health", "", "unhealthy", 200},
		{"aiWrongStatus", "mastra", "/internal/health", `{"status":"error"}`, "unhealthy", 200},
		{"aiArray", "mastra", "/internal/health", `[]`, "unhealthy", 200},
		{"aiMultipleJSON", "mastra", "/internal/health", `{"status":"ok"}{"secret":1}`, "unhealthy", 200},
		{"aiOversize", "mastra", "/internal/health", strings.Repeat("x", serviceResponseLimit+1), "unhealthy", 200},
		{"qdrantNotReady", "qdrant", "/readyz", "secret upstream", "unhealthy", 503},
		{"qdrantEmptyReady", "qdrant", "/readyz", "", "unhealthy", 200},
		{"qdrantHTMLReady", "qdrant", "/readyz", "<html>secret</html>", "unhealthy", 200},
		{"qdrant401", "qdrant", "/collections", "qdrant-test-key", "unauthorized", 401},
		{"qdrant403", "qdrant", "/collections", "qdrant-test-key", "unauthorized", 403},
		{"qdrantHTML", "qdrant", "/collections", "<html>secret</html>", "unhealthy", 200},
		{"qdrantMissingResult", "qdrant", "/collections", `{"status":"ok"}`, "unhealthy", 200},
		{"qdrantMissingCollections", "qdrant", "/collections", `{"status":"ok","result":{}}`, "unhealthy", 200},
		{"qdrantNullCollections", "qdrant", "/collections", `{"status":"ok","result":{"collections":null}}`, "unhealthy", 200},
		{"qdrantWrongCollections", "qdrant", "/collections", `{"status":"ok","result":{"collections":{}}}`, "unhealthy", 200},
		{"qdrantInvalidName", "qdrant", "/collections", `{"status":"ok","result":{"collections":[{}]}}`, "unhealthy", 200},
		{"qdrantBadStatus", "qdrant", "/collections", `{"status":"error","result":{"collections":[]}}`, "unhealthy", 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Path == tc.path {
					w.WriteHeader(tc.code)
					_, _ = w.Write([]byte(tc.body))
				} else {
					_, _ = w.Write([]byte("all shards are ready"))
				}
			}))
			defer server.Close()
			s := statusService(server.URL, server.URL)
			defer s.client.CloseIdleConnections()
			item := s.probe(context.Background(), tc.id, server.URL, "internal-test-secret")
			if item.Status != tc.want || item.LatencyMS == nil {
				t.Fatalf("unexpected %+v", item)
			}
			if strings.Contains(item.Message, "secret") || strings.Contains(item.Message, server.URL) || strings.Contains(item.Message, tc.body) && tc.body != "" {
				t.Fatalf("unsafe message %q", item.Message)
			}
			if tc.name == "qdrantNotReady" && calls.Load() != 1 {
				t.Fatal("credential check continued after failed readiness")
			}
		})
	}
}

func TestServiceStatusUnreachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close()
	s := statusService(server.URL, server.URL)
	defer s.client.CloseIdleConnections()
	view, err := s.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range view.Services {
		if item.Status != "unreachable" || item.LatencyMS == nil {
			t.Fatal(item)
		}
	}
}

func TestServiceStatusNoRedirectOrProxy(t *testing.T) {
	var targetCalls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetCalls.Add(1)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL+"/stolen", 302) }))
	defer origin.Close()
	t.Setenv("HTTP_PROXY", target.URL)
	t.Setenv("HTTPS_PROXY", target.URL)
	t.Setenv("ALL_PROXY", target.URL)
	s := statusService(origin.URL, origin.URL)
	defer s.client.CloseIdleConnections()
	if s.client.Transport.(*http.Transport).Proxy != nil {
		t.Fatal("environment proxy enabled")
	}
	view, _ := s.Get(context.Background())
	for _, item := range view.Services {
		if item.Status != "unhealthy" {
			t.Fatal(item)
		}
	}
	if targetCalls.Load() != 0 {
		t.Fatal("followed redirect or proxy")
	}
}

func TestServiceStatusTimeoutParallel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	s := statusService(server.URL, server.URL)
	defer s.client.CloseIdleConnections()
	s.probeTimeout = 100 * time.Millisecond
	start := time.Now()
	view, err := s.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	for _, item := range view.Services {
		if item.Status != "timeout" {
			t.Fatal(item)
		}
	}
	if elapsed > 190*time.Millisecond {
		t.Fatalf("probes not parallel: %v", elapsed)
	}
}

func TestServiceStatusQdrantTotalTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timer := time.NewTimer(70 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-r.Context().Done():
			return
		case <-timer.C:
		}
		if r.URL.Path == "/readyz" {
			_, _ = w.Write([]byte("all shards are ready"))
		} else {
			_, _ = w.Write([]byte(`{"status":"ok","result":{"collections":[]}}`))
		}
	}))
	defer server.Close()
	s := statusService("", server.URL)
	defer s.client.CloseIdleConnections()
	s.probeTimeout = 100 * time.Millisecond
	view, _ := s.Get(context.Background())
	if view.Services[1].Status != "timeout" {
		t.Fatal("timeout should cover both requests", view)
	}
}

func TestServiceStatusCacheConcurrencyAndIsolation(t *testing.T) {
	var calls atomic.Int32
	gate := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		<-gate
		if r.URL.Path == "/collections" {
			_, _ = w.Write([]byte(`{"status":"ok","result":{"collections":[]}}`))
		} else if r.URL.Path == "/readyz" {
			_, _ = w.Write([]byte("all shards are ready"))
		} else {
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		}
	}))
	defer server.Close()
	s := statusService(server.URL, server.URL)
	defer s.client.CloseIdleConnections()
	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	results := make([]ServiceStatusView, n)
	for i := range n {
		go func() {
			defer wg.Done()
			v, err := s.Get(context.Background())
			if err != nil {
				t.Error(err)
			}
			results[i] = v
		}()
	}
	waitStatus(t, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.flight != nil && s.flight.waiters == n })
	close(gate)
	wg.Wait()
	if calls.Load() != 3 {
		t.Fatalf("not merged: %d requests", calls.Load())
	}
	results[0].Services[0].Status = "mutated"
	*results[0].Services[0].LatencyMS = -1
	view, _ := s.Get(context.Background())
	if view.Services[0].Status != "healthy" || *view.Services[0].LatencyMS < 0 || calls.Load() != 3 {
		t.Fatal("cache alias or miss", view)
	}
	if s.cacheTTL != 10*time.Second || s.probeTimeout != 3*time.Second {
		t.Fatal("invalid production limits")
	}
	s.mu.Lock()
	s.expires = time.Now().Add(-time.Second)
	s.mu.Unlock()
	_, _ = s.Get(context.Background())
	if calls.Load() != 6 {
		t.Fatal("cache did not expire")
	}
	other := statusService(server.URL, server.URL)
	defer other.client.CloseIdleConnections()
	_, _ = other.Get(context.Background())
	if calls.Load() != 9 {
		t.Fatal("cache unexpectedly global")
	}
}

func TestServiceStatusCancellationPreservesOtherWaiters(t *testing.T) {
	gate := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-gate:
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	s := statusService(server.URL, "")
	defer s.client.CloseIdleConnections()
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	second := make(chan ServiceStatusView, 1)
	go func() { _, err := s.Get(ctx); first <- err }()
	waitStatus(t, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.flight != nil && s.flight.waiters == 1 })
	go func() { v, _ := s.Get(context.Background()); second <- v }()
	waitStatus(t, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.flight != nil && s.flight.waiters == 2 })
	cancel()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	close(gate)
	if v := <-second; v.Services[0].Status != "healthy" {
		t.Fatal("first cancellation leaked", v)
	}
}

func TestServiceStatusLastCancellationAbortsAndDoesNotCache(t *testing.T) {
	started, aborted := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			close(started)
			<-r.Context().Done()
			close(aborted)
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()
	s := statusService(server.URL, "")
	defer s.client.CloseIdleConnections()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := s.Get(ctx); done <- err }()
	<-started
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	select {
	case <-aborted:
	case <-time.After(time.Second):
		t.Fatal("orphaned HTTP request")
	}
	view, _ := s.Get(context.Background())
	if calls.Load() != 2 || view.Services[0].Status != "healthy" {
		t.Fatal("canceled probe cached", view)
	}
	canceled, stop := context.WithCancel(context.Background())
	stop()
	if _, err := s.Get(canceled); !errors.Is(err, context.Canceled) {
		t.Fatal("pre-canceled caller used cache")
	}
}

func waitStatus(t *testing.T, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !predicate() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached")
		}
		time.Sleep(time.Millisecond)
	}
}
