package handler

import (
	"context"
	"encoding/json"
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

type knowledgeHandlerStore struct {
	service.KnowledgeStore
	existing bool
}

func (knowledgeHandlerStore) File(_ context.Context, user, id uint) (*model.FileModel, error) {
	if user != 2 || id != 1 {
		return nil, gorm.ErrRecordNotFound
	}
	return &model.FileModel{ID: 1, UserID: 2, Name: "note.md", FileHash: "aabbccdd"}, nil
}

func (f knowledgeHandlerStore) ByFile(context.Context, uint, uint) (*model.KnowledgeItem, error) {
	if !f.existing {
		return nil, gorm.ErrRecordNotFound
	}
	return &model.KnowledgeItem{ID: 10, Title: "原笔记", Status: model.KnowledgeReady}, nil
}

func (knowledgeHandlerStore) FolderName(context.Context, uint, uint) (string, error) {
	return "根目录", nil
}

func (knowledgeHandlerStore) Create(_ context.Context, item *model.KnowledgeItem) (*model.KnowledgeItem, bool, error) {
	item.ID = 10
	return item, true, nil
}

func (knowledgeHandlerStore) CreateManual(_ context.Context, item *model.KnowledgeItem) error {
	item.ID = 11
	return nil
}

func (knowledgeHandlerStore) Update(_ context.Context, user, id uint, title, content, sourceHash string, baseVersion uint) (*model.KnowledgeItem, error) {
	if user != 2 || id != 10 {
		return nil, gorm.ErrRecordNotFound
	}
	if baseVersion != 1 {
		return nil, repository.ErrKnowledgeConflict
	}
	return &model.KnowledgeItem{ID: 10, UserID: 2, Title: title, Content: content, Status: model.KnowledgeReady, Version: 2}, nil
}

func (knowledgeHandlerStore) Get(_ context.Context, user, id uint) (*model.KnowledgeItem, error) {
	if user != 2 || id != 10 {
		return nil, gorm.ErrRecordNotFound
	}
	return &model.KnowledgeItem{ID: 10, UserID: 2, Title: "笔记", Content: "# 内容", Status: model.KnowledgeReady}, nil
}

func (knowledgeHandlerStore) Delete(_ context.Context, user, id uint) error {
	if user != 2 || id != 10 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func TestKnowledgeDeleteHandler(t *testing.T) {
	for _, tc := range []struct {
		user   uint
		id     string
		status int
	}{
		{0, "10", 401}, {2, "10", 200}, {3, "10", 404}, {2, "invalid", 400},
	} {
		r := gin.New()
		if tc.user > 0 {
			r.Use(func(c *gin.Context) { c.Set("user_id", tc.user) })
		}
		h := &KnowledgeHandler{Service: service.NewKnowledgeService(knowledgeHandlerStore{}, "")}
		r.DELETE("/knowledge/:id", h.Delete)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("DELETE", "/knowledge/"+tc.id, nil))
		require.Equal(t, tc.status, w.Code)
	}
}

func TestKnowledgeHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &KnowledgeHandler{Service: service.NewKnowledgeService(knowledgeHandlerStore{}, "")}
	for _, tc := range []struct {
		name, method, path, body string
		user                     uint
		status                   int
	}{
		{"unauthorized", "GET", "/knowledge/10", "", 0, 401},
		{"detail", "GET", "/knowledge/10", "", 2, 200},
		{"other user", "GET", "/knowledge/10", "", 3, 404},
		{"invalid id", "GET", "/knowledge/no", "", 2, 400},
		{"zero id", "GET", "/knowledge/0", "", 2, 400},
		{"missing source", "POST", "/knowledge", `{}`, 2, 400},
		{"malformed JSON", "POST", "/knowledge", `{`, 2, 400},
		{"invalid pagination", "GET", "/knowledge?page=0", "", 2, 400},
		{"invalid size", "GET", "/knowledge?page_size=101", "", 2, 400},
		{"invalid status", "GET", "/knowledge?status=unsupported", "", 2, 400},
		{"non numeric page", "GET", "/knowledge?page=x", "", 2, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			if tc.user != 0 {
				r.Use(func(c *gin.Context) { c.Set("user_id", tc.user) })
			}
			r.GET("/knowledge/:id", h.Get)
			r.GET("/knowledge", h.List)
			r.POST("/knowledge", h.Add)
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			var payload map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
			if tc.status == 200 {
				data := payload["data"].(map[string]any)
				require.Equal(t, "# 内容", data["content"])
				require.NotContains(t, data, "user_id")
				require.NotContains(t, data, "source_hash")
			}
		})
	}
}

