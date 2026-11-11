package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"

	"github.com/stretchr/testify/require"
)

// organizeFakeTagger 记录采纳时追加的标签，并返回预设的标签视图。
type organizeFakeTagger struct {
	views       []TagView
	listErr     error
	attached    []TagView
	attachErr   error
	attachIDs   []uint
	attachNames []string
	attachCalls int
}

func (f *organizeFakeTagger) List(context.Context, uint) ([]TagView, error) {
	return f.views, f.listErr
}

func (f *organizeFakeTagger) AttachForKnowledge(_ context.Context, _, _ uint, tagIDs []uint, names []string) ([]TagView, error) {
	f.attachCalls++
	f.attachIDs, f.attachNames = tagIDs, names
	return f.attached, f.attachErr
}

func organizeDraftPayload() string {
	return `{"summary":"草稿摘要","keywords":["甲"],"suggested_tags":["乙","新概念"],"concepts":["RAG"],"base_version":3}`
}

func newAcceptOrganizeService(item *model.KnowledgeItem, payload string) (*KnowledgeService, *aiTaskFakeStore, *organizeFakeTagger) {
	store := &aiTaskFakeStore{latest: &model.AiTask{
		ID: 9, TaskType: model.AiTaskTypeOrganize, TargetID: item.ID, Status: model.AiTaskSucceeded, Payload: payload,
	}}
	service := NewKnowledgeService(&knowledgeFake{item: item}, "")
	service.Tasks = NewAiTaskService(store)
	tagger := &organizeFakeTagger{views: []TagView{{ID: 4, Name: "乙"}}}
	service.Tags = tagger
	return service, store, tagger
}

func TestAcceptOrganizeAppliesEditsAndAppendsTags(t *testing.T) {
	item := &model.KnowledgeItem{ID: 5, UserID: 2, Title: "标题", Content: "正文", Status: model.KnowledgeReady, Version: 3}
	service, store, tagger := newAcceptOrganizeService(item, organizeDraftPayload())
	accepted := "  用户改后的摘要  "
	keywords := []string{" 乙 ", "丙", "丙", "  "}
	tagger.attached = []TagView{{ID: 4, Name: "乙"}, {ID: 8, Name: "丙"}}

	updated, tags, draft, err := service.AcceptOrganize(context.Background(), 2, 5, OrganizeAcceptInput{
		Summary: &accepted, Keywords: &keywords, TagIDs: []uint{4}, TagNames: []string{"新概念"},
	})
	require.NoError(t, err)
	require.Equal(t, "用户改后的摘要", updated.Summary)
	require.Equal(t, `["乙","丙"]`, updated.Keywords)
	require.Len(t, tags, 2)
	// 采纳只追加标签，不经过会删除手工标签的替换逻辑。
	require.Equal(t, []uint{4}, tagger.attachIDs)
	require.Equal(t, []string{"新概念"}, tagger.attachNames)
	require.True(t, draft.Accepted)
	require.NotNil(t, draft.AcceptedAt)
	require.False(t, draft.Stale)
	// 正文与版本不受影响，草稿的 base_version 被保留。
	require.Equal(t, "正文", updated.Content)
	require.Equal(t, uint(3), updated.Version)
	require.Len(t, store.payloads, 1)
	require.Equal(t, uint(9), store.payloads[0].taskID)
	var persisted OrganizePayload
	require.NoError(t, json.Unmarshal([]byte(store.payloads[0].payload), &persisted))
	require.Equal(t, "用户改后的摘要", persisted.Summary)
	require.Equal(t, uint(3), persisted.BaseVersion)
	require.NotNil(t, persisted.AcceptedAt)
}

func TestAcceptOrganizeFallsBackToDraft(t *testing.T) {
	item := &model.KnowledgeItem{ID: 5, UserID: 2, Title: "标题", Content: "正文", Status: model.KnowledgeReady, Version: 3}
	service, _, tagger := newAcceptOrganizeService(item, organizeDraftPayload())

	updated, tags, draft, err := service.AcceptOrganize(context.Background(), 2, 5, OrganizeAcceptInput{})
	require.NoError(t, err)
	require.Equal(t, "草稿摘要", updated.Summary)
	require.Equal(t, `["甲"]`, updated.Keywords)
	require.Empty(t, tags)
	// 用户没有勾选标签时不应写入任何标签绑定。
	require.Zero(t, tagger.attachCalls)
	require.True(t, draft.Accepted)
	require.Equal(t, "草稿摘要", draft.Summary)
}

