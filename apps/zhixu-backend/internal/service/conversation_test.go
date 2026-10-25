package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"zhixu-backend/internal/model"

	"github.com/stretchr/testify/require"
)

type chatSourceStoreFake struct {
	rows        []model.ChatTurnSource
	saved       []model.ChatTurnSource
	lastID      string
	lastUsr     uint
	states      map[string]model.ChatConversationState
	markErr     error
	completeErr error
	deletedErr  error
}

func (f *chatSourceStoreFake) SaveTurnSource(_ context.Context, row *model.ChatTurnSource) error {
	if state := f.states[ResourceID(row.UserID)+":"+row.ConversationID]; state.Tombstoned {
		return ErrConversationDeleted
	}
	f.saved = append(f.saved, *row)
	return nil
}

func (f *chatSourceStoreFake) ListTurnSources(_ context.Context, conversationID string, user uint) ([]model.ChatTurnSource, error) {
	f.lastID, f.lastUsr = conversationID, user
	return f.rows, nil
}

func (f *chatSourceStoreFake) IsDeleted(_ context.Context, id string, user uint) (bool, error) {
	if f.deletedErr != nil {
		return false, f.deletedErr
	}
	return f.states[ResourceID(user)+":"+id].Tombstoned, nil
}

func (f *chatSourceStoreFake) MarkDeleted(_ context.Context, id string, user uint) error {
	if f.markErr != nil {
		return f.markErr
	}
	if f.states == nil {
		f.states = make(map[string]model.ChatConversationState)
	}
	key := ResourceID(user) + ":" + id
	state := model.ChatConversationState{UserID: user, ConversationID: id, Tombstoned: true, CleanupPending: true}
	f.states[key] = state
	f.rows = nil
	f.saved = nil
	return nil
}

func (f *chatSourceStoreFake) CompleteCleanup(_ context.Context, id string, user uint) error {
	if f.completeErr != nil {
		return f.completeErr
	}
	key := ResourceID(user) + ":" + id
	state := f.states[key]
	state.CleanupPending = false
	f.states[key] = state
	f.rows = nil
	f.saved = nil
	return nil
}

func (f *chatSourceStoreFake) PendingCleanups(_ context.Context) ([]model.ChatConversationState, error) {
	var rows []model.ChatConversationState
	for _, state := range f.states {
		if state.Tombstoned && state.CleanupPending {
			rows = append(rows, state)
		}
	}
	return rows, nil
}

func TestResourceID(t *testing.T) {
	require.Equal(t, "u42", ResourceID(42))
}

func TestRenderContent(t *testing.T) {
	t.Run("v2 parts", func(t *testing.T) {
		raw := json.RawMessage(`{"format":2,"parts":[{"type":"text","text":"你好"},{"type":"step-start"},{"type":"text","text":"世界"}]}`)
		require.Equal(t, "你好世界", renderContent(raw))
	})
	t.Run("fallback content", func(t *testing.T) {
		raw := json.RawMessage(`{"format":2,"parts":[],"content":"纯文本"}`)
		require.Equal(t, "纯文本", renderContent(raw))
	})
	t.Run("invalid json", func(t *testing.T) {
		require.Equal(t, "", renderContent(json.RawMessage(`not-json`)))
	})
}

func newConversationFixture(t *testing.T, handler http.HandlerFunc) (*ConversationService, *chatSourceStoreFake) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	sources := &chatSourceStoreFake{}
	return &ConversationService{AIURL: server.URL, Secret: "s3cret", Sources: sources}, sources
}

