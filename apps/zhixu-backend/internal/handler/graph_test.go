package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"zhixu-backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type graphHandlerReader struct {
	view   service.GraphView
	err    error
	calls  int
	userID uint
	query  service.GraphQuery
}

func (f *graphHandlerReader) Graph(_ context.Context, userID uint, query service.GraphQuery) (service.GraphView, error) {
	f.calls++
	f.userID = userID
	f.query = query
	return f.view, f.err
}

func serveGraph(t *testing.T, userID uint, target string, reader GraphReader) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", userID) })
	r.GET("/knowledge/graph", (&GraphHandler{Service: reader}).Get)
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
	return recorder
}

func decodeGraphBody(t *testing.T, recorder *httptest.ResponseRecorder) graphEnvelope {
	t.Helper()
	var body graphEnvelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	return body
}

type graphEnvelope struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Nodes []struct {
			ID         uint   `json:"id"`
			Title      string `json:"title"`
			SourceType string `json:"source_type"`
			Summary    string `json:"summary"`
			Degree     int    `json:"degree"`
		} `json:"nodes"`
		Edges []struct {
			ID           uint   `json:"id"`
			SourceID     uint   `json:"source_id"`
			TargetID     uint   `json:"target_id"`
			RelationType string `json:"relation_type"`
			Status       string `json:"status"`
			Source       string `json:"source"`
			Confidence   float64 `json:"confidence"`
		} `json:"edges"`
		Truncated bool `json:"truncated"`
	} `json:"data"`
}

func TestGraphHandlerRequiresLogin(t *testing.T) {
	reader := &graphHandlerReader{}
	recorder := serveGraph(t, 0, "/knowledge/graph", reader)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Zero(t, reader.calls)
}

// 无参数时默认 depth=1、不带建议边、不指定中心。
func TestGraphHandlerDefaults(t *testing.T) {
	reader := &graphHandlerReader{view: service.GraphView{Nodes: []service.GraphNode{}, Edges: []service.GraphEdge{}}}
	recorder := serveGraph(t, 2, "/knowledge/graph", reader)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 1, reader.calls)
	require.Equal(t, uint(2), reader.userID)
	require.Zero(t, reader.query.CenterID)
	require.False(t, reader.query.IncludeSuggested)
	body := decodeGraphBody(t, recorder)
	require.Zero(t, body.Code)
	require.Empty(t, body.Data.Nodes)
	require.Empty(t, body.Data.Edges)
	require.False(t, body.Data.Truncated)
	require.Contains(t, recorder.Body.String(), "\"nodes\":[]")
	require.Contains(t, recorder.Body.String(), "\"edges\":[]")
}

func TestGraphHandlerPassesCenterAndSuggested(t *testing.T) {
	reader := &graphHandlerReader{view: service.GraphView{
		Nodes: []service.GraphNode{{ID: 7, Title: "中心知识", SourceType: "markdown", Summary: "摘要", Degree: 1, UpdatedAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}},
		Edges: []service.GraphEdge{{ID: 11, SourceID: 7, TargetID: 8, RelationType: "depends_on", Status: "confirmed", Source: "manual"}},
	}}
	recorder := serveGraph(t, 2, "/knowledge/graph?center_id=7&depth=1&include_suggested=1", reader)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, uint(7), reader.query.CenterID)
	require.True(t, reader.query.IncludeSuggested)
	body := decodeGraphBody(t, recorder)
	require.Len(t, body.Data.Nodes, 1)
	require.Equal(t, "中心知识", body.Data.Nodes[0].Title)
	require.Equal(t, "markdown", body.Data.Nodes[0].SourceType)
	require.Equal(t, 1, body.Data.Nodes[0].Degree)
	require.Len(t, body.Data.Edges, 1)
	require.Equal(t, uint(7), body.Data.Edges[0].SourceID)
	require.Equal(t, uint(8), body.Data.Edges[0].TargetID)
	require.Equal(t, "depends_on", body.Data.Edges[0].RelationType)
}

// 深度本批固定为 1；大于 1 或其他值都必须拒绝，不能悄悄降级。
func TestGraphHandlerRejectsUnsupportedDepth(t *testing.T) {
	for _, depth := range []string{"2", "0", "-1", "abc"} {
		t.Run(depth, func(t *testing.T) {
			reader := &graphHandlerReader{}
			recorder := serveGraph(t, 2, "/knowledge/graph?depth="+depth, reader)
			require.Equal(t, http.StatusBadRequest, recorder.Code)
			require.Zero(t, reader.calls)
			require.Contains(t, recorder.Body.String(), "图谱深度仅支持 1")
		})
	}
}

func TestGraphHandlerRejectsInvalidQueries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		target string
		msg    string
	}{
		{"center is not a number", "/knowledge/graph?center_id=abc", "中心知识 ID 无效"},
		{"center is zero", "/knowledge/graph?center_id=0", "中心知识 ID 无效"},
		{"suggested flag is not boolean", "/knowledge/graph?include_suggested=yes", "建议边参数无效"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := &graphHandlerReader{}
			recorder := serveGraph(t, 2, tc.target, reader)
			require.Equal(t, http.StatusBadRequest, recorder.Code)
			require.Zero(t, reader.calls)
			require.Contains(t, recorder.Body.String(), tc.msg)
		})
	}
}

// 中心不存在、已归档或属于他人：服务统一返回知识不存在，接口映射 404。
func TestGraphHandlerMapsMissingCenterToNotFound(t *testing.T) {
	reader := &graphHandlerReader{err: service.ErrKnowledgeNotFound}
	recorder := serveGraph(t, 2, "/knowledge/graph?center_id=99", reader)
	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Body.String(), "知识不存在或已归档")
}

func TestGraphHandlerHidesInternalErrors(t *testing.T) {
	reader := &graphHandlerReader{err: errors.New("database down")}
	recorder := serveGraph(t, 2, "/knowledge/graph", reader)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	body := decodeGraphBody(t, recorder)
	require.Equal(t, 1004, body.Code)
	require.Contains(t, body.Msg, "图谱服务暂时不可用")
	require.NotContains(t, body.Msg, "database down")
}
