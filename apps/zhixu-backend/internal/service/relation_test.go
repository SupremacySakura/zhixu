package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type relationFakeStore struct {
	created   *model.KnowledgeRelation
	rows      []repository.RelationRecord
	createErr error
	listErr   error
	deleteErr error
	deleted   []uint
}

func (f *relationFakeStore) Create(_ context.Context, relation *model.KnowledgeRelation) (*model.KnowledgeRelation, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	relation.ID = 41
	f.created = relation
	return relation, nil
}

func (f *relationFakeStore) ListByKnowledge(_ context.Context, _, _ uint) ([]repository.RelationRecord, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.rows, nil
}

func (f *relationFakeStore) Delete(_ context.Context, _, id uint) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, id)
	return nil
}

type relationFakeKnowledge struct{ items map[uint]*model.KnowledgeItem }

func (f *relationFakeKnowledge) Get(_ context.Context, _ uint, id uint) (*model.KnowledgeItem, error) {
	item, ok := f.items[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return item, nil
}

type relationFakeTasks struct {
	task  *model.AiTask
	err   error
	calls int
}

func (f *relationFakeTasks) Latest(_ context.Context, _ uint, _ string, _ uint) (*model.AiTask, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if f.task == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return f.task, nil
}

func newRelationFixture() (*RelationService, *relationFakeStore, *relationFakeKnowledge) {
	svc, store, knowledge, _ := newRelationFixtureWithTasks(nil)
	return svc, store, knowledge
}

func newRelationFixtureWithTasks(tasks RelationTaskSource) (*RelationService, *relationFakeStore, *relationFakeKnowledge, *relationFakeTasks) {
	store := &relationFakeStore{}
	knowledge := &relationFakeKnowledge{items: map[uint]*model.KnowledgeItem{
		3: {ID: 3, UserID: 2, Title: "源知识"},
		9: {ID: 9, UserID: 2, Title: "目标知识"},
	}}
	fakeTasks := &relationFakeTasks{}
	if tasks != nil {
		return NewRelationService(store, knowledge, tasks), store, knowledge, fakeTasks
	}
	return NewRelationService(store, knowledge, fakeTasks), store, knowledge, fakeTasks
}

// 手动创建直接落已确认 + manual，并回填对端信息。
func TestCreateRelationBuildsConfirmedManualView(t *testing.T) {
	svc, store, _ := newRelationFixture()

	view, err := svc.Create(context.Background(), 2, 3, RelationCreateInput{TargetID: 9, RelationType: model.RelationTypeRelated})

	require.NoError(t, err)
	require.Equal(t, uint(41), view.ID)
	require.Equal(t, model.RelationConfirmed, view.Status)
	require.Equal(t, model.RelationSourceManual, view.Source)
	require.Equal(t, model.RelationTypeRelated, view.RelationType)
	require.Equal(t, uint(9), view.CounterpartID)
	require.Equal(t, "目标知识", view.CounterpartTitle)
	require.Equal(t, uint(3), store.created.SourceID)
	require.Equal(t, uint(9), store.created.TargetID)
}

// 非法类型、缺目标、指向自身都算参数错误（400）。
func TestCreateRelationRejectsInvalidInput(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name  string
		input RelationCreateInput
		setup func(*relationFakeKnowledge)
	}{
		{name: "非法类型", input: RelationCreateInput{TargetID: 9, RelationType: "friend"}},
		{name: "缺目标", input: RelationCreateInput{RelationType: model.RelationTypeRelated}},
		{name: "指向自身", input: RelationCreateInput{TargetID: 3, RelationType: model.RelationTypeRelated}},
		{
			name:  "目标已归档",
			input: RelationCreateInput{TargetID: 9, RelationType: model.RelationTypeRelated},
			setup: func(k *relationFakeKnowledge) { k.items[9].ArchivedAt = &now },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, store, knowledge := newRelationFixture()
			if tc.setup != nil {
				tc.setup(knowledge)
			}
			_, err := svc.Create(context.Background(), 2, 3, tc.input)
			require.ErrorIs(t, err, ErrRelationInvalid)
			require.Nil(t, store.created)
		})
	}
}

// 源头或目标不属于当前用户时按不存在处理（404）。
func TestCreateRelationTreatsForeignKnowledgeAsNotFound(t *testing.T) {
	svc, _, knowledge := newRelationFixture()
	_, err := svc.Create(context.Background(), 2, 77, RelationCreateInput{TargetID: 9, RelationType: model.RelationTypeRelated})
	require.ErrorIs(t, err, ErrKnowledgeNotFound)

	delete(knowledge.items, 9)
	_, err = svc.Create(context.Background(), 2, 3, RelationCreateInput{TargetID: 9, RelationType: model.RelationTypeRelated})
	require.ErrorIs(t, err, ErrKnowledgeNotFound)
}

