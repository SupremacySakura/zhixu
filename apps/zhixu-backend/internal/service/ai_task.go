package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"zhixu-backend/internal/log"
	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"

	"gorm.io/gorm"
)

// AiTaskProcessor 是各 AI 工作流（整理、关联、演化）的执行入口。
// 返回值是写回 ai_tasks.payload 的产物 JSON；错误用 TerminalTaskError 标记不可重试。
type AiTaskProcessor interface {
	TaskType() string
	Process(ctx context.Context, task *model.AiTask) (json.RawMessage, error)
}

// AiTaskTerminalError 标记不可重试的失败（模型未配置、知识已删除、Schema 不合规等）：
// 任务直接进入 failed，不再消耗模型调用。
type AiTaskTerminalError struct{ Message string }

func (e *AiTaskTerminalError) Error() string { return e.Message }

// TerminalTaskError 构造一个不可重试的任务错误。
func TerminalTaskError(message string) error { return &AiTaskTerminalError{Message: message} }

// AiTaskStore 是 AI 任务状态机的持久化接口。
type AiTaskStore interface {
	Enqueue(context.Context, model.AiTask) (*model.AiTask, error)
	Latest(context.Context, uint, string, uint) (*model.AiTask, error)
	Claim(context.Context, string, time.Time) (*model.AiTask, error)
	RecycleExpired(context.Context, time.Time) error
	Succeed(context.Context, *model.AiTask, string) error
	Fail(context.Context, *model.AiTask, string, bool, time.Time) error
	UpdatePayload(context.Context, uint, uint, string) error
}

// AiTaskService 统一驱动所有 AI 工作流：入队幂等、租约领取、失败退避与终态收敛。
type AiTaskService struct {
	Store AiTaskStore
	// Processors 按 task_type 注册处理器；为空时不领取任务（任务停在 pending，不产生模型调用）。
	Processors map[string]AiTaskProcessor
}

func NewAiTaskService(store AiTaskStore) *AiTaskService {
	return &AiTaskService{Store: store, Processors: map[string]AiTaskProcessor{}}
}

// Register 注册一个工作流处理器，返回自身便于装配链式调用。
func (s *AiTaskService) Register(processor AiTaskProcessor) *AiTaskService {
	if s.Processors == nil {
		s.Processors = map[string]AiTaskProcessor{}
	}
	s.Processors[processor.TaskType()] = processor
	return s
}

// Enqueue 幂等排入一个 AI 任务；payload 为 nil 时落空对象。
func (s *AiTaskService) Enqueue(ctx context.Context, userID uint, taskType string, targetID uint, payload any) (*model.AiTask, error) {
	body := "{}"
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = string(encoded)
	}
	return s.Store.Enqueue(ctx, model.AiTask{
		UserID: userID, TaskType: taskType, TargetID: targetID, Payload: body,
	})
}

// EnqueueRelation 按知识维度入队关联任务：dedupe_key 为 relation:<knowledgeID>，
// 同一知识在活跃期只保留一个任务；成功后再次触发会重新判定并与既有建议合并。
func (s *AiTaskService) EnqueueRelation(ctx context.Context, userID, knowledgeID, baseVersion uint) error {
	payload, err := json.Marshal(RelationEnqueuePayload{KnowledgeID: knowledgeID, BaseVersion: baseVersion})
	if err != nil {
		return err
	}
	key := fmt.Sprintf("relation:%d", knowledgeID)
	_, err = s.Store.Enqueue(ctx, model.AiTask{
		UserID: userID, TaskType: model.AiTaskTypeRelation, TargetID: knowledgeID,
		Payload: string(payload), DedupeKey: &key,
	})
	return err
}

// Latest 返回目标最近一次任务。
func (s *AiTaskService) Latest(ctx context.Context, userID uint, taskType string, targetID uint) (*model.AiTask, error) {
	return s.Store.Latest(ctx, userID, taskType, targetID)
}

// UpdatePayload 覆盖任务产物（用于记录用户采纳整理结果等状态）。
func (s *AiTaskService) UpdatePayload(ctx context.Context, userID, taskID uint, payload []byte) error {
	return s.Store.UpdatePayload(ctx, userID, taskID, string(payload))
}

// ProcessNext 领取并执行一条到期任务；没有可执行任务时返回 false。
func (s *AiTaskService) ProcessNext(ctx context.Context) (bool, error) {
	if len(s.Processors) == 0 {
		return false, nil
	}
	token := make([]byte, 16)
	if _, err := rand.Read(token); err != nil {
		return false, err
	}
	task, err := s.Store.Claim(ctx, hex.EncodeToString(token), time.Now())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	processor, ok := s.Processors[task.TaskType]
	if !ok {
		return true, s.finishFail(ctx, task, "该类型的 AI 工作流尚未启用", false)
	}
	payload, processErr := processor.Process(ctx, task)
	if ctx.Err() != nil {
		// 进程退出：保留租约，等重启后由回收流程重新排队。
		return true, ctx.Err()
	}
	if processErr != nil {
		var terminal *AiTaskTerminalError
		if errors.As(processErr, &terminal) {
			return true, s.finishFail(ctx, task, terminal.Message, false)
		}
		return true, s.finishFail(ctx, task, processErr.Error(), true)
	}
	return true, s.finish(task.ID, s.Store.Succeed(ctx, task, string(payload)))
}

func (s *AiTaskService) finishFail(ctx context.Context, task *model.AiTask, message string, retryable bool) error {
	return s.finish(task.ID, s.Store.Fail(ctx, task, message, retryable, time.Now()))
}

// finish 把租约丢失当作正常竞态处理：另一 worker 已经接管，本次结果作废即可。
func (s *AiTaskService) finish(taskID uint, err error) error {
	if errors.Is(err, repository.ErrAiTaskLeaseLost) {
		log.Warn().Uint("task_id", taskID).Msg("AI 任务租约已失效，本次结果被丢弃")
		return nil
	}
	return err
}

// Run 常驻驱动：2 秒一轮，先回收过期租约，再按注册的处理器消费任务。
func (s *AiTaskService) Run(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		if err := s.Store.RecycleExpired(ctx, time.Now()); err != nil && ctx.Err() == nil {
			log.Error().Err(err).Msg("AI 任务租约回收失败")
		}
		if len(s.Processors) > 0 {
			for ctx.Err() == nil {
				processed, err := s.ProcessNext(ctx)
				if err != nil && ctx.Err() == nil {
					log.Error().Err(err).Msg("AI 任务处理失败")
				}
				if !processed || err != nil {
					break
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// AiTaskView 是对外暴露的任务快照，隐藏租约、幂等键等内部字段。
type AiTaskView struct {
	ID           uint               `json:"id"`
	TaskType     string             `json:"task_type"`
	Status       model.AiTaskStatus `json:"status"`
	ErrorMessage string             `json:"error_message"`
	RetryCount   int                `json:"retry_count"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

// NewAiTaskView 由任务记录构造对外快照。
func NewAiTaskView(task *model.AiTask) *AiTaskView {
	if task == nil {
		return nil
	}
	return &AiTaskView{
		ID: task.ID, TaskType: task.TaskType, Status: task.Status,
		ErrorMessage: task.ErrorMessage, RetryCount: task.RetryCount, UpdatedAt: task.UpdatedAt,
	}
}
