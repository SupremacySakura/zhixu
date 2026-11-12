package handler

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"
	"zhixu-backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type evolutionHandlerStore struct {
	service.EvolutionAlertStore
	resolved bool
	// stale 模拟两个标签页并发裁决：期望状态校验失败，仓储返回 ErrEvolutionConflict。
	stale bool
}

func (s *evolutionHandlerStore) List(_ context.Context, _ uint, filter repository.EvolutionAlertFilter) ([]model.EvolutionAlert, int64, error) {
	if filter.Type == model.EvolutionAlertType("dupe") {
		return nil, 0, nil
	}
	alert := model.EvolutionAlert{
		ID: 5, UserID: 2, AlertType: model.EvolutionAlertTypeDuplicate, Status: model.EvolutionAlertPending,
		Severity: model.EvolutionSeverityReview, SubjectIDs: "[10,11]",
		Payload:   `{"type":"duplicate","reason":"正文完全一致","evidence":[{"knowledge_id":10,"quote":"同一段正文"}],"suggested_action":"archive_one"}`,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	return []model.EvolutionAlert{alert}, 1, nil
}

func (s *evolutionHandlerStore) Get(_ context.Context, userID, id uint) (*model.EvolutionAlert, error) {
	if id != 5 || userID != 2 {
		return nil, gorm.ErrRecordNotFound
	}
	status := model.EvolutionAlertPending
	if s.resolved {
		status = model.EvolutionAlertResolved
	}
	return &model.EvolutionAlert{
		ID: 5, UserID: 2, AlertType: model.EvolutionAlertTypeDuplicate, Status: status,
		Severity: model.EvolutionSeverityReview, SubjectIDs: "[10,11]", Payload: `{}`,
	}, nil
}

func (s *evolutionHandlerStore) Summary(context.Context, uint) ([]repository.EvolutionAlertTypeCount, error) {
	return []repository.EvolutionAlertTypeCount{
		{Status: model.EvolutionAlertPending, AlertType: model.EvolutionAlertTypeDuplicate, Count: 3},
		{Status: model.EvolutionAlertDeferred, AlertType: model.EvolutionAlertTypeOutdated, Count: 2},
	}, nil
}

func (s *evolutionHandlerStore) Resolve(_ context.Context, _ uint, _ uint, action model.EvolutionAction, note string, _ model.EvolutionAlertStatus, apply func(*gorm.DB, *model.EvolutionAlert) error) (*model.EvolutionAlert, bool, error) {
	if s.stale {
		return nil, false, repository.ErrEvolutionConflict
	}
	if s.resolved {
		return &model.EvolutionAlert{ID: 5, UserID: 2, AlertType: model.EvolutionAlertTypeDuplicate, Status: model.EvolutionAlertResolved, SubjectIDs: "[10,11]", Payload: `{}`}, false, nil
	}
	alert := &model.EvolutionAlert{ID: 5, UserID: 2, AlertType: model.EvolutionAlertTypeDuplicate, SubjectIDs: "[10,11]", Payload: `{}`}
	if apply != nil {
		if err := apply(nil, alert); err != nil {
			return nil, false, err
		}
	}
	now := time.Now()
	alert.Status = model.EvolutionAlertResolved
	alert.ResolvedAt = &now
	alert.ResolvedAction = action
	alert.ResolutionNote = note
	return alert, true, nil
}

type evolutionHandlerKnowledge struct {
	service.EvolutionKnowledgeSource
}

func (evolutionHandlerKnowledge) Get(_ context.Context, userID, id uint) (*model.KnowledgeItem, error) {
	switch id {
	case 10:
		return &model.KnowledgeItem{ID: 10, UserID: userID, Title: "副本", Status: model.KnowledgeReady, Version: 2}, nil
	case 11:
		return &model.KnowledgeItem{ID: 11, UserID: userID, Title: "原件", Status: model.KnowledgeReady}, nil
	default:
		return nil, service.ErrKnowledgeNotFound
	}
}

func (evolutionHandlerKnowledge) ListByIDs(_ context.Context, userID uint, ids []uint) ([]model.KnowledgeItem, error) {
	out := make([]model.KnowledgeItem, 0, len(ids))
	for _, id := range ids {
		item, err := evolutionHandlerKnowledge{}.Get(context.Background(), userID, id)
		if err == nil {
			out = append(out, *item)
		}
	}
	return out, nil
}

type evolutionHandlerArchiver struct {
	archived []uint
}

func (a *evolutionHandlerArchiver) Archive(_ context.Context, _ uint, id uint) (*time.Time, error) {
	a.archived = append(a.archived, id)
	now := time.Now()
	return &now, nil
}

type evolutionHandlerTaskStore struct {
	service.AiTaskStore
	enqueued model.AiTask
}

func (s *evolutionHandlerTaskStore) Enqueue(_ context.Context, task model.AiTask) (*model.AiTask, error) {
	s.enqueued = task
	task.ID = 21
	task.Status = model.AiTaskPending
	return &task, nil
}

func serveEvolution(t *testing.T, user uint, method, path, body string, resolved, withTasks bool) *httptest.ResponseRecorder {
	return serveEvolutionWith(t, user, method, path, body, &evolutionHandlerStore{resolved: resolved}, withTasks)
}

func serveEvolutionWith(t *testing.T, user uint, method, path, body string, store *evolutionHandlerStore, withTasks bool) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if user != 0 {
		r.Use(func(c *gin.Context) { c.Set("user_id", user) })
	}
	svc := &service.EvolutionService{
		Alerts:    store,
		Knowledge: evolutionHandlerKnowledge{},
		Archiver:  &evolutionHandlerArchiver{},
	}
	if withTasks {
		svc.Tasks = service.NewAiTaskService(&evolutionHandlerTaskStore{})
	}
	h := &EvolutionHandler{Service: svc}
	r.GET("/evolution/alerts", h.List)
	r.GET("/evolution/alerts/summary", h.Summary)
	r.POST("/evolution/alerts/:id/resolve", h.Resolve)
	r.POST("/knowledge/:id/evolve/regenerate", h.Regenerate)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
	return w
}