// 列表按状态分区：confirmed 进已确认区，suggested 进建议区并带上证据与置信度。
func TestListRelationsSplitsConfirmedAndSuggested(t *testing.T) {
	svc, store, _, tasks := newRelationFixtureWithTasks(nil)
	now := time.Date(2026, 5, 6, 7, 8, 9, 0, time.UTC)
	store.rows = []repository.RelationRecord{
		{KnowledgeRelation: model.KnowledgeRelation{ID: 1, UserID: 2, SourceID: 3, TargetID: 9, RelationType: model.RelationTypeDependsOn, Status: model.RelationConfirmed, Source: model.RelationSourceManual}, CounterpartID: 9, CounterpartTitle: "目标知识"},
		{KnowledgeRelation: model.KnowledgeRelation{ID: 2, UserID: 2, SourceID: 3, TargetID: 11, RelationType: model.RelationTypeRelated, Status: model.RelationSuggested, Source: model.RelationSourceAI, Confidence: 0.72, Evidence: "{\"reason\":\"主题相近\",\"source_quote\":\"甲\",\"target_quote\":\"乙\"}", CreatedAt: now}, CounterpartID: 11, CounterpartTitle: "待定知识"},
	}

	list, err := svc.List(context.Background(), 2, 3)

	require.NoError(t, err)
	require.Len(t, list.Confirmed, 1)
	require.Equal(t, uint(1), list.Confirmed[0].ID)
	require.Equal(t, "目标知识", list.Confirmed[0].CounterpartTitle)
	require.Len(t, list.Suggested, 1)
	suggested := list.Suggested[0]
	require.Equal(t, uint(2), suggested.ID)
	require.Equal(t, model.RelationTypeRelated, suggested.RelationType)
	require.Equal(t, model.RelationSuggested, suggested.Status)
	require.Equal(t, model.RelationSourceAI, suggested.Source)
	require.InDelta(t, 0.72, suggested.Confidence, 1e-9)
	require.Contains(t, suggested.Evidence, "主题相近")
	require.Equal(t, "待定知识", suggested.CounterpartTitle)
	require.Equal(t, now, suggested.CreatedAt)
	require.Nil(t, list.Task)
	require.Equal(t, 1, tasks.calls, "没有 relation 任务时也要查一次最新任务")
}

// 建议区为空时必须是空数组而不是 null，前端无需判空。
func TestListRelationsKeepsEmptySuggestedArray(t *testing.T) {
	svc, _, _, _ := newRelationFixtureWithTasks(nil)
	list, err := svc.List(context.Background(), 2, 3)
	require.NoError(t, err)
	require.NotNil(t, list.Suggested)
	require.Empty(t, list.Suggested)
}

// 最近一次 relation 任务的状态随列表下发，供建议区展示失败原因与重试次数。
func TestListRelationsExposesLatestRelationTask(t *testing.T) {
	svc, _, _, tasks := newRelationFixtureWithTasks(nil)
	now := time.Date(2026, 5, 6, 7, 8, 9, 0, time.UTC)
	tasks.task = &model.AiTask{
		ID: 7, UserID: 2, TaskType: model.AiTaskTypeRelation, TargetID: 3,
		Status: model.AiTaskFailed, ErrorMessage: "AI 关联调用超时，请稍后重试", RetryCount: 2, UpdatedAt: now,
	}

	list, err := svc.List(context.Background(), 2, 3)

	require.NoError(t, err)
	require.NotNil(t, list.Task)
	require.Equal(t, uint(7), list.Task.ID)
	require.Equal(t, model.AiTaskFailed, list.Task.Status)
	require.Equal(t, "AI 关联调用超时，请稍后重试", list.Task.ErrorMessage)
	require.EqualValues(t, 2, list.Task.RetryCount)
	require.Equal(t, now, list.Task.UpdatedAt)
}

func TestListRelationsPropagatesTaskLookupError(t *testing.T) {
	svc, _, _, tasks := newRelationFixtureWithTasks(nil)
	tasks.err = errors.New("db down")
	_, err := svc.List(context.Background(), 2, 3)
	require.ErrorContains(t, err, "db down")
}

func TestListRelationsRejectsForeignKnowledge(t *testing.T) {
	svc, _, _ := newRelationFixture()
	_, err := svc.List(context.Background(), 2, 77)
	require.ErrorIs(t, err, ErrKnowledgeNotFound)
}

func TestDeleteRelationMapsMissingToNotFound(t *testing.T) {
	svc, store, _ := newRelationFixture()
	store.deleteErr = gorm.ErrRecordNotFound
	require.ErrorIs(t, svc.Delete(context.Background(), 2, 41), ErrRelationNotFound)
}

func TestDeleteRelationRemovesOwnedRow(t *testing.T) {
	svc, store, _ := newRelationFixture()
	require.NoError(t, svc.Delete(context.Background(), 2, 41))
	require.Equal(t, []uint{41}, store.deleted)
}

func TestDeleteRelationPropagatesUnexpectedError(t *testing.T) {
	svc, store, _ := newRelationFixture()
	store.deleteErr = errors.New("db down")
	require.ErrorContains(t, svc.Delete(context.Background(), 2, 41), "db down")
}
