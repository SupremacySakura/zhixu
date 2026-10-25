package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestEmbeddingGatewayFixedBatchModelAndCapability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, scenario := range []string{"valid", "changed text", "wrong dimension", "wrong endpoint", "private denied", "provider error", "duplicate index"} {
		t.Run(scenario, func(t *testing.T) {
			calls := 0
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				require.Equal(t, "/embeddings", r.URL.Path)
				require.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
				var body map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				require.Equal(t, "configured-model", body["model"])
				require.Equal(t, []any{"正文", "尾部"}, body["input"])
				require.NotContains(t, body, "tools")
				require.NotContains(t, body, "dimensions")
				w.Header().Set("Content-Type", "application/json")
				switch scenario {
				case "provider error":
					w.WriteHeader(401)
					_, _ = w.Write([]byte(`{"error":"secret leaked upstream"}`))
				case "wrong dimension":
					_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[1]},{"index":1,"embedding":[1]}]}`))
				case "duplicate index":
					_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[1,2]},{"index":0,"embedding":[3,4]}]}`))
				default:
					_, _ = w.Write([]byte(`{"data":[{"index":1,"embedding":[3,4]},{"index":0,"embedding":[1,2]}]}`))
				}
			}))
			defer provider.Close()
			s := &service.ChatService{AllowedPrivateEndpoints: provider.URL}
			if scenario == "private denied" {
				s.AllowedPrivateEndpoints = ""
			}
			token, revoke, err := s.Register(&service.ChatCapability{
				Context: context.Background(), Key: "secret", Texts: []string{"正文", "尾部"},
				Model: model.ProviderModel{Kind: "embedding", ModelID: "configured-model", Dimension: 2},
				Provider: model.ModelProvider{BaseURL: provider.URL, Protocol: "responses"},
			})
			require.NoError(t, err)
			defer revoke()
			router := gin.New()
			router.POST("/internal/chat/v1/*endpoint", (&ChatHandler{Service: s}).Gateway)
			input, endpoint := `{"input":["正文","尾部"],"model":"attacker","tools":[{}],"dimensions":1}`, "embeddings"
			expected := 200
			switch scenario {
			case "changed text":
				input, expected = `{"input":["另一个请求"]}`, 400
			case "wrong endpoint":
				endpoint, expected = "chat/completions", 404
			case "wrong dimension", "duplicate index":
				expected = 502
			case "private denied":
				expected = 403
			case "provider error":
				expected = 401
			}
			request := func() *httptest.ResponseRecorder {
				req := httptest.NewRequest("POST", "/internal/chat/v1/"+endpoint, strings.NewReader(input))
				req.Header.Set("Authorization", "Bearer "+token)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				return w
			}
			w := request()
			require.Equal(t, expected, w.Code)
			require.NotContains(t, w.Body.String(), "secret")
			require.Equal(t, 404, request().Code)
			if expected == 400 || expected == 403 || expected == 404 {
				require.Zero(t, calls)
			} else {
				require.Equal(t, 1, calls)
			}
		})
	}
}
