package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"zhixu-backend/internal/model"

	"github.com/stretchr/testify/require"
)

func relateSnapshot() *ChatCapability {
	return &ChatCapability{
		Model:    model.ProviderModel{ModelID: "deepseek-chat", Dimension: 3},
		Provider: model.ModelProvider{Protocol: "chat", BaseURL: "https://api.example.com/v1"},
		Key:      "sk-test",
	}
}

func relateCandidates() []RelateCandidate {
	return []RelateCandidate{{
		KnowledgeID: 21, Title: "候选标题", Excerpt: "候选摘要", Score: 0.82,
	}}
}

// 请求体必须带模型配置、任务/知识 ID 与候选；密钥只走 Bearer，不进 JSON。
func TestRelateClientSendsContract(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/relate", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "Bearer internal-secret", r.Header.Get("Authorization"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		_, _ = w.Write([]byte("{\"relations\":[{\"target_id\":21,\"relation_type\":\"related\",\"confidence\":0.78,\"evidence\":{\"reason\":\"主题相近\",\"source_quote\":\"源片段\",\"target_quote\":\"候选片段\"}}]}"))
	}))
	defer server.Close()

	relations, err := (&RelateClient{AIURL: server.URL, Secret: "internal-secret"}).Relate(
		context.Background(), relateSnapshot(), 8, 12, "源标题", "源摘要", relateCandidates(),
	)

	require.NoError(t, err)
	require.Len(t, relations, 1)
	require.Equal(t, uint(21), relations[0].TargetID)
	require.Equal(t, model.RelationTypeRelated, relations[0].RelationType)
	require.InDelta(t, 0.78, relations[0].Confidence, 1e-9)
	require.Equal(t, "主题相近", relations[0].Evidence.Reason)
	require.Equal(t, "源片段", relations[0].Evidence.SourceQuote)
	require.Equal(t, "候选片段", relations[0].Evidence.TargetQuote)

	require.EqualValues(t, 1, body["version"])
	require.EqualValues(t, 8, body["taskId"])
	require.EqualValues(t, 12, body["knowledgeId"])
	modelBody, ok := body["model"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "chat", modelBody["protocol"])
	require.Equal(t, "deepseek-chat", modelBody["model_id"])
	require.Equal(t, "https://api.example.com/v1", modelBody["base_url"])
	require.Equal(t, "sk-test", modelBody["api_key"])
	knowledge, ok := body["knowledge"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "源标题", knowledge["title"])
	require.Equal(t, "源摘要", knowledge["excerpt"])
	candidates, ok := body["candidates"].([]any)
	require.True(t, ok)
	require.Len(t, candidates, 1)
	first, ok := candidates[0].(map[string]any)
	require.True(t, ok)
	require.EqualValues(t, 21, first["knowledge_id"])
	require.Equal(t, "候选标题", first["title"])
	require.Equal(t, "候选摘要", first["excerpt"])
	require.InDelta(t, 0.82, first["score"], 1e-9)
	_, leaked := body["api_key"]
	require.False(t, leaked, "密钥不得出现在顶层")
}

// 422 表示模型输出不可用：调用方应终止任务而不是重试。
func TestRelateClientMapsSchemaInvalid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte("{\"code\":\"schema_invalid\"}"))
	}))
	defer server.Close()

	_, err := (&RelateClient{AIURL: server.URL, Secret: "s"}).Relate(
		context.Background(), relateSnapshot(), 8, 12, "标题", "摘要", relateCandidates(),
	)

	require.ErrorIs(t, err, ErrRelateInvalidOutput)
	require.NotErrorIs(t, err, ErrRelateRejected)
}

// 连接建立不了时是“不可达”，不能伪装成超时。
func TestRelateClientReportsTimeoutAsTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"relations":[]}`))
	}))
	defer server.Close()
	client := &RelateClient{AIURL: server.URL, Secret: "internal-secret"}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := client.Relate(ctx, relateSnapshot(), 5, 12, "源标题", "源摘录", []RelateCandidate{
		{KnowledgeID: 21, Title: "候选", Excerpt: "候选摘录", Score: 0.8},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "超时")
}

// 504 是可重试的瞬时错误，文案要让用户知道是超时。
func TestRelateClientMapsTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	defer server.Close()

	_, err := (&RelateClient{AIURL: server.URL, Secret: "s"}).Relate(
		context.Background(), relateSnapshot(), 8, 12, "标题", "摘要", relateCandidates(),
	)

	require.ErrorContains(t, err, "超时")
	require.NotErrorIs(t, err, ErrRelateInvalidOutput)
}

// 403/400 是配置或鉴权问题，归为被拒绝，任务终止。
func TestRelateClientMapsRejected(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusBadRequest} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
		}))
		_, err := (&RelateClient{AIURL: server.URL, Secret: "s"}).Relate(
			context.Background(), relateSnapshot(), 8, 12, "标题", "摘要", relateCandidates(),
		)
		server.Close()
		require.ErrorIs(t, err, ErrRelateRejected)
	}
}

// 5xx 是上游暂时不可用，保持普通错误以便退避重试。
func TestRelateClientMapsUpstreamFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	_, err := (&RelateClient{AIURL: server.URL, Secret: "s"}).Relate(
		context.Background(), relateSnapshot(), 8, 12, "标题", "摘要", relateCandidates(),
	)

	require.ErrorContains(t, err, "暂不可用")
	require.NotErrorIs(t, err, ErrRelateInvalidOutput)
}

func TestRelateClientRequiresConfiguration(t *testing.T) {
	_, err := (&RelateClient{}).Relate(context.Background(), nil, 8, 12, "标题", "摘要", nil)
	require.ErrorContains(t, err, "AI 关联未配置")

	_, err = (&RelateClient{AIURL: "http://127.0.0.1:9"}).Relate(
		context.Background(), relateSnapshot(), 8, 12, "标题", "摘要", relateCandidates(),
	)
	require.ErrorContains(t, err, "INTERNAL_API_SECRET")
}

func TestRelateClientReportsUnreachableService(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := server.URL
	server.Close()

	_, err := (&RelateClient{AIURL: url, Secret: "s"}).Relate(
		context.Background(), relateSnapshot(), 8, 12, "标题", "摘要", relateCandidates(),
	)

	require.ErrorContains(t, err, "AI 服务未启动或无法连接")
}

func TestRelateClientRejectsInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	_, err := (&RelateClient{AIURL: server.URL, Secret: "s"}).Relate(
		context.Background(), relateSnapshot(), 8, 12, "标题", "摘要", relateCandidates(),
	)

	require.ErrorContains(t, err, "无效的关联结果")
}
