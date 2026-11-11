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

type relationHandlerStore struct {
	service.RelationStore
}

func (relationHandlerStore) Create(_ context.Context, relation *model.KnowledgeRelation) (*model.KnowledgeRelation, error) {
	relation.ID = 41
	return relation, nil
}

func (relationHandlerStore) ListByKnowledge(_ context.Context, _, knowledgeID uint) ([]repository.RelationRecord, error) {
	rows := []repository.RelationRecord{{
		KnowledgeRelation: model.KnowledgeRelation{
			ID: 41, UserID: 2, SourceID: knowledgeID, TargetID: 9,
			RelationType: model.RelationTypeRelated, Status: model.RelationConfirmed,
			Source: model.RelationSourceManual, CreatedAt: time.Now(),
		},
		CounterpartID: 9, CounterpartTitle: "目标知识",
	}}
	if knowledgeID == 10 {
		rows = append(rows, repository.RelationRecord{
			KnowledgeRelation: model.KnowledgeRelation{
				ID: 42, UserID: 2, SourceID: 10, TargetID: 3,
				RelationType: model.RelationTypeSupplements, Status: model.RelationSuggested,
				Source: model.RelationSourceAI, Confidence: 0.72,
				Evidence:  "{\"reason\":\"互补\",\"source_quote\":\"甲\",\"target_quote\":\"乙\"}",
				CreatedAt: time.Now(),
			},
			CounterpartID: 3, CounterpartTitle: "源知识",
		})
	}
	return rows, nil
}

type relationHandlerTasks struct {
	service.RelationTaskSource
}

