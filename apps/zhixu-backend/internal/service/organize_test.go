package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"zhixu-backend/internal/model"

	"github.com/stretchr/testify/require"
)

func organizeTestCapability() *ChatCapability {
	return &ChatCapability{
		Model:    model.ProviderModel{ModelID: "gpt-4o-mini"},
		Provider: model.ModelProvider{Protocol: "chat", BaseURL: "https://api.example.com/v1"},
		Key:      "sk-test",
	}
}

func TestOrganizeClientSendsContractAndSanitizesDraft(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/organize", r.URL.Path)
		require.Equal(t, "Bearer internal-secret", r.Header.Get("Authorization"))
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(raw, &body))
		_, _ = w.Write([]byte(`{"draft":{"summary":"  一段摘要  ","keywords":["分块","分块","  ","索引"],"suggested_tags":["检索","检索","` + strings.Repeat("长", 31) + `"],"concepts":["RAG"]}}`))
	}))
	defer server.Close()
	client := &OrganizeClient{AIURL: server.URL + "/", Secret: "internal-secret"}
	tags := make([]string, 0, 60)
	for i := 0; i < 60; i++ {
		tags = append(tags, "标签"+strconv.Itoa(i))
	}
	draft, err := client.Organize(context.Background(), organizeTestCapability(), 7, 12, " 知识标题 ", strings.Repeat("正", OrganizeContentMaxRunes+50), tags)
	require.NoError(t, err)
	require.Equal(t, "一段摘要", draft.Summary)
	require.Equal(t, []string{"分块", "索引"}, draft.Keywords)
	// 31 字标签超过 TagNameMaxRunes，必须丢弃而不是截断。
	require.Equal(t, []string{"检索"}, draft.SuggestedTags)
	require.Equal(t, []string{"RAG"}, draft.Concepts)

	require.EqualValues(t, 1, body["version"])
	require.EqualValues(t, 7, body["taskId"])
	require.EqualValues(t, 12, body["knowledgeId"])
	require.Equal(t, "知识标题", body["title"])
	require.Equal(t, OrganizeContentMaxRunes, utf8.RuneCountInString(body["content"].(string)))
	require.Len(t, body["existingTags"], OrganizeExistingTagMax)
	wireModel, ok := body["model"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "chat", wireModel["protocol"])
	require.Equal(t, "gpt-4o-mini", wireModel["model_id"])
	require.Equal(t, "https://api.example.com/v1", wireModel["base_url"])
	require.Equal(t, "sk-test", wireModel["api_key"])
}

func TestOrganizeClientMapsFailureClasses(t *testing.T) {
	cases := []struct {
		name         string
		status       int
		body         string
		wantInvalid  bool
		wantRejected bool
	}{
		{"schema invalid", http.StatusUnprocessableEntity, `{"code":"schema_invalid"}`, true, false},
		{"bad request", http.StatusBadRequest, `{}`, false, true},
		{"forbidden", http.StatusForbidden, `{}`, false, true},
		{"upstream", http.StatusBadGateway, `{}`, false, false},
		{"timeout", http.StatusGatewayTimeout, `{}`, false, false},
		{"not json", http.StatusOK, "not json", false, false},
		{"empty summary", http.StatusOK, `{"draft":{"summary":"  ","keywords":["a"]}}`, true, false},
		{"no keywords", http.StatusOK, `{"draft":{"summary":"摘要","keywords":[]}}`, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			client := &OrganizeClient{AIURL: server.URL, Secret: "s"}
			_, err := client.Organize(context.Background(), organizeTestCapability(), 1, 2, "t", "正文", nil)
			require.Error(t, err)
			require.Equal(t, tc.wantInvalid, errors.Is(err, ErrOrganizeInvalidOutput))
			require.Equal(t, tc.wantRejected, errors.Is(err, ErrOrganizeRejected))
		})
	}
}

func TestOrganizeClientRejectsMissingConfig(t *testing.T) {
	client := &OrganizeClient{AIURL: "http://127.0.0.1:1", Secret: " "}
	_, err := client.Organize(context.Background(), organizeTestCapability(), 1, 2, "t", "正文", nil)
	require.ErrorContains(t, err, "INTERNAL_API_SECRET")

	_, err = client.Organize(context.Background(), nil, 1, 2, "t", "正文", nil)
	require.ErrorContains(t, err, "未配置")

	configured := &OrganizeClient{AIURL: "http://127.0.0.1:1", Secret: "s"}
	_, err = configured.Organize(context.Background(), organizeTestCapability(), 1, 2, "t", "  ", nil)
	require.ErrorIs(t, err, ErrOrganizeRejected)
}

type organizeFakeChat struct {
	cap *ChatCapability
	err error
}

func (f organizeFakeChat) ResolveChatModel(context.Context, uint) (*ChatCapability, error) {
	return f.cap, f.err
}

type organizeFakeKnowledge struct {
	item *model.KnowledgeItem
	err  error
}

func (f organizeFakeKnowledge) Get(context.Context, uint, uint) (*model.KnowledgeItem, error) {
	return f.item, f.err
}

type organizeFakeTags struct{ names []string }

func (f organizeFakeTags) List(context.Context, uint) ([]TagView, error) {
	views := make([]TagView, 0, len(f.names))
	for i, name := range f.names {
		views = append(views, TagView{ID: uint(i + 1), Name: name})
	}
	return views, nil
}

func newOrganizeTestServer(t *testing.T, received *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(raw, received))
		_, _ = w.Write([]byte(`{"draft":{"summary":"草稿摘要","keywords":["分块","检索"],"suggested_tags":["检索"],"concepts":["RAG"]}}`))
	}))
}

