package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"zhixu-backend/internal/config"
	"zhixu-backend/internal/service"
	"zhixu-backend/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func TestServiceStatusRouterJWTAndEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); _, _ = w.Write([]byte(`{"status":"ok"}`)) }))
	defer upstream.Close()
	cfg := &config.Config{JWTSecret: "status-router-test-secret", ChatAIURL: upstream.URL, InternalAPISecret: "test-secret"}
	utils.InitJWT(cfg)
	r := gin.New()
	RegisterServiceStatusRouter(r, service.NewServiceStatusService(cfg))
	expired := jwt.NewWithClaims(jwt.SigningMethodHS256, utils.Claims{UserID: 1, RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour))}})
	expiredToken, _ := expired.SignedString([]byte(cfg.JWTSecret))
	for _, authorization := range []string{"", "invalid", "Bearer bad-token", "Bearer " + expiredToken} {
		req := httptest.NewRequest(http.MethodGet, "/settings/services/status", nil)
		req.Header.Set("Authorization", authorization)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected JWT 401, got %d", w.Code)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("unauthenticated request probed service")
	}
	token, err := utils.GenerateToken(1)
	if err != nil {
		t.Fatal(err)
	}
	// Caller-provided URLs are ignored rather than becoming an SSRF target.
	req := httptest.NewRequest(http.MethodGet, "/settings/services/status?url=http://localhost:1&mastra_url=http://localhost:2", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code, w.Header())
	}
	var body struct {
		Code int                       `json:"code"`
		Msg  string                    `json:"msg"`
		Data service.ServiceStatusView `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != 0 || body.Msg != "success" || len(body.Data.Services) != 2 || body.Data.Services[0].Status != "healthy" || body.Data.Services[1].Status != "unconfigured" {
		t.Fatal(w.Body.String())
	}
	var shape map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &shape)
	data := shape["data"].(map[string]any)
	items := data["services"].([]any)
	if items[1].(map[string]any)["latency_ms"] != nil {
		t.Fatal("unconfigured latency must be JSON null")
	}
	if calls.Load() != 1 {
		t.Fatal("unexpected upstream requests")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req = httptest.NewRequest(http.MethodGet, "/settings/services/status", nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Body.Len() != 0 {
		t.Fatal("canceled request wrote response")
	}
}