func TestConversationListSendsSecretAndResource(t *testing.T) {
	svc, _ := newConversationFixture(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/internal/conversations", r.URL.Path)
		require.Equal(t, "Bearer s3cret", r.Header.Get("Authorization"))
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "u7", body["resource"])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"conversations":[{"id":"c1","title":"发布流程","created_at":"2026-10-24T10:00:00Z","updated_at":"2026-10-24T10:05:00Z"}]}`))
	})

	list, err := svc.List(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "c1", list[0].ID)
	require.Equal(t, "发布流程", list[0].Title)
}

func TestConversationDetailPairsSourcesWithAssistantTurns(t *testing.T) {
	svc, sources := newConversationFixture(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/internal/conversations/messages", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"conversation":{"id":"c1","title":"t","created_at":"2026-10-24T10:00:00Z","updated_at":"2026-10-24T10:05:00Z"},
			"messages":[
				{"id":"m1","role":"user","content":{"format":2,"parts":[{"type":"text","text":"怎么发布？"}]},"created_at":"2026-10-24T10:00:00Z"},
				{"id":"m2","role":"assistant","content":{"format":2,"parts":[{"type":"text","text":"先构建"}]},"created_at":"2026-10-24T10:01:00Z"},
				{"id":"m3","role":"system","content":{"format":2,"parts":[{"type":"text","text":"忽略我"}]},"created_at":"2026-10-24T10:01:30Z"},
				{"id":"m4","role":"assistant","content":{"format":2,"parts":[{"type":"text","text":"再部署"}]},"created_at":"2026-10-24T10:02:00Z"}
			]}`))
	})
	sources.rows = []model.ChatTurnSource{
		{ConversationID: "c1", UserID: 7, MessageID: "m2", Query: "发布", Sources: `[{"knowledge_id":9,"title":"发布流程","section":"步骤","content":"构建产物","score":0.9}]`, CreatedAt: time.Now()},
		{ConversationID: "c1", UserID: 7, MessageID: "m4", Query: "部署", Sources: `[]`, CreatedAt: time.Now()},
	}

	detail, err := svc.Detail(context.Background(), 7, "c1")
	require.NoError(t, err)
	require.Equal(t, "c1", sources.lastID)
	require.Equal(t, uint(7), sources.lastUsr)
	require.Len(t, detail.Messages, 3) // system message skipped

	require.Equal(t, "user", detail.Messages[0].Role)
	require.Equal(t, "怎么发布？", detail.Messages[0].Content)
	require.Empty(t, detail.Messages[0].Sources)

	require.Equal(t, "assistant", detail.Messages[1].Role)
	require.Equal(t, "先构建", detail.Messages[1].Content)
	require.Equal(t, "发布", detail.Messages[1].Query)
	require.Len(t, detail.Messages[1].Sources, 1)
	require.Equal(t, "发布流程", detail.Messages[1].Sources[0].Title)

	require.Equal(t, "assistant", detail.Messages[2].Role)
	require.Equal(t, "部署", detail.Messages[2].Query)
	require.Empty(t, detail.Messages[2].Sources)
}

func TestConversationNotFound(t *testing.T) {
	svc, _ := newConversationFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	_, err := svc.Detail(context.Background(), 1, "missing")
	require.ErrorIs(t, err, errConversationNotFound)
}

func TestConversationRequiresSecret(t *testing.T) {
	svc := &ConversationService{AIURL: "http://127.0.0.1:1"}
	_, err := svc.List(context.Background(), 1)
	require.ErrorContains(t, err, "not configured")
}