// 期望状态对不上说明另一个标签页已经裁决过：必须回 409 让前端刷新，而不是 500。
func TestEvolutionHandlerResolveConflict(t *testing.T) {
	w := serveEvolutionWith(t, 2, "POST", "/evolution/alerts/5/resolve",
		`{"action":"keep_both","base_status":"pending"}`,
		&evolutionHandlerStore{stale: true}, false)
	require.Equal(t, 409, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "提醒状态已变化")
}

func TestEvolutionHandlerRoutes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		method   string
		path     string
		body     string
		user     uint
		status   int
		contains string
		tasks    bool
	}{
		{"unauthorized list", "GET", "/evolution/alerts", "", 0, 401, "", false},
		{"list alerts", "GET", "/evolution/alerts?open=true", "", 2, 200, "副本", false},
		{"list keeps payload reason", "GET", "/evolution/alerts", "", 2, 200, "正文完全一致", false},
		{"list rejects bad type", "GET", "/evolution/alerts?type=dupe", "", 2, 400, "", false},
		{"list rejects bad open", "GET", "/evolution/alerts?open=maybe", "", 2, 400, "", false},
		{"list rejects bad page", "GET", "/evolution/alerts?page=x", "", 2, 400, "", false},
		{"summary", "GET", "/evolution/alerts/summary", "", 2, 200, "\"pending\":3", false},
		{"summary includes deferred", "GET", "/evolution/alerts/summary", "", 2, 200, "\"deferred\":2", false},
		{"unauthorized summary", "GET", "/evolution/alerts/summary", "", 0, 401, "", false},
		{"resolve", "POST", "/evolution/alerts/5/resolve", `{"action":"keep_both","note":"都不删"}`, 2, 200, "都不删", false},
		{"resolve rejects mismatched action", "POST", "/evolution/alerts/5/resolve", `{"action":"accept_revision"}`, 2, 400, "不适用", false},
		{"resolve rejects unknown action", "POST", "/evolution/alerts/5/resolve", `{"action":"merge"}`, 2, 400, "", false},
		{"resolve missing alert", "POST", "/evolution/alerts/6/resolve", `{"action":"keep_both"}`, 2, 404, "演化提醒不存在", false},
		{"resolve invalid id", "POST", "/evolution/alerts/x/resolve", `{"action":"keep_both"}`, 2, 400, "", false},
		{"resolve bad json", "POST", "/evolution/alerts/5/resolve", `{`, 2, 400, "裁决参数无效", false},
		{"resolve idempotent", "POST", "/evolution/alerts/5/resolve", `{"action":"keep_both"}`, 2, 200, "resolved", false},
		{"regenerate unsupported", "POST", "/knowledge/10/evolve/regenerate", "", 2, 409, "演化检查未启用", false},
		{"regenerate enqueues", "POST", "/knowledge/10/evolve/regenerate", "", 2, 202, "pending", true},
		{"regenerate missing knowledge", "POST", "/knowledge/99/evolve/regenerate", "", 2, 404, "知识不存在", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := serveEvolution(t, tc.user, tc.method, tc.path, tc.body, tc.name == "resolve idempotent", tc.tasks)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			if tc.contains != "" {
				require.Contains(t, w.Body.String(), tc.contains)
			}
		})
	}
}
