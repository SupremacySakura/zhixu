package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type aiTaskSucceedCall struct {
	taskID  uint
	payload string
}

type aiTaskFailCall struct {
	taskID    uint
	message   string
	retryable bool
}

type aiTaskPayloadCall struct {
	userID  uint
	taskID  uint
	payload string
}

type aiTaskFakeStore struct {
	AiTaskStore
	enqueued   model.AiTask
	latest     *model.AiTask
	claim      *model.AiTask
	claimErr   error
	claimCalls int
	succeedErr error
	recycled   int
	succeeds   []aiTaskSucceedCall
	fails      []aiTaskFailCall
	payloads   []aiTaskPayloadCall
}

func (f *aiTaskFakeStore) Enqueue(_ context.Context, task model.AiTask) (*model.AiTask, error) {
	f.enqueued = task
	task.ID = 11
	task.Status = model.AiTaskPending
	return &task, nil
}

func (f *aiTaskFakeStore) Latest(context.Context, uint, string, uint) (*model.AiTask, error) {
	return f.latest, nil
}

func (f *aiTaskFakeStore) Claim(context.Context, string, time.Time) (*model.AiTask, error) {
	f.claimCalls++
	if f.claimErr != nil {
		return nil, f.claimErr
	}
	return f.claim, nil
}

func (f *aiTaskFakeStore) RecycleExpired(context.Context, time.Time) error {
	f.recycled++
	return nil
}

func (f *aiTaskFakeStore) Succeed(_ context.Context, task *model.AiTask, payload string) error {
	f.succeeds = append(f.succeeds, aiTaskSucceedCall{taskID: task.ID, payload: payload})
	return f.succeedErr
}

func (f *aiTaskFakeStore) Fail(_ context.Context, task *model.AiTask, message string, retryable bool, _ time.Time) error {
	f.fails = append(f.fails, aiTaskFailCall{taskID: task.ID, message: message, retryable: retryable})
	return nil
}

func (f *aiTaskFakeStore) UpdatePayload(_ context.Context, userID, taskID uint, payload string) error {
	f.payloads = append(f.payloads, aiTaskPayloadCall{userID: userID, taskID: taskID, payload: payload})
	return nil
}

type aiTaskFakeProcessor struct {
	taskType string
	payload  string
	err      error
	calls    int
}

func (p *aiTaskFakeProcessor) TaskType() string { return p.taskType }

func (p *aiTaskFakeProcessor) Process(context.Context, *model.AiTask) (json.RawMessage, error) {
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	return json.RawMessage(p.payload), nil
}

func newAiTaskFixture(taskType string) (*AiTaskService, *aiTaskFakeStore, *aiTaskFakeProcessor) {
	store := &aiTaskFakeStore{}
	processor := &aiTaskFakeProcessor{taskType: taskType, payload: `{"summary":"草稿"}`}
	store.claim = &model.AiTask{ID: 5, UserID: 2, TaskType: taskType, TargetID: 12, Status: model.AiTaskRunning}
	svc := NewAiTaskService(store).Register(processor)
	return svc, store, processor
}

func TestAiTaskServiceEnqueueSerializesPayload(t *testing.T) {
	svc, store, _ := newAiTaskFixture(model.AiTaskTypeOrganize)
	task, err := svc.Enqueue(context.Background(), 2, model.AiTaskTypeOrganize, 12, map[string]string{"trigger": "create"})
	require.NoError(t, err)
	require.Equal(t, `{"trigger":"create"}`, store.enqueued.Payload)
	require.Equal(t, model.AiTaskTypeOrganize, task.TaskType)

	_, err = svc.Enqueue(context.Background(), 2, model.AiTaskTypeOrganize, 12, nil)
	require.NoError(t, err)
	require.Equal(t, "{}", store.enqueued.Payload)
}

func TestAiTaskServiceProcessNextWithoutProcessors(t *testing.T) {
	store := &aiTaskFakeStore{}
	svc := NewAiTaskService(store)
	processed, err := svc.ProcessNext(context.Background())
	require.NoError(t, err)
	require.False(t, processed)
	// 没有处理器时绝不领取任务：任务停在 pending，不烧模型调用。
	require.Zero(t, store.claimCalls)
}

func TestAiTaskServiceProcessNextSkipsEmptyQueue(t *testing.T) {
	svc, store, _ := newAiTaskFixture(model.AiTaskTypeOrganize)
	store.claimErr = gorm.ErrRecordNotFound
	processed, err := svc.ProcessNext(context.Background())
	require.NoError(t, err)
	require.False(t, processed)
}

func TestAiTaskServiceProcessNextSucceeds(t *testing.T) {
	svc, store, processor := newAiTaskFixture(model.AiTaskTypeOrganize)
	processed, err := svc.ProcessNext(context.Background())
	require.NoError(t, err)
	require.True(t, processed)
	require.Equal(t, 1, processor.calls)
	require.Len(t, store.succeeds, 1)
	require.Equal(t, uint(5), store.succeeds[0].taskID)
	require.JSONEq(t, `{"summary":"草稿"}`, store.succeeds[0].payload)
	require.Empty(t, store.fails)
}

func TestAiTaskServiceProcessNextUnknownTypeFailsPermanently(t *testing.T) {
	svc, store, _ := newAiTaskFixture(model.AiTaskTypeOrganize)
	store.claim.TaskType = model.AiTaskTypeRelation
	processed, err := svc.ProcessNext(context.Background())
	require.NoError(t, err)
	require.True(t, processed)
	require.Len(t, store.fails, 1)
	require.False(t, store.fails[0].retryable)
	require.Contains(t, store.fails[0].message, "尚未启用")
}

func TestAiTaskServiceProcessNextTerminalErrorIsNotRetried(t *testing.T) {
	svc, store, processor := newAiTaskFixture(model.AiTaskTypeOrganize)
	processor.err = TerminalTaskError("未配置可用的对话模型")
	processed, err := svc.ProcessNext(context.Background())
	require.NoError(t, err)
	require.True(t, processed)
	require.Len(t, store.fails, 1)
	require.False(t, store.fails[0].retryable)
	require.Equal(t, "未配置可用的对话模型", store.fails[0].message)
}

func TestAiTaskServiceProcessNextTransientErrorRetries(t *testing.T) {
	svc, store, processor := newAiTaskFixture(model.AiTaskTypeOrganize)
	processor.err = errors.New("connection refused")
	processed, err := svc.ProcessNext(context.Background())
	require.NoError(t, err)
	require.True(t, processed)
	require.Len(t, store.fails, 1)
	require.True(t, store.fails[0].retryable)
	require.Contains(t, store.fails[0].message, "connection refused")
}

func TestAiTaskServiceProcessNextLostLeaseIsNotAnError(t *testing.T) {
	svc, store, _ := newAiTaskFixture(model.AiTaskTypeOrganize)
	store.succeedErr = repository.ErrAiTaskLeaseLost
	processed, err := svc.ProcessNext(context.Background())
	require.NoError(t, err)
	require.True(t, processed)
}

func TestAiTaskServiceViewHidesInternals(t *testing.T) {
	view := NewAiTaskView(&model.AiTask{ID: 5, TaskType: model.AiTaskTypeOrganize, Status: model.AiTaskFailed, ErrorMessage: "boom", RetryCount: 2})
	require.NotNil(t, view)
	require.Equal(t, uint(5), view.ID)
	require.Equal(t, model.AiTaskFailed, view.Status)
	require.Nil(t, NewAiTaskView(nil))
}