func TestOrganizeProcessorWritesDraftToPayload(t *testing.T) {
	var received map[string]any
	server := newOrganizeTestServer(t, &received)
	defer server.Close()
	processor := &OrganizeProcessor{
		Chat:      organizeFakeChat{cap: organizeTestCapability()},
		Knowledge: organizeFakeKnowledge{item: &model.KnowledgeItem{ID: 12, Title: "标题", Content: "正文内容", Status: model.KnowledgeReady}},
		Tags:      organizeFakeTags{names: []string{"检索", "推荐"}},
		Client:    &OrganizeClient{AIURL: server.URL, Secret: "s"},
	}
	payload, err := processor.Process(context.Background(), &model.AiTask{ID: 3, UserID: 2, TargetID: 12, TaskType: model.AiTaskTypeOrganize})
	require.NoError(t, err)
	require.Equal(t, model.AiTaskTypeOrganize, processor.TaskType())
	require.Equal(t, []any{"检索", "推荐"}, received["existingTags"])
	var draft OrganizeDraft
	require.NoError(t, json.Unmarshal(payload, &draft))
	require.Equal(t, "草稿摘要", draft.Summary)
	require.Equal(t, []string{"分块", "检索"}, draft.Keywords)
}

func TestOrganizeProcessorTerminalFailures(t *testing.T) {
	ready := &model.KnowledgeItem{ID: 12, Title: "标题", Content: "正文", Status: model.KnowledgeReady}
	cases := []struct {
		name      string
		chat      organizeChatResolver
		knowledge organizeKnowledgeSource
		want      string
	}{
		{"model missing", organizeFakeChat{err: errors.New("未配置可用的对话模型")}, organizeFakeKnowledge{item: ready}, "模型设置"},
		{"knowledge deleted", organizeFakeChat{cap: organizeTestCapability()}, organizeFakeKnowledge{err: ErrKnowledgeNotFound}, "不存在"},
		{"not ready", organizeFakeChat{cap: organizeTestCapability()}, organizeFakeKnowledge{item: &model.KnowledgeItem{ID: 12, Content: "正文", Status: model.KnowledgePending}}, "尚未就绪"},
		{"empty content", organizeFakeChat{cap: organizeTestCapability()}, organizeFakeKnowledge{item: &model.KnowledgeItem{ID: 12, Content: "   ", Status: model.KnowledgeReady}}, "尚未就绪"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			processor := &OrganizeProcessor{Chat: tc.chat, Knowledge: tc.knowledge, Client: &OrganizeClient{AIURL: "http://127.0.0.1:1", Secret: "s"}}
			_, err := processor.Process(context.Background(), &model.AiTask{ID: 3, UserID: 2, TargetID: 12})
			var terminal *AiTaskTerminalError
			require.True(t, errors.As(err, &terminal))
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestOrganizeProcessorKeepsUpstreamErrorsRetryable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()
	processor := &OrganizeProcessor{
		Chat:      organizeFakeChat{cap: organizeTestCapability()},
		Knowledge: organizeFakeKnowledge{item: &model.KnowledgeItem{ID: 12, Content: "正文", Status: model.KnowledgeReady}},
		Client:    &OrganizeClient{AIURL: server.URL, Secret: "s"},
	}
	_, err := processor.Process(context.Background(), &model.AiTask{ID: 3, UserID: 2, TargetID: 12})
	require.Error(t, err)
	var terminal *AiTaskTerminalError
	require.False(t, errors.As(err, &terminal), "上游故障必须可重试")
}

func TestKnowledgeOrganizeStateDraftOnlyAfterSuccess(t *testing.T) {
	item := &model.KnowledgeItem{ID: 5, UserID: 2, Title: "标题", Content: "正文", Status: model.KnowledgeReady, Keywords: `["分块","检索"]`}
	service := NewKnowledgeService(&knowledgeFake{item: item}, "")
	service.Tasks = NewAiTaskService(&aiTaskFakeStore{latest: &model.AiTask{
		ID: 9, TaskType: model.AiTaskTypeOrganize, TargetID: 5, Status: model.AiTaskSucceeded,
		Payload: `{"summary":"草稿摘要","keywords":["甲"],"suggested_tags":["乙"],"concepts":[]}`,
	}})
	view, draft, err := service.OrganizeState(context.Background(), 2, 5)
	require.NoError(t, err)
	require.Equal(t, uint(9), view.ID)
	require.Equal(t, model.AiTaskSucceeded, view.Status)
	require.Equal(t, "草稿摘要", draft.Summary)
	require.Equal(t, []string{"甲"}, draft.Keywords)
	require.Equal(t, []OrganizeTagView{{Name: "乙"}}, draft.SuggestedTags)
	require.False(t, draft.Accepted)
	// 未记录 base_version 的旧数据不做过期判断。
	require.False(t, draft.Stale)

	// 运行中的任务没有草稿，前端只能展示进度。
	service.Tasks = NewAiTaskService(&aiTaskFakeStore{latest: &model.AiTask{ID: 10, TargetID: 5, Status: model.AiTaskRunning, Payload: "{}"}})
	view, draft, err = service.OrganizeState(context.Background(), 2, 5)
	require.NoError(t, err)
	require.Nil(t, draft)
	require.Equal(t, model.AiTaskRunning, view.Status)

	// 关键词列在读取时解析为接口字段。
	got, err := service.Get(context.Background(), 2, 5)
	require.NoError(t, err)
	require.Equal(t, []string{"分块", "检索"}, got.KeywordList)
}
