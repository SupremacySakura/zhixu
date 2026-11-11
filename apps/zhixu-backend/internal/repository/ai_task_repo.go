package repository

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"zhixu-backend/internal/model"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrAiTaskLeaseLost 表示任务租约已被回收或被别的 worker 接管，本次结果必须丢弃。
var ErrAiTaskLeaseLost = errors.New("ai task lease lost")

const (
	// AiTaskMaxRetries 是一个任务允许的重试次数上限（首次执行 + 3 次重试，第 4 次失败才终态）。
	AiTaskMaxRetries = 3
	// AiTaskLeaseTTL 覆盖一次模型调用的最坏耗时；worker 崩溃后任务最多等这么久才回收。
	AiTaskLeaseTTL      = 3 * time.Minute
	aiTaskErrorMaxRunes = 500
)

// AiTaskBackoff 返回第 attempt 次失败后的等待时间：30s / 60s / 120s。
func AiTaskBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	return time.Duration(30<<(attempt-1)) * time.Second
}

type AiTaskRepository struct{ DB *gorm.DB }

// aiTaskDedupeKey 只在活跃期占位；带上用户维度，避免不同用户的同号目标互相阻塞。
func aiTaskDedupeKey(userID uint, taskType string, targetID uint) string {
	return fmt.Sprintf("%d:%s:%d", userID, taskType, targetID)
}

func truncateAiTaskError(message string) string {
	if utf8.RuneCountInString(message) <= aiTaskErrorMaxRunes {
		return message
	}
	return string([]rune(message)[:aiTaskErrorMaxRunes])
}

func isDuplicateAiTask(err error) bool {
	var duplicate *mysqlDriver.MySQLError
	return errors.Is(err, gorm.ErrDuplicatedKey) || (errors.As(err, &duplicate) && duplicate.Number == 1062)
}

// Enqueue 幂等入队：同一目标已有活跃任务时返回该任务，不新建也不报错。
func (r *AiTaskRepository) Enqueue(ctx context.Context, task model.AiTask) (*model.AiTask, error) {
	if task.Payload == "" {
		task.Payload = "{}"
	}
	if task.NextRunAt.IsZero() {
		task.NextRunAt = time.Now()
	}
	if task.DedupeKey == nil {
		key := aiTaskDedupeKey(task.UserID, task.TaskType, task.TargetID)
		task.DedupeKey = &key
	}
	task.Status = model.AiTaskPending
	if err := r.DB.WithContext(ctx).Create(&task).Error; err != nil {
		if isDuplicateAiTask(err) {
			return r.latestActive(ctx, task.UserID, task.TaskType, task.TargetID)
		}
		return nil, err
	}
	return &task, nil
}

func (r *AiTaskRepository) latestActive(ctx context.Context, userID uint, taskType string, targetID uint) (*model.AiTask, error) {
	var task model.AiTask
	err := r.DB.WithContext(ctx).
		Where("user_id = ? AND task_type = ? AND target_id = ? AND status IN ?",
			userID, taskType, targetID, []model.AiTaskStatus{model.AiTaskPending, model.AiTaskRunning}).
		Order("id DESC").Take(&task).Error
	return &task, err
}

// HasTaskForTarget 判断目标是否已经有过该类型的任务（任意状态），
// 供自动流程判断“这个知识是否已经跑过”，避免编辑正文时反复触发。
func (r *AiTaskRepository) HasTaskForTarget(ctx context.Context, userID uint, taskType string, targetID uint) (bool, error) {
	var count int64
	if err := r.DB.WithContext(ctx).Model(&model.AiTask{}).
		Where("user_id = ? AND task_type = ? AND target_id = ?", userID, taskType, targetID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Latest 返回目标最近一次任务（任意状态），用于详情页展示整理状态。
func (r *AiTaskRepository) Latest(ctx context.Context, userID uint, taskType string, targetID uint) (*model.AiTask, error) {
	var task model.AiTask
	err := r.DB.WithContext(ctx).
		Where("user_id = ? AND task_type = ? AND target_id = ?", userID, taskType, targetID).
		Order("id DESC").Take(&task).Error
	return &task, err
}

// Claim 用 SKIP LOCKED 领取一条到期任务并打上租约，多个 worker 不会重复领取同一条。
func (r *AiTaskRepository) Claim(ctx context.Context, token string, now time.Time) (*model.AiTask, error) {
	var task model.AiTask
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND next_run_at <= ?", model.AiTaskPending, now).
			Order("id ASC").Limit(1).Find(&task)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		expires := now.Add(AiTaskLeaseTTL)
		if err := tx.Model(&model.AiTask{}).Where("id = ?", task.ID).Updates(map[string]any{
			"status": model.AiTaskRunning, "lease_token": token, "lease_expires_at": expires,
		}).Error; err != nil {
			return err
		}
		task.Status, task.LeaseToken, task.LeaseExpiresAt = model.AiTaskRunning, token, &expires
		return nil
	})
	return &task, err
}