func TestAcceptOrganizeRejectsInvalidInput(t *testing.T) {
	tooLong := strings.Repeat("长", OrganizeSummaryMaxRunes+1)
	longKeyword := strings.Repeat("词", OrganizeKeywordMaxRunes+1)
	cases := []struct {
		name    string
		status  model.AiTaskStatus
		input   OrganizeAcceptInput
		wantErr error
	}{
		{name: "no draft", status: model.AiTaskRunning, wantErr: ErrOrganizeNoDraft},
		{name: "summary too long", status: model.AiTaskSucceeded, input: OrganizeAcceptInput{Summary: &tooLong}, wantErr: ErrKnowledgeInvalid},
		{name: "keyword too long", status: model.AiTaskSucceeded, input: OrganizeAcceptInput{Keywords: &[]string{longKeyword}}, wantErr: ErrKnowledgeInvalid},
		{name: "tags unavailable", status: model.AiTaskSucceeded, input: OrganizeAcceptInput{TagIDs: []uint{4}}, wantErr: ErrKnowledgeInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := &model.KnowledgeItem{ID: 5, UserID: 2, Title: "标题", Content: "正文", Status: model.KnowledgeReady, Version: 3}
			service, _, _ := newAcceptOrganizeService(item, organizeDraftPayload())
			service.Tasks = NewAiTaskService(&aiTaskFakeStore{latest: &model.AiTask{
				ID: 9, TaskType: model.AiTaskTypeOrganize, TargetID: 5, Status: tc.status, Payload: organizeDraftPayload(),
			}})
			if tc.name == "tags unavailable" {
				service.Tags = nil
			}
			updated, _, _, err := service.AcceptOrganize(context.Background(), 2, 5, tc.input)
			require.ErrorIs(t, err, tc.wantErr)
			require.Nil(t, updated)
			// 校验失败时正式字段保持原样，未被草稿污染。
			require.Empty(t, item.Summary)
			require.Empty(t, item.Keywords)
		})
	}
}

func TestAcceptOrganizePropagatesTagFailure(t *testing.T) {
	item := &model.KnowledgeItem{ID: 5, UserID: 2, Title: "标题", Content: "正文", Status: model.KnowledgeReady, Version: 3}
	service, store, tagger := newAcceptOrganizeService(item, organizeDraftPayload())
	tagger.attachErr = errors.New("标签写入失败")

	_, _, _, err := service.AcceptOrganize(context.Background(), 2, 5, OrganizeAcceptInput{TagNames: []string{"新概念"}})
	require.EqualError(t, err, "标签写入失败")
	// 标签失败时不写摘要，也不标 accepted_at，用户可以重试。
	require.Empty(t, item.Summary)
	require.Empty(t, store.payloads)
}

func TestOrganizeDraftMarksStaleAndResolvesExistingTags(t *testing.T) {
	item := &model.KnowledgeItem{ID: 5, UserID: 2, Title: "标题", Content: "正文", Status: model.KnowledgeReady, Version: 4}
	service, _, _ := newAcceptOrganizeService(item, organizeDraftPayload())

	_, draft, err := service.OrganizeState(context.Background(), 2, 5)
	require.NoError(t, err)
	// 正文已从版本 3 改到版本 4，草稿必须标为过期。
	require.True(t, draft.Stale)
	require.Equal(t, uint(3), draft.BaseVersion)
	require.Equal(t, []OrganizeTagView{
		{Name: "乙", TagID: 4, Existing: true},
		{Name: "新概念"},
	}, draft.SuggestedTags)
	require.Equal(t, []string{"RAG"}, draft.Concepts)
	require.False(t, draft.Accepted)
}

func TestRegenerateOrganizeRequiresReadyKnowledge(t *testing.T) {
	notReady := &model.KnowledgeItem{ID: 5, UserID: 2, Status: model.KnowledgePending, Content: "正文"}
	service := NewKnowledgeService(&knowledgeFake{item: notReady}, "")
	service.Tasks = NewAiTaskService(&aiTaskFakeStore{})
	_, _, err := service.RegenerateOrganize(context.Background(), 2, 5)
	require.ErrorIs(t, err, repository.ErrKnowledgeConflict)

	empty := &model.KnowledgeItem{ID: 5, UserID: 2, Status: model.KnowledgeReady, Content: "   "}
	service = NewKnowledgeService(&knowledgeFake{item: empty}, "")
	service.Tasks = NewAiTaskService(&aiTaskFakeStore{})
	_, _, err = service.RegenerateOrganize(context.Background(), 2, 5)
	require.ErrorIs(t, err, repository.ErrKnowledgeConflict)
}

func TestRegenerateOrganizeEnqueuesTask(t *testing.T) {
	item := &model.KnowledgeItem{ID: 5, UserID: 2, Title: "标题", Content: "正文", Status: model.KnowledgeReady, Version: 3}
	store := &aiTaskFakeStore{latest: &model.AiTask{
		ID: 11, TaskType: model.AiTaskTypeOrganize, TargetID: 5, Status: model.AiTaskPending, Payload: "{}",
	}}
	service := NewKnowledgeService(&knowledgeFake{item: item}, "")
	service.Tasks = NewAiTaskService(store)

	view, draft, err := service.RegenerateOrganize(context.Background(), 2, 5)
	require.NoError(t, err)
	require.Equal(t, model.AiTaskPending, view.Status)
	require.Nil(t, draft)
	require.Equal(t, model.AiTaskTypeOrganize, store.enqueued.TaskType)
	require.Equal(t, uint(5), store.enqueued.TargetID)
}
