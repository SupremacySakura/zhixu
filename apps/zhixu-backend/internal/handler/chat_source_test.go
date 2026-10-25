package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type handlerSourceStore struct {
	saved   *model.ChatTurnSource
	deleted bool
	saveErr error
}

func (s *handlerSourceStore) SaveTurnSource(_ context.Context, row *model.ChatTurnSource) error {
	s.saved = row
	return s.saveErr
}
func (*handlerSourceStore) ListTurnSources(context.Context, string, uint) ([]model.ChatTurnSource, error) {
	return nil, nil
}
func (s *handlerSourceStore) IsDeleted(context.Context, string, uint) (bool, error) {
	return s.deleted, nil
}
func (*handlerSourceStore) MarkDeleted(context.Context, string, uint) error     { return nil }
func (*handlerSourceStore) CompleteCleanup(context.Context, string, uint) error { return nil }
func (*handlerSourceStore) PendingCleanups(context.Context) ([]model.ChatConversationState, error) {
	return nil, nil
}

func TestChatSendPersistsExactMessageSourceBeforeDone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			store := &handlerSourceStore{}
			if fail {
				store.saveErr = fmt.Errorf("database unavailable")
			}
			ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "Bearer internal-secret", r.Header.Get("Authorization"))
				w.Header().Set("Content-Type", "application/x-ndjson")
				_, _ = w.Write([]byte("{\"type\":\"sources\",\"query\":\"检索词\",\"items\":[{\"knowledge_id\":1,\"chunk_id\":2}]}\n{\"type\":\"done\",\"message_id\":\"assistant-rag-2\"}\n"))
			}))
			defer ai.Close()
			h := &ChatHandler{Service: &service.ChatService{Store: ragChatStore{}}, AIURL: ai.URL, InternalBackendURL: "http://127.0.0.1:9000", InternalAPISecret: "internal-secret", Rag: &service.RagSearcher{}, Conversations: &service.ConversationService{Sources: store}}
			r := gin.New()
			r.POST("/chat", func(c *gin.Context) { c.Set("user_id", uint(7)); h.Send(c) })
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("POST", "/chat", strings.NewReader(`{"rag":true,"conversation_id":"thread-1","messages":[{"role":"user","content":"回滚步骤"}]}`)))
			require.Equal(t, http.StatusOK, w.Code)
			require.NotNil(t, store.saved)
			require.Equal(t, "assistant-rag-2", store.saved.MessageID)
			require.Equal(t, "检索词", store.saved.Query)
			if fail {
				require.Contains(t, w.Body.String(), `"type":"error"`)
				require.NotContains(t, w.Body.String(), `"type":"done"`)
			} else {
				require.Contains(t, w.Body.String(), `"message_id":"assistant-rag-2"`)
			}
		})
	}
}

func TestChatSendRejectsDeletedConversationBeforeAI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ChatHandler{Service: &service.ChatService{Store: chatHandlerStore{}}, InternalAPISecret: "internal-secret", Conversations: &service.ConversationService{Sources: &handlerSourceStore{deleted: true}}}
	r := gin.New()
	r.POST("/chat", func(c *gin.Context) { c.Set("user_id", uint(7)); h.Send(c) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/chat", strings.NewReader(`{"conversation_id":"deleted","messages":[{"role":"user","content":"你好"}]}`)))
	require.Equal(t, http.StatusConflict, w.Code)
}
