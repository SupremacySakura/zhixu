package service

import (
	"context"
	"encoding/json"
	"errors"

	"zhixu-backend/internal/log"
	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"

	"gorm.io/gorm"
)

// evolveKnowledgeSource 读取触发检查的知识（*KnowledgeService 满足）。
type evolveKnowledgeSource interface {
	Get(context.Context, uint, uint) (*model.KnowledgeItem, error)
	FindByContentHash(context.Context, uint, string, uint) (*model.KnowledgeItem, error)
}

// evolveAlertStore 写入提醒（*repository.EvolutionAlertRepository 满足）。
type evolveAlertStore interface {
	Upsert(context.Context, *model.EvolutionAlert) (repository.EvolutionUpsertOutcome, error)
}

// EvolveProcessor 是演化检查工作流的执行入口：一条触发知识产出 0~N 条提醒。
//
// 本迭代只做精确重复（正文指纹相同）。语义近重复、过时与冲突判定会复用同一条任务，
// 在那之前这里刻意不调用模型：没有模型也能跑通「就绪 → 入队 → 落提醒 → 裁决」的整条链路。
type EvolveProcessor struct {
	Knowledge evolveKnowledgeSource
	Alerts    evolveAlertStore
}

func (p *EvolveProcessor) TaskType() string { return model.AiTaskTypeEvolve }

func (p *EvolveProcessor) Process(ctx context.Context, task *model.AiTask) (json.RawMessage, error) {
	if p.Knowledge == nil || p.Alerts == nil {
		return nil, TerminalTaskError("演化检查未配置")
	}
	var payload EvolveEnqueuePayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return nil, TerminalTaskError("演化检查载荷损坏")
	}
	item, err := p.Knowledge.Get(ctx, task.UserID, task.TargetID)
	// KnowledgeService 会把 not found 包成 ErrKnowledgeNotFound，直接注入仓储时是 gorm 的错误，两者都要认。
	if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, ErrKnowledgeNotFound) {
		return nil, TerminalTaskError("知识不存在或已删除")
	}
	if err != nil {
		return nil, err
	}
	if item.ArchivedAt != nil {
		return nil, TerminalTaskError("知识已归档，无法执行演化检查")
	}
	if item.Status != model.KnowledgeReady {
		return nil, TerminalTaskError("知识尚未就绪，无法执行演化检查")
	}
	// 版本栅栏：入队时的正文与现在的正文不是同一份，旧结论不该落到新版本上。
	// 重试同样受它保护——任务重试期间用户改了正文，这次检查直接作废而不是给出过期判定。
	if payload.BaseVersion != 0 && item.Version != payload.BaseVersion {
		return nil, TerminalTaskError("知识已更新，本次检查作废")
	}
	duplicate, err := p.Knowledge.FindByContentHash(ctx, task.UserID, item.ContentHash, item.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, ErrKnowledgeNotFound) {
		return p.result(item.ID, 0)
	}
	if err != nil {
		return nil, err
	}
	created, err := p.recordDuplicate(ctx, item, duplicate)
	if err != nil {
		return nil, err
	}
	if created {
		return p.result(item.ID, 1)
	}
	return p.result(item.ID, 0)
}

// recordDuplicate 落一条精确重复提醒，返回是否真的写入了新行。
// 证据取双方正文的开头：正文完全一致时同一段引文在两侧都能定位，左右并排才有对照价值。
func (p *EvolveProcessor) recordDuplicate(ctx context.Context, trigger, duplicate *model.KnowledgeItem) (bool, error) {
	quote := evolutionExcerpt(duplicate.Content, EvolveQuoteMaxRunes)
	if quote == "" {
		// 空正文不构成重复证据：没有内容可比的两条知识不该互相提醒。
		return false, nil
	}
	evidence := validEvidence(trigger.Content, []EvolutionEvidence{
		{KnowledgeID: trigger.ID, Quote: quote},
		{KnowledgeID: duplicate.ID, Quote: quote},
	})
	if len(evidence) == 0 {
		return false, nil
	}
	subjectIDs := []uint{trigger.ID, duplicate.ID}
	body, err := json.Marshal(EvolutionPayload{
		Type:   model.EvolutionAlertTypeDuplicate,
		Reason: "两条知识的正文完全一致（正文指纹相同），可能是重复导入或重复创建。",
		// 证据顺序与 subject 顺序对齐：第 0 条对应本次触发的知识。
		Evidence:        evidence,
		SuggestedAction: model.EvolutionActionArchiveOne,
	})
	if err != nil {
		return false, err
	}
	alert := &model.EvolutionAlert{
		UserID:    trigger.UserID,
		AlertType: model.EvolutionAlertTypeDuplicate,
		Status:    model.EvolutionAlertPending,
		// 精确重复是确定性判定，没有模型复核的空间，固定按「复核」展示。
		Severity:  model.EvolutionSeverityReview,
		Payload:   string(body),
		DedupeKey: EvolutionDedupeKey(model.EvolutionAlertTypeDuplicate, subjectIDs, evidence),
	}
	if err := alert.SetSubjectIDs(subjectIDs); err != nil {
		return false, err
	}
	outcome, err := p.Alerts.Upsert(ctx, alert)
	if err != nil {
		return false, err
	}
	if outcome == repository.EvolutionAlertSuppressed {
		log.Info().Uint("knowledge_id", trigger.ID).Msg("同证据的演化提醒已处理过，本次不再提醒")
	}
	return outcome == repository.EvolutionAlertCreated || outcome == repository.EvolutionAlertRevived, nil
}

// result 是写回 ai_tasks.payload 的产物，供详情页展示本次检查产出。
func (p *EvolveProcessor) result(knowledgeID uint, alertCount int) (json.RawMessage, error) {
	return json.Marshal(map[string]any{
		"knowledge_id": knowledgeID,
		"alert_count":  alertCount,
	})
}
