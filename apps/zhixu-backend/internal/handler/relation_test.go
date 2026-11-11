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
	return []repository.RelationRecord{{
		KnowledgeRelation: model.KnowledgeRelation{
			ID: 41, UserID: 2, SourceID: knowledgeID, TargetID: 9,
			RelationType: model.RelationTypeRelated, Status: model.RelationConfirmed,
			Source: model.RelationSourceManual, CreatedAt: time.Now(),
		},
		CounterpartID: 9, CounterpartTitle: "目标知识",
	}}, nil
}

func (relationHandlerStore) Delete(_ context.Context, _, id uint) error {
	if id == 404 {
		return gorm.ErrRecordNotFound
	}
	return nil
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
	h := &RelationHandler{Service: service.NewRelationService(relationHandlerStore{}, relationHandlerKnowledge{})}
	r.GET("/knowledge/:id/relations", h.List)
	r.POST("/knowledge/:id/relations", h.Create)
	r.DELETE("/relations/:id", h.Delete)
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