func (relationHandlerTasks) Latest(_ context.Context, _ uint, taskType string, targetID uint) (*model.AiTask, error) {
	if taskType == model.AiTaskTypeRelation && targetID == 10 {
		return &model.AiTask{
			ID: 7, UserID: 2, TaskType: model.AiTaskTypeRelation, TargetID: 10,
			Status: model.AiTaskFailed, ErrorMessage: "AI 关联调用超时，请稍后重试", RetryCount: 2, UpdatedAt: time.Now(),
		}, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (relationHandlerStore) Get(_ context.Context, _, id uint) (*repository.RelationRecord, error) {
	var status model.RelationStatus
	relationType := model.RelationTypeRelated
	switch id {
	case 42:
		status = model.RelationSuggested
	case 43:
		status = model.RelationConfirmed
	case 44:
		status = model.RelationRejected
	case 45:
		status = model.RelationConfirmed
		relationType = model.RelationTypeDependsOn
	default:
		return nil, gorm.ErrRecordNotFound
	}
	return &repository.RelationRecord{
		KnowledgeRelation: model.KnowledgeRelation{
			ID: id, UserID: 2, SourceID: 3, TargetID: 9, RelationType: relationType,
			Status: status, Source: model.RelationSourceAI, Confidence: 0.72,
		},
		CounterpartID: 9, CounterpartTitle: "目标知识",
	}, nil
}

func (relationHandlerStore) UpdateStatus(_ context.Context, _, _ uint, _ model.RelationStatus, _ *time.Time) error {
	return nil
}

func (relationHandlerStore) UpdateType(_ context.Context, _, id uint, relationType model.RelationType) (*model.KnowledgeRelation, error) {
	if id == 45 {
		return nil, repository.ErrRelationDuplicate
	}
	return &model.KnowledgeRelation{ID: id, SourceID: 3, TargetID: 9, RelationType: relationType}, nil
}

func (relationHandlerStore) Delete(_ context.Context, _, id uint) error {
	if id == 404 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (relationHandlerTasks) EnqueueRelationTask(_ context.Context, _, knowledgeID, _ uint) (*model.AiTask, error) {
	return &model.AiTask{
		ID: 8, UserID: 2, TaskType: model.AiTaskTypeRelation, TargetID: knowledgeID,
		Status: model.AiTaskPending, UpdatedAt: time.Now(),
	}, nil
}

type relationHandlerKnowledge struct {
	service.RelationKnowledgeStore
}

func (relationHandlerKnowledge) Get(_ context.Context, _ uint, id uint) (*model.KnowledgeItem, error) {
	switch id {
	case 3:
		return &model.KnowledgeItem{ID: id, UserID: 2, Title: "源知识"}, nil
	case 9, 10:
		return &model.KnowledgeItem{ID: id, UserID: 2, Title: "目标知识"}, nil
	case 88:
		now := time.Now()
		return &model.KnowledgeItem{ID: id, UserID: 2, Title: "已归档", ArchivedAt: &now}, nil
	default:
		return nil, gorm.ErrRecordNotFound
	}
}

func serveRelation(t *testing.T, user uint, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if user != 0 {
		r.Use(func(c *gin.Context) { c.Set("user_id", user) })
	}
	h := &RelationHandler{Service: service.NewRelationService(relationHandlerStore{}, relationHandlerKnowledge{}, relationHandlerTasks{})}
	r.GET("/knowledge/:id/relations", h.List)
	r.POST("/knowledge/:id/relations", h.Create)
	r.DELETE("/relations/:id", h.Delete)
	r.POST("/relations/:id/confirm", h.Confirm)
	r.POST("/relations/:id/reject", h.Reject)
	r.PUT("/relations/:id", h.Update)
	r.POST("/knowledge/:id/relations/regenerate", h.Regenerate)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
	return w
}

func TestRelationHandlerRoutes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		method   string
		path     string
		body     string
		user     uint
		status   int
		contains string
	}{
		{"unauthorized list", "GET", "/knowledge/3/relations", "", 0, 401, ""},
		{"list relations", "GET", "/knowledge/3/relations", "", 2, 200, "目标知识"},
		{"list keeps empty sections", "GET", "/knowledge/3/relations", "", 2, 200, `"suggested":[]`},
		{"list suggested relations", "GET", "/knowledge/10/relations", "", 2, 200, "\"confidence\":0.72"},
		{"list relation task", "GET", "/knowledge/10/relations", "", 2, 200, "AI 关联调用超时"},
		{"list foreign knowledge", "GET", "/knowledge/77/relations", "", 2, 404, "知识不存在"},
		{"list invalid knowledge id", "GET", "/knowledge/x/relations", "", 2, 400, ""},
		{"create relation", "POST", "/knowledge/3/relations", `{"target_id":9,"relation_type":"related"}`, 2, 201, "目标知识"},
		{"create invalid type", "POST", "/knowledge/3/relations", `{"target_id":9,"relation_type":"friend"}`, 2, 400, "关系类型无效"},
		{"create missing target", "POST", "/knowledge/3/relations", `{"target_id":0,"relation_type":"related"}`, 2, 400, "缺少目标知识"},
		{"create self relation", "POST", "/knowledge/3/relations", `{"target_id":3,"relation_type":"related"}`, 2, 400, "自身"},
		{"create archived target", "POST", "/knowledge/3/relations", `{"target_id":88,"relation_type":"related"}`, 2, 400, "已归档"},
		{"create foreign knowledge", "POST", "/knowledge/77/relations", `{"target_id":9,"relation_type":"related"}`, 2, 404, "知识不存在"},
		{"create foreign target", "POST", "/knowledge/3/relations", `{"target_id":77,"relation_type":"related"}`, 2, 404, "知识不存在"},
		{"create bad json", "POST", "/knowledge/3/relations", `{`, 2, 400, ""},
		{"delete relation", "DELETE", "/relations/41", "", 2, 200, ""},
		{"delete missing relation", "DELETE", "/relations/404", "", 2, 404, "关系不存在"},
		{"delete invalid relation id", "DELETE", "/relations/x", "", 2, 400, "关系ID无效"},
		{"unauthorized delete", "DELETE", "/relations/41", "", 0, 401, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := serveRelation(t, tc.user, tc.method, tc.path, tc.body)
			require.Equal(t, tc.status, w.Code)
			if tc.contains != "" {
				require.Contains(t, w.Body.String(), tc.contains)
			}
		})
	}
}
func TestRelationHandlerConfirmationRoutes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		method   string
		path     string
		body     string
		user     uint
		status   int
		contains string
	}{
		{"confirm suggested", "POST", "/relations/42/confirm", "", 2, 200, "\"status\":\"confirmed\""},
		{"confirm is idempotent", "POST", "/relations/43/confirm", "", 2, 200, "\"status\":\"confirmed\""},
		{"confirm rejected", "POST", "/relations/44/confirm", "", 2, 409, "已被忽略"},
		{"confirm missing", "POST", "/relations/404/confirm", "", 2, 404, "关系不存在"},
		{"confirm invalid id", "POST", "/relations/x/confirm", "", 2, 400, "关系ID无效"},
		{"confirm unauthorized", "POST", "/relations/42/confirm", "", 0, 401, ""},
		{"reject suggested", "POST", "/relations/42/reject", "", 2, 200, "\"status\":\"rejected\""},
		{"reject is idempotent", "POST", "/relations/44/reject", "", 2, 200, "\"status\":\"rejected\""},
		{"reject confirmed", "POST", "/relations/43/reject", "", 2, 409, "已确认"},
		{"reject missing", "POST", "/relations/404/reject", "", 2, 404, "关系不存在"},
		{"update type", "PUT", "/relations/43", `{"relation_type":"updates"}`, 2, 200, "\"relation_type\":\"updates\""},
		{"update invalid type", "PUT", "/relations/43", `{"relation_type":"friend"}`, 2, 400, "关系类型无效"},
		{"update requires confirmed", "PUT", "/relations/42", `{"relation_type":"updates"}`, 2, 409, "已确认"},
		{"update duplicate pair", "PUT", "/relations/45", `{"relation_type":"related"}`, 2, 409, "已存在"},
		{"update bad json", "PUT", "/relations/43", `{`, 2, 400, "关系参数无效"},
		{"update missing", "PUT", "/relations/404", `{"relation_type":"updates"}`, 2, 404, "关系不存在"},
		{"regenerate", "POST", "/knowledge/3/relations/regenerate", "", 2, 202, "\"task\""},
		{"regenerate queued", "POST", "/knowledge/3/relations/regenerate", "", 2, 202, "\"status\":\"pending\""},
		{"regenerate archived", "POST", "/knowledge/88/relations/regenerate", "", 2, 409, "归档"},
		{"regenerate foreign", "POST", "/knowledge/77/relations/regenerate", "", 2, 404, "知识不存在"},
		{"regenerate invalid id", "POST", "/knowledge/x/relations/regenerate", "", 2, 400, ""},
		{"regenerate unauthorized", "POST", "/knowledge/3/relations/regenerate", "", 0, 401, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := serveRelation(t, tc.user, tc.method, tc.path, tc.body)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			if tc.contains != "" {
				require.Contains(t, w.Body.String(), tc.contains)
			}
		})
	}
}
