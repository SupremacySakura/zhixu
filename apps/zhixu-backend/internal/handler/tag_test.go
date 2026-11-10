package handler

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"
	"zhixu-backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type tagHandlerStore struct {
	service.TagStore
}

func (tagHandlerStore) List(context.Context, uint) ([]repository.TagUsage, error) {
	return []repository.TagUsage{
		{Tag: model.Tag{ID: 1, Name: "工作"}, UsageCount: 2},
		{Tag: model.Tag{ID: 2, Name: "学习"}},
	}, nil
}

func (tagHandlerStore) FindOrCreate(_ context.Context, _ uint, name string) (*model.Tag, error) {
	return &model.Tag{ID: 6, Name: name}, nil
}

func (tagHandlerStore) Rename(_ context.Context, _ uint, id uint, name string) (*model.Tag, error) {
	switch id {
	case 6:
		return nil, repository.ErrTagNameConflict
	case 404:
		return nil, gorm.ErrRecordNotFound
	default:
		return &model.Tag{ID: id, Name: name}, nil
	}
}

func (tagHandlerStore) Delete(_ context.Context, _ uint, id uint) error {
	if id == 404 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (tagHandlerStore) TagsByKnowledge(_ context.Context, _ uint, knowledgeID uint) ([]model.Tag, error) {
	if knowledgeID == 99 {
		return nil, gorm.ErrRecordNotFound
	}
	return []model.Tag{{ID: 1, Name: "工作"}, {ID: 2, Name: "学习"}}, nil
}

func (tagHandlerStore) ReplaceForKnowledge(_ context.Context, _ uint, knowledgeID uint, names []string, _ string) ([]model.Tag, error) {
	if knowledgeID == 99 {
		return nil, gorm.ErrRecordNotFound
	}
	tags := make([]model.Tag, 0, len(names))
	for i, name := range names {
		tags = append(tags, model.Tag{ID: uint(i + 1), Name: name})
	}
	return tags, nil
}

func serveTag(t *testing.T, store service.TagStore, user uint, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if user != 0 {
		r.Use(func(c *gin.Context) { c.Set("user_id", user) })
	}
	h := &TagHandler{Service: service.NewTagService(store)}
	r.GET("/tags", h.List)
	r.POST("/tags", h.Create)
	r.PUT("/tags/:id", h.Rename)
	r.DELETE("/tags/:id", h.Delete)
	r.GET("/knowledge/:id/tags", h.KnowledgeTags)
	r.PUT("/knowledge/:id/tags", h.ReplaceKnowledgeTags)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
	return w
}

func TestTagHandlerRoutes(t *testing.T) {
	tooMany := `{"tags":[` + strings.Repeat(`"标签",`, service.TagMaxPerKnowledge) + `"标签"]}`
	store := tagHandlerStore{}
	for _, tc := range []struct {
		name     string
		method   string
		path     string
		body     string
		user     uint
		status   int
		contains string
	}{
		{"unauthorized list", "GET", "/tags", "", 0, 401, ""},
		{"list tags", "GET", "/tags", "", 2, 200, "工作"},
		{"list usage count", "GET", "/tags", "", 2, 200, "usage_count"},
		{"create tag", "POST", "/tags", `{"name":" 学习 "}`, 2, 201, "学习"},
		{"create invalid name", "POST", "/tags", `{"name":""}`, 2, 400, "标签名称"},
		{"create bad json", "POST", "/tags", `{`, 2, 400, ""},
		{"rename tag", "PUT", "/tags/5", `{"name":"新名字"}`, 2, 200, "新名字"},
		{"rename conflict", "PUT", "/tags/6", `{"name":"工作"}`, 2, 409, "标签名称已存在"},
		{"rename missing", "PUT", "/tags/404", `{"name":"工作"}`, 2, 404, "标签不存在"},
		{"rename invalid id", "PUT", "/tags/x", `{"name":"工作"}`, 2, 400, "标签ID无效"},
		{"delete tag", "DELETE", "/tags/5", "", 2, 200, ""},
		{"delete missing", "DELETE", "/tags/404", "", 2, 404, "标签不存在"},
		{"knowledge tags", "GET", "/knowledge/10/tags", "", 2, 200, "工作"},
		{"knowledge tags missing", "GET", "/knowledge/99/tags", "", 2, 404, "知识不存在"},
		{"knowledge tags invalid id", "GET", "/knowledge/x/tags", "", 2, 400, ""},
		{"replace tags", "PUT", "/knowledge/10/tags", `{"tags":["工作","学习"]}`, 2, 200, "学习"},
		{"replace tags too many", "PUT", "/knowledge/10/tags", tooMany, 2, 400, "最多绑定"},
		{"replace tags missing knowledge", "PUT", "/knowledge/99/tags", `{"tags":["工作"]}`, 2, 404, "知识不存在"},
		{"replace tags invalid name", "PUT", "/knowledge/10/tags", `{"tags":[""]}`, 2, 400, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := serveTag(t, store, tc.user, tc.method, tc.path, tc.body)
			require.Equal(t, tc.status, w.Code)
			if tc.contains != "" {
				require.Contains(t, w.Body.String(), tc.contains)
			}
		})
	}
}