func TestKnowledgeHandlerAddAcceptedAndReuse(t *testing.T) {
	for _, existing := range []bool{false, true} {
		h := &KnowledgeHandler{Service: service.NewKnowledgeService(knowledgeHandlerStore{existing: existing}, "")}
		r := gin.New()
		r.Use(func(c *gin.Context) { c.Set("user_id", uint(2)) })
		r.POST("/knowledge", h.Add)
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/knowledge", strings.NewReader(`{"file_id":1,"title":"新标题"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if existing {
			require.Equal(t, 200, w.Code)
			require.Contains(t, w.Body.String(), "原笔记")
		} else {
			require.Equal(t, 202, w.Code)
			require.Contains(t, w.Body.String(), `"status":"pending"`)
		}
	}
}

func TestKnowledgeHandlerCreateManual(t *testing.T) {
	h := &KnowledgeHandler{Service: service.NewKnowledgeService(knowledgeHandlerStore{}, "")}
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"created", `{"title":"手写笔记","content":"# 正文"}`, 202},
		{"file_id and content conflict", `{"file_id":1,"title":"标题","content":"正文"}`, 400},
		{"missing title", `{"content":"正文"}`, 400},
		{"missing content", `{"title":"标题"}`, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.Use(func(c *gin.Context) { c.Set("user_id", uint(2)) })
			r.POST("/knowledge", h.Add)
			req := httptest.NewRequest("POST", "/knowledge", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			if tc.status == 202 {
				require.Contains(t, w.Body.String(), "手写笔记")
			}
		})
	}
}

func TestKnowledgeHandlerUpdate(t *testing.T) {
	h := &KnowledgeHandler{Service: service.NewKnowledgeService(knowledgeHandlerStore{}, "")}
	for _, tc := range []struct {
		name, path, body string
		user             uint
		status           int
	}{
		{"updated", "/knowledge/10", `{"title":"新标题","content":"# 新正文","base_version":1}`, 2, 200},
		{"missing base version", "/knowledge/10", `{"title":"新标题","content":"正文"}`, 2, 400},
		{"stale base version", "/knowledge/10", `{"title":"新标题","content":"正文","base_version":2}`, 2, 409},
		{"unknown id", "/knowledge/11", `{"title":"新标题","content":"正文","base_version":1}`, 2, 404},
		{"invalid id", "/knowledge/no", `{"title":"新标题","content":"正文","base_version":1}`, 2, 400},
		{"unauthorized", "/knowledge/10", `{"title":"新标题","content":"正文","base_version":1}`, 0, 401},
		{"malformed JSON", "/knowledge/10", `{`, 2, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			if tc.user != 0 {
				r.Use(func(c *gin.Context) { c.Set("user_id", tc.user) })
			}
			r.PUT("/knowledge/:id", h.Update)
			req := httptest.NewRequest("PUT", tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			if tc.status == 200 {
				require.Contains(t, w.Body.String(), "新标题")
				require.Contains(t, w.Body.String(), `"version":2`)
			}
		})
	}
}

type knowledgeVersionHandlerStore struct {
	knowledgeHandlerStore
}

