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

// 处置结果定义在 model 里：合并纯函数也要用它，两边必须是同一个类型。
type EvolutionUpsertOutcome = model.EvolutionUpsertOutcome

const (
	EvolutionAlertCreated    = model.EvolutionAlertCreated
	EvolutionAlertRefreshed  = model.EvolutionAlertRefreshed
	EvolutionAlertRevived    = model.EvolutionAlertRevived
	EvolutionAlertSuppressed = model.EvolutionAlertSuppressed
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

func (r *EvolutionAlertRepository) findByPairKey(ctx context.Context, userID uint, key string) (*model.EvolutionAlert, error) {
	var alert model.EvolutionAlert
	err := r.DB.WithContext(ctx).Where("user_id = ? AND pair_key = ?", userID, key).Take(&alert).Error
	return &alert, err
}

// Upsert 按知识对幂等写入一条提醒：一对知识只有一行，双向检查落在同一行上。
//
// 收敛规则不在仓储里，而是调用 model.EvolutionMergeFunc（服务层注入的纯函数），
// 这样合并语义可以用表驱动测试覆盖，不必靠拼 SQL 断言。这里只负责读、调、写，
// 以及唯一索引冲突后的回读重试。
func (r *EvolutionAlertRepository) Upsert(ctx context.Context, alert *model.EvolutionAlert, merge model.EvolutionMergeFunc) (EvolutionUpsertOutcome, error) {
	existing, err := r.findByPairKey(ctx, alert.UserID, alert.PairKey)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	if err == nil {
		return r.applyMerge(ctx, existing, alert, merge)
	}
	if err := r.DB.WithContext(ctx).Create(alert).Error; err != nil {
		// 并发写入命中唯一索引：回读旧行按刷新处理，对上层表现为幂等。
		if !isDuplicateEvolutionAlert(err) {
			return "", err
		}
		existing, lookupErr := r.findByPairKey(ctx, alert.UserID, alert.PairKey)
		if lookupErr != nil {
			return "", lookupErr
		}
		return r.applyMerge(ctx, existing, alert, merge)
	}
	return EvolutionAlertCreated, nil
}

// applyMerge 调用合并函数并把结果写回；合并函数返回 nil 表示本次抑制，不写库。
func (r *EvolutionAlertRepository) applyMerge(ctx context.Context, existing, incoming *model.EvolutionAlert, merge model.EvolutionMergeFunc) (EvolutionUpsertOutcome, error) {
	if merge == nil {
		return "", errors.New("evolution alert merge function is required")
	}
	merged, outcome := merge(existing, incoming)
	if merged == nil || outcome == EvolutionAlertSuppressed {
		return EvolutionAlertSuppressed, nil
	}
	// 合并结果落在原行上：ID 与创建时间保持不变，裁决历史因此不会丢。
	updates := map[string]any{
		"alert_type":      merged.AlertType,
		"status":          merged.Status,
		"severity":        merged.Severity,
		"subject_ids":     merged.SubjectIDs,
		"pair_versions":   merged.PairVersions,
		"payload":         merged.Payload,
		"dedupe_key":      merged.DedupeKey,
		"resolved_at":     merged.ResolvedAt,
		"resolved_action": merged.ResolvedAction,
		"resolution_note": merged.ResolutionNote,
		"updated_at":      time.Now(),
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
