package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"zhixu-backend/internal/model"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrEvolutionConflict 表示提醒状态已被改动（期望状态校验失败或已被他人裁决）。
var ErrEvolutionConflict = errors.New("evolution alert state conflict")

// EvolutionUpsertOutcome 描述一次提醒写入的结果。
type EvolutionUpsertOutcome string

const (
	// EvolutionAlertCreated 新建了一条待处理提醒。
	EvolutionAlertCreated EvolutionUpsertOutcome = "created"
	// EvolutionAlertRefreshed 同指纹已有待处理提醒，只刷新了更新时间。
	EvolutionAlertRefreshed EvolutionUpsertOutcome = "refreshed"
	// EvolutionAlertRevived 同指纹提醒处于「暂不处理」，因重新检查或知识版本变化回到待处理。
	EvolutionAlertRevived EvolutionUpsertOutcome = "revived"
	// EvolutionAlertSuppressed 同指纹提醒已处理或已忽略，抑制本次提醒。
	EvolutionAlertSuppressed EvolutionUpsertOutcome = "suppressed"
)

// EvolutionAlertTypeCount 是摘要接口的一行分组计数。
type EvolutionAlertTypeCount struct {
	Status    model.EvolutionAlertStatus
	AlertType model.EvolutionAlertType
	Count     int64
}

// EvolutionAlertFilter 描述提醒列表的筛选条件；零值表示不筛选。
type EvolutionAlertFilter struct {
	Page int
	Size int
	Type model.EvolutionAlertType
	// Statuses 是允许的状态集合：空表示不筛选，单值按等值匹配，多值按 IN 匹配。
	// 多值用于演化中心的「处理记录」页签——已处理与已忽略要同时展示。
	Statuses []model.EvolutionAlertStatus
	// Open 为真时只取待办（待处理 + 暂不处理），优先于 Statuses。
	Open bool
}

type EvolutionAlertRepository struct{ DB *gorm.DB }

func NewEvolutionAlertRepository(db *gorm.DB) *EvolutionAlertRepository {
	return &EvolutionAlertRepository{DB: db}
}