// RecycleExpired 回收租约过期的任务：先把超过重试上限的置为失败，其余退回 pending。
// 回收计一次失败，避免一条总是让 worker 崩溃的任务被无限重领。
func (r *AiTaskRepository) RecycleExpired(ctx context.Context, now time.Time) error {
	db := r.DB.WithContext(ctx)
	if err := db.Model(&model.AiTask{}).
		Where("status = ? AND lease_expires_at IS NOT NULL AND lease_expires_at < ? AND retry_count + 1 > ?",
			model.AiTaskRunning, now, AiTaskMaxRetries).
		Updates(map[string]any{
			"status": model.AiTaskFailed, "error_message": "任务多次超时未完成，已停止重试",
			"dedupe_key": nil, "lease_token": "", "lease_expires_at": nil,
		}).Error; err != nil {
		return err
	}
	return db.Model(&model.AiTask{}).
		Where("status = ? AND lease_expires_at IS NOT NULL AND lease_expires_at < ?", model.AiTaskRunning, now).
		Updates(map[string]any{
			"status": model.AiTaskPending, "lease_token": "", "lease_expires_at": nil,
			"retry_count": gorm.Expr("retry_count + 1"), "next_run_at": now,
		}).Error
}

// UpdatePayload 只覆盖任务产物（如记录用户采纳状态），不触碰状态、租约与重试计数。
func (r *AiTaskRepository) UpdatePayload(ctx context.Context, userID, taskID uint, payload string) error {
	result := r.DB.WithContext(ctx).Model(&model.AiTask{}).
		Where("id = ? AND user_id = ?", taskID, userID).
		Update("payload", payload)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Succeed 写入工作流产物并置为成功；租约已失效时不覆盖新任务的结果。
func (r *AiTaskRepository) Succeed(ctx context.Context, task *model.AiTask, payload string) error {
	if payload == "" {
		payload = "{}"
	}
	result := r.DB.WithContext(ctx).Model(&model.AiTask{}).
		Where("id = ? AND status = ? AND lease_token = ?", task.ID, model.AiTaskRunning, task.LeaseToken).
		Updates(map[string]any{
			"status": model.AiTaskSucceeded, "payload": payload, "error_message": "",
			"dedupe_key": nil, "lease_token": "", "lease_expires_at": nil,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrAiTaskLeaseLost
	}
	return nil
}

// Fail 记录失败：可重试且未超限时按指数退避退回 pending，否则置为终态失败并释放幂等键。
func (r *AiTaskRepository) Fail(ctx context.Context, task *model.AiTask, message string, retryable bool, now time.Time) error {
	attempt := task.RetryCount + 1
	updates := map[string]any{
		"retry_count": attempt, "error_message": truncateAiTaskError(message),
		"lease_token": "", "lease_expires_at": nil,
	}
	if !retryable || attempt > AiTaskMaxRetries {
		updates["status"] = model.AiTaskFailed
		updates["dedupe_key"] = nil
	} else {
		updates["status"] = model.AiTaskPending
		updates["next_run_at"] = now.Add(AiTaskBackoff(attempt))
	}
	result := r.DB.WithContext(ctx).Model(&model.AiTask{}).
		Where("id = ? AND status = ? AND lease_token = ?", task.ID, model.AiTaskRunning, task.LeaseToken).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrAiTaskLeaseLost
	}
	return nil
}