func TestConversationMixedTurnsNeverShiftSources(t *testing.T) {
	svc, sources := newConversationFixture(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"conversation":{"id":"c1"},"messages":[
			{"id":"plain","role":"assistant","content":{"content":"plain"}},
			{"id":"failed","role":"assistant","content":{"content":"partial"}},
			{"id":"rag2","role":"assistant","content":{"content":"rag"}},
			{"id":"missing-save","role":"assistant","content":{"content":"missing"}},
			{"id":"rag1","role":"assistant","content":{"content":"rag1"}},
			{"role":"assistant","content":{"content":"no id"}}
		]}`))
	})
	sources.rows = []model.ChatTurnSource{
		{Query: "legacy", Sources: `[{"title":"wrong"}]`},
		{MessageID: "rag1", Query: "one", Sources: `[{"title":"one"}]`},
		{MessageID: "rag2", Query: "two", Sources: `[{"title":"two"}]`},
		{MessageID: "absent", Query: "orphan", Sources: `[{"title":"wrong"}]`},
	}
	detail, err := svc.Detail(context.Background(), 7, "c1")
	require.NoError(t, err)
	require.Len(t, detail.Messages, 6)
	for _, i := range []int{0, 1, 3, 5} {
		require.Empty(t, detail.Messages[i].Sources)
		require.Empty(t, detail.Messages[i].Query)
	}
	require.Equal(t, "two", detail.Messages[2].Sources[0].Title)
	require.Equal(t, "one", detail.Messages[4].Sources[0].Title)
}

func TestConversationDeletionRetryAndLateSave(t *testing.T) {
	calls := 0
	var sources *chatSourceStoreFake
	svc, store := newConversationFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, "/internal/conversations/delete", r.URL.Path)
		require.True(t, sources.states["u7:c1"].Tombstoned, "tombstone must commit before remote deletion")
		require.Empty(t, sources.saved)
		if calls == 1 {
			w.WriteHeader(503)
		} else {
			w.WriteHeader(404)
		}
	})
	sources = store
	sources.saved = []model.ChatTurnSource{{MessageID: "old"}}
	require.Error(t, svc.Delete(context.Background(), 7, "c1"))
	require.True(t, sources.states["u7:c1"].CleanupPending)
	require.ErrorIs(t, svc.SaveTurnSource(context.Background(), "c1", 7, "late", "query", nil), ErrConversationDeleted)
	// A fresh service instance reuses only persisted work, not an in-memory queue.
	restarted := &ConversationService{AIURL: svc.AIURL, Secret: svc.Secret, Sources: sources}
	require.NoError(t, restarted.RetryPendingCleanups(context.Background()))
	require.Equal(t, 2, calls)
	require.False(t, sources.states["u7:c1"].CleanupPending)
	require.True(t, sources.states["u7:c1"].Tombstoned)
	require.ErrorIs(t, restarted.EnsureActive(context.Background(), 7, "c1"), ErrConversationDeleted)
	require.NoError(t, restarted.EnsureActive(context.Background(), 8, "c1"))
	require.ErrorIs(t, restarted.SaveTurnSource(context.Background(), "c1", 7, "later", "q", nil), ErrConversationDeleted)
	require.NoError(t, restarted.Delete(context.Background(), 7, "c1"), "repeated 404 deletion is idempotent")
}

func TestConversationDeletionDoesNotCallRemoteOnMarkFailure(t *testing.T) {
	svc, sources := newConversationFixture(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("must not delete remotely before committing tombstone")
	})
	sources.markErr = errors.New("database unavailable")
	require.ErrorIs(t, svc.Delete(context.Background(), 7, "c1"), sources.markErr)
}

func TestConversationCleanupCompletionFailureRemainsPending(t *testing.T) {
	svc, sources := newConversationFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	sources.completeErr = errors.New("commit failed")
	require.ErrorIs(t, svc.Delete(context.Background(), 7, "c1"), sources.completeErr)
	require.True(t, sources.states["u7:c1"].CleanupPending)
	sources.completeErr = nil
	require.NoError(t, svc.RetryPendingCleanups(context.Background()))
	require.True(t, sources.states["u7:c1"].Tombstoned)
	require.False(t, sources.states["u7:c1"].CleanupPending)
}

func TestConversationDeletedThreadHiddenAndRejected(t *testing.T) {
	svc, sources := newConversationFixture(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/internal/conversations", r.URL.Path)
		_, _ = w.Write([]byte(`{"conversations":[{"id":"deleted"},{"id":"active"}]}`))
	})
	require.NoError(t, sources.MarkDeleted(context.Background(), "deleted", 7))
	list, err := svc.List(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "active", list[0].ID)
	_, err = svc.Detail(context.Background(), 7, "deleted")
	require.ErrorIs(t, err, ErrConversationDeleted)
	require.ErrorIs(t, svc.Rename(context.Background(), 7, "deleted", "new"), ErrConversationDeleted)
}

func TestConversationDetailRechecksDeletionAfterRemoteRead(t *testing.T) {
	var sources *chatSourceStoreFake
	svc, store := newConversationFixture(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, sources.MarkDeleted(context.Background(), "c1", 7))
		_, _ = w.Write([]byte(`{"conversation":{"id":"c1"},"messages":[]}`))
	})
	sources = store
	_, err := svc.Detail(context.Background(), 7, "c1")
	require.ErrorIs(t, err, ErrConversationDeleted)
}

func TestConversationTombstoneLookupFailureIsClosed(t *testing.T) {
	svc, sources := newConversationFixture(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/internal/conversations", r.URL.Path)
		_, _ = w.Write([]byte(`{"conversations":[{"id":"c1"}]}`))
	})
	sources.deletedErr = errors.New("lookup unavailable")
	list, err := svc.List(context.Background(), 7)
	require.ErrorIs(t, err, sources.deletedErr)
	require.Nil(t, list)
	require.ErrorIs(t, svc.EnsureActive(context.Background(), 7, "c1"), sources.deletedErr)
	_, err = svc.Detail(context.Background(), 7, "c1")
	require.ErrorIs(t, err, sources.deletedErr)
	require.ErrorIs(t, svc.Rename(context.Background(), 7, "c1", "new"), sources.deletedErr)
}

func TestSaveTurnSource(t *testing.T) {
	sources := &chatSourceStoreFake{}
	svc := &ConversationService{Sources: sources}

	require.NoError(t, svc.SaveTurnSource(context.Background(), "", 1, "m1", "q", nil))
	require.Empty(t, sources.saved, "empty conversation id must be a no-op")
	require.NoError(t, svc.SaveTurnSource(context.Background(), "c1", 1, "", "q", []model.KnowledgeSource{{Title: "no id"}}))
	require.NoError(t, svc.SaveTurnSource(context.Background(), "c1", 1, "  ", "q", nil))
	require.Empty(t, sources.saved, "missing message id must not store an unassociated citation")

	require.NoError(t, svc.SaveTurnSource(context.Background(), "c1", 5, "m9", "发布", []model.KnowledgeSource{{KnowledgeID: 9, Title: "发布流程"}}))
	require.Len(t, sources.saved, 1)
	require.Equal(t, "c1", sources.saved[0].ConversationID)
	require.Equal(t, uint(5), sources.saved[0].UserID)
	require.Contains(t, sources.saved[0].Sources, "发布流程")

	svcNoStore := &ConversationService{}
	require.NoError(t, svcNoStore.SaveTurnSource(context.Background(), "c1", 1, "m1", "q", nil))
}