func (r *EvolutionAlertRepository) List(ctx context.Context, userID uint, filter EvolutionAlertFilter) ([]model.EvolutionAlert, int64, error) {
	db := r.DB.WithContext(ctx).Model(&model.EvolutionAlert{}).Where("user_id = ?", userID)
	if filter.Type != "" {
		db = db.Where("alert_type = ?", filter.Type)
	}
	if filter.Open {
		db = db.Where("status IN ?", []model.EvolutionAlertStatus{model.EvolutionAlertPending, model.EvolutionAlertDeferred})
	} else if len(filter.Statuses) == 1 {
		db = db.Where("status = ?", filter.Statuses[0])
	} else if len(filter.Statuses) > 1 {
		db = db.Where("status IN ?", filter.Statuses)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	// 默认按创建时间倒序：按严重度排会让旧的高严重度提醒永远压在顶部。
	// 多状态筛选是「处理记录」视野，按裁决时间倒序才符合「最近处理过的在前」。
	order := "created_at DESC, id DESC"
	if !filter.Open && len(filter.Statuses) > 1 {
		order = "resolved_at DESC, id DESC"
	}
	alerts := make([]model.EvolutionAlert, 0)
	err := db.Order(order).
		Offset((filter.Page - 1) * filter.Size).Limit(filter.Size).Find(&alerts).Error
	return alerts, total, err
}

func (r *EvolutionAlertRepository) Get(ctx context.Context, userID, id uint) (*model.EvolutionAlert, error) {
	var alert model.EvolutionAlert
	err := r.DB.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Take(&alert).Error
	return &alert, err
}

func (r *EvolutionAlertRepository) findByDedupeKey(ctx context.Context, userID uint, key string) (*model.EvolutionAlert, error) {
	var alert model.EvolutionAlert
	err := r.DB.WithContext(ctx).Where("user_id = ? AND dedupe_key = ?", userID, key).Take(&alert).Error
	return &alert, err
}

// Upsert 按指纹幂等写入一条提醒。指纹带唯一索引且终态不清空，因此同证据只会有一行：
// 待处理只刷新时间；暂不处理回到待处理；已处理/已忽略保持终态并抑制本次提醒。
func (r *EvolutionAlertRepository) Upsert(ctx context.Context, alert *model.EvolutionAlert) (EvolutionUpsertOutcome, error) {
	existing, err := r.findByDedupeKey(ctx, alert.UserID, alert.DedupeKey)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	if err == nil {
		return r.refresh(ctx, existing)
	}
	if err := r.DB.WithContext(ctx).Create(alert).Error; err != nil {
		// 并发写入命中唯一索引：回读旧行按刷新处理，对上层表现为幂等。
		if !isDuplicateEvolutionAlert(err) {
			return "", err
		}
		existing, lookupErr := r.findByDedupeKey(ctx, alert.UserID, alert.DedupeKey)
		if lookupErr != nil {
			return "", lookupErr
		}
		return r.refresh(ctx, existing)
	}
	return EvolutionAlertCreated, nil
}

func (r *EvolutionAlertRepository) refresh(ctx context.Context, existing *model.EvolutionAlert) (EvolutionUpsertOutcome, error) {
	if !existing.Status.Open() {
		return EvolutionAlertSuppressed, nil
	}
	updates := map[string]any{"updated_at": time.Now()}
	outcome := EvolutionAlertRefreshed
	if existing.Status == model.EvolutionAlertDeferred {
		// 复活条件只可能由「知识版本变化后重新检查」或「手动重新检查」触发，
		// 两者都会走到这里，因此同指纹再次命中即回到待处理。
		updates["status"] = model.EvolutionAlertPending
		updates["resolved_at"] = nil
		updates["resolved_action"] = ""
		updates["resolution_note"] = ""
		outcome = EvolutionAlertRevived
	}
	if err := r.DB.WithContext(ctx).Model(&model.EvolutionAlert{}).
		Where("id = ? AND user_id = ?", existing.ID, existing.UserID).Updates(updates).Error; err != nil {
		return "", err
	}
	return outcome, nil
}

// Summary 一条分组查询算出待处理与暂不处理的按类型计数。
func (r *EvolutionAlertRepository) Summary(ctx context.Context, userID uint) ([]EvolutionAlertTypeCount, error) {
	rows := make([]EvolutionAlertTypeCount, 0)
	err := r.DB.WithContext(ctx).Model(&model.EvolutionAlert{}).
		Select("status, alert_type, COUNT(*) AS count").
		Where("user_id = ? AND status IN ?", userID, []model.EvolutionAlertStatus{model.EvolutionAlertPending, model.EvolutionAlertDeferred}).
		Group("status, alert_type").
		Scan(&rows).Error
	return rows, err
}

// Resolve 在单事务内完成一次裁决：锁行、校验状态、执行动作副作用、写回裁决字段。
// apply 负责动作对知识侧的副作用（如归档其一）；返回 changed=false 表示提醒已不在待办视野，
// 调用方按幂等处理，不再写日志。
func (r *EvolutionAlertRepository) Resolve(ctx context.Context, userID, id uint, action model.EvolutionAction, note string, expected model.EvolutionAlertStatus, apply func(tx *gorm.DB, alert *model.EvolutionAlert) error) (*model.EvolutionAlert, bool, error) {
	var alert model.EvolutionAlert
	changed := false
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", id, userID).Take(&alert).Error; err != nil {
			return err
		}
		if !alert.Status.Open() {
			return nil
		}
		if expected != "" && expected != alert.Status {
			return ErrEvolutionConflict
		}
		if apply != nil {
			if err := apply(tx, &alert); err != nil {
				return err
			}
		}
		now := time.Now()
		updates := map[string]any{"status": evolutionResolveStatus(action), "resolution_note": note, "updated_at": now}
		if action == model.EvolutionActionDefer {
			// 暂不处理是状态迁移而不是裁决：不写裁决时间，也不终止提醒。
			updates["resolved_at"] = nil
			updates["resolved_action"] = ""
		} else {
			updates["resolved_at"] = now
			updates["resolved_action"] = action
		}
		if err := tx.Model(&model.EvolutionAlert{}).Where("id = ? AND user_id = ?", id, userID).
			Updates(updates).Error; err != nil {
			return err
		}
		alert.Status = evolutionResolveStatus(action)
		if action != model.EvolutionActionDefer {
			alert.ResolvedAt = &now
			alert.ResolvedAction = action
		}
		alert.ResolutionNote = note
		changed = true
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return &alert, changed, nil
}

// evolutionResolveStatus 把动作映射成裁决后的状态。
// 「忽略」与「已核实/保留/归档」在处理记录里是两个页签：忽略是 dismissed，其余是 resolved。
func evolutionResolveStatus(action model.EvolutionAction) model.EvolutionAlertStatus {
	switch action {
	case model.EvolutionActionDefer:
		return model.EvolutionAlertDeferred
	case model.EvolutionActionDismiss:
		return model.EvolutionAlertDismissed
	default:
		return model.EvolutionAlertResolved
	}
}

// deleteEvolutionAlertsByKnowledge 清理涉及被删除知识的提醒；与知识删除同事务完成。
func deleteEvolutionAlertsByKnowledge(tx *gorm.DB, userID uint, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	payload, err := jsonArrayOfUint(ids)
	if err != nil {
		return err
	}
	return tx.Where("user_id = ? AND JSON_OVERLAPS(subject_ids, ?)", userID, payload).
		Delete(&model.EvolutionAlert{}).Error
}

// jsonArrayOfUint 把 ID 列表编码成 JSON 数组，供 JSON_OVERLAPS 匹配 subject_ids。
func jsonArrayOfUint(ids []uint) (string, error) {
	encoded, err := json.Marshal(ids)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func isDuplicateEvolutionAlert(err error) bool {
	var duplicate *mysqlDriver.MySQLError
	return errors.Is(err, gorm.ErrDuplicatedKey) || (errors.As(err, &duplicate) && duplicate.Number == 1062)
}