func (knowledgeVersionHandlerStore) Get(_ context.Context, user, id uint) (*model.KnowledgeItem, error) {
	if user != 2 || id != 10 {
		return nil, gorm.ErrRecordNotFound
	}
	return &model.KnowledgeItem{ID: 10, UserID: 2, Title: "笔记", Content: "# 当前正文", Status: model.KnowledgeReady, Version: 2}, nil
}

func (knowledgeVersionHandlerStore) ListVersions(_ context.Context, user, id uint) ([]model.KnowledgeVersion, error) {
	if user != 2 || id != 10 {
		return nil, gorm.ErrRecordNotFound
	}
	return []model.KnowledgeVersion{
		{ID: 1, UserID: 2, KnowledgeID: 10, VersionNumber: 1, Title: "第一版", Content: "# 第一版正文", ChangeSource: model.KnowledgeChangeSourceManual, CreatedAt: time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)},
	}, nil
}

func (knowledgeVersionHandlerStore) GetVersion(_ context.Context, user, id, version uint) (*model.KnowledgeVersion, error) {
	if user != 2 || id != 10 || version != 1 {
		return nil, gorm.ErrRecordNotFound
	}
	return &model.KnowledgeVersion{ID: 1, UserID: 2, KnowledgeID: 10, VersionNumber: 1, Title: "第一版", Content: "# 第一版正文", ChangeSource: model.KnowledgeChangeSourceManual}, nil
}

func (knowledgeVersionHandlerStore) Update(_ context.Context, user, id uint, title, content, sourceHash string, baseVersion uint) (*model.KnowledgeItem, error) {
	if user != 2 || id != 10 || baseVersion != 2 {
		return nil, repository.ErrKnowledgeConflict
	}
	return &model.KnowledgeItem{ID: 10, UserID: 2, Title: title, Content: content, Status: model.KnowledgeReady, Version: 3}, nil
}

func TestKnowledgeHandlerVersions(t *testing.T) {
	h := &KnowledgeHandler{Service: service.NewKnowledgeService(knowledgeVersionHandlerStore{}, "")}
	for _, tc := range []struct {
		name, method, path string
		user               uint
		status             int
		contains           string
	}{
		{"list", "GET", "/knowledge/10/versions", 2, 200, `"current":true`},
		{"list unauthorized", "GET", "/knowledge/10/versions", 0, 401, ""},
		{"list unknown", "GET", "/knowledge/11/versions", 2, 404, ""},
		{"detail current", "GET", "/knowledge/10/versions/2", 2, 200, "# 当前正文"},
		{"detail snapshot", "GET", "/knowledge/10/versions/1", 2, 200, "# 第一版正文"},
		{"detail zero", "GET", "/knowledge/10/versions/0", 2, 400, ""},
		{"detail invalid", "GET", "/knowledge/10/versions/x", 2, 400, ""},
		{"detail missing", "GET", "/knowledge/10/versions/3", 2, 404, ""},
		{"restore", "POST", "/knowledge/10/versions/1/restore", 2, 200, `"version":3`},
		{"restore current", "POST", "/knowledge/10/versions/2/restore", 2, 200, `"version":2`},
		{"restore missing", "POST", "/knowledge/10/versions/3/restore", 2, 404, ""},
		{"restore zero", "POST", "/knowledge/10/versions/0/restore", 2, 400, ""},
		{"restore unauthorized", "POST", "/knowledge/10/versions/1/restore", 0, 401, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			if tc.user != 0 {
				r.Use(func(c *gin.Context) { c.Set("user_id", tc.user) })
			}
			r.GET("/knowledge/:id/versions", h.Versions)
			r.GET("/knowledge/:id/versions/:version", h.VersionDetail)
			r.POST("/knowledge/:id/versions/:version/restore", h.RestoreVersion)
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(""))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			if tc.contains != "" {
				require.Contains(t, w.Body.String(), tc.contains)
			}
		})
	}
}
