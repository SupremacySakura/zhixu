package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// evolveKnowledgeFake 提供触发知识与指纹命中结果；命中用 found 表示「没有重复」。
type evolveKnowledgeFake struct {
	items      map[uint]*model.KnowledgeItem
	duplicate  *model.KnowledgeItem
	dupErr     error
	gotHash    string
	gotExclude uint
	findCalls  int
}

func (f *evolveKnowledgeFake) Get(_ context.Context, userID, id uint) (*model.KnowledgeItem, error) {
	item, ok := f.items[id]
	if !ok || item.UserID != userID {
		return nil, gorm.ErrRecordNotFound
	}
	return item, nil
}

func (f *evolveKnowledgeFake) FindByContentHash(_ context.Context, _ uint, hash string, excludeID uint) (*model.KnowledgeItem, error) {
	f.findCalls++
	f.gotHash, f.gotExclude = hash, excludeID
	if f.dupErr != nil {
		return nil, f.dupErr
	}
	if f.duplicate == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return f.duplicate, nil
}

type evolveAlertFake struct {
	upserted []model.EvolutionAlert
	outcome  repository.EvolutionUpsertOutcome
	err      error
}

func (f *evolveAlertFake) Upsert(_ context.Context, alert *model.EvolutionAlert) (repository.EvolutionUpsertOutcome, error) {
	if f.err != nil {
		return "", f.err
	}
	f.upserted = append(f.upserted, *alert)
	if f.outcome == "" {
		return repository.EvolutionAlertCreated, nil
	}
	return f.outcome, nil
}

type evolveFixture struct {
	processor *EvolveProcessor
	knowledge *evolveKnowledgeFake
	alerts    *evolveAlertFake
	task      *model.AiTask
}

func newEvolveFixture() *evolveFixture {
	knowledge := &evolveKnowledgeFake{items: map[uint]*model.KnowledgeItem{
		10: {ID: 10, UserID: 2, Title: "副本", Content: "同一段正文", ContentHash: "hash-a", Status: model.KnowledgeReady, Version: 2},
		11: {ID: 11, UserID: 2, Title: "原件", Content: "同一段正文", ContentHash: "hash-a", Status: model.KnowledgeReady, Version: 5},
		12: {ID: 12, UserID: 2, Title: "空正文", Content: "", ContentHash: "", Status: model.KnowledgeReady, Version: 1},
	}}
	now := time.Now()
	knowledge.items[13] = &model.KnowledgeItem{ID: 13, UserID: 2, Title: "已归档", Content: "x", Status: model.KnowledgeReady, ArchivedAt: &now}
	knowledge.items[14] = &model.KnowledgeItem{ID: 14, UserID: 2, Title: "处理中", Content: "x", Status: model.KnowledgePending}
	payload, _ := json.Marshal(EvolveEnqueuePayload{KnowledgeID: 10, BaseVersion: 2})
	f := &evolveFixture{knowledge: knowledge, alerts: &evolveAlertFake{}}
	f.processor = &EvolveProcessor{Knowledge: knowledge, Alerts: f.alerts}
	f.task = &model.AiTask{
		ID: 3, UserID: 2, TaskType: model.AiTaskTypeEvolve, TargetID: 10,
		Status: model.AiTaskRunning, Payload: string(payload),
	}
	return f
}

// 精确重复：命中同用户未归档非自身的 ID 最小条目，落一条待处理提醒，不调模型。
func TestEvolveProcessorRecordsDuplicate(t *testing.T) {
	f := newEvolveFixture()
	f.knowledge.duplicate = f.knowledge.items[11]

	payload, err := f.processor.Process(context.Background(), f.task)

	require.NoError(t, err)
	require.Equal(t, "hash-a", f.knowledge.gotHash)
	require.Equal(t, uint(10), f.knowledge.gotExclude, "排除自身")
	require.Len(t, f.alerts.upserted, 1)
	alert := f.alerts.upserted[0]
	require.Equal(t, model.EvolutionAlertTypeDuplicate, alert.AlertType)
	require.Equal(t, model.EvolutionAlertPending, alert.Status)
	require.Equal(t, model.EvolutionSeverityReview, alert.Severity)
	require.Equal(t, []uint{10, 11}, alert.SubjectIDList(), "触发知识在前，被比对知识在后")
	require.Len(t, alert.DedupeKey, 64)
	require.Contains(t, string(payload), "\"alert_count\":1")
	parsed := ParseEvolutionPayload(alert.Payload)
	require.Equal(t, model.EvolutionAlertTypeDuplicate, parsed.Type)
	require.Equal(t, model.EvolutionActionArchiveOne, parsed.SuggestedAction)
	require.Len(t, parsed.Evidence, 2)
	require.Equal(t, "同一段正文", parsed.Evidence[0].Quote)
}

// 无重复：任务成功但零提醒。
func TestEvolveProcessorNoDuplicate(t *testing.T) {
	f := newEvolveFixture()

	payload, err := f.processor.Process(context.Background(), f.task)

	require.NoError(t, err)
	require.Empty(t, f.alerts.upserted)
	require.Contains(t, string(payload), "\"alert_count\":0")
}

// 同指纹已处理过：抑制本次提醒，任务仍然成功（不把「用户已经裁决过」当成失败）。
func TestEvolveProcessorSuppressed(t *testing.T) {
	f := newEvolveFixture()
	f.knowledge.duplicate = f.knowledge.items[11]
	f.alerts.outcome = repository.EvolutionAlertSuppressed

	payload, err := f.processor.Process(context.Background(), f.task)

	require.NoError(t, err)
	require.Contains(t, string(payload), "\"alert_count\":0")
}

// 版本栅栏：入队后正文被改过，本次检查作废且不落提醒。
func TestEvolveProcessorVersionFence(t *testing.T) {
	f := newEvolveFixture()
	f.knowledge.duplicate = f.knowledge.items[11]
	f.knowledge.items[10].Version = 3

	_, err := f.processor.Process(context.Background(), f.task)

	terminal := requireTerminalError(t, err)
	require.Equal(t, "知识已更新，本次检查作废", terminal.Message)
	require.Empty(t, f.alerts.upserted)
	require.Equal(t, 0, f.knowledge.findCalls, "作废的任务不该再查重")
}

// 前置状态不满足时都是不可重试的失败，避免反复消耗任务额度。
func TestEvolveProcessorTerminalGuards(t *testing.T) {
	for _, tc := range []struct {
		name    string
		target  uint
		payload string
		message string
	}{
		{"missing knowledge", 99, `{"knowledge_id":99,"base_version":1}`, "知识不存在或已删除"},
		{"archived knowledge", 13, `{"knowledge_id":13,"base_version":1}`, "知识已归档，无法执行演化检查"},
		{"not ready", 14, `{"knowledge_id":14,"base_version":1}`, "知识尚未就绪，无法执行演化检查"},
		{"broken payload", 10, `{`, "演化检查载荷损坏"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newEvolveFixture()
			f.task.TargetID, f.task.Payload = tc.target, tc.payload
			_, err := f.processor.Process(context.Background(), f.task)
			terminal := requireTerminalError(t, err)
			require.Equal(t, tc.message, terminal.Message)
		})
	}
}

// 未装配依赖时给明确提示，而不是空指针。
func TestEvolveProcessorUnconfigured(t *testing.T) {
	p := &EvolveProcessor{}
	_, err := p.Process(context.Background(), &model.AiTask{TaskType: model.AiTaskTypeEvolve})
	terminal := requireTerminalError(t, err)
	require.Equal(t, "演化检查未配置", terminal.Message)
	require.Equal(t, model.AiTaskTypeEvolve, p.TaskType())
}

// 空正文不构成重复证据：两条没有内容的知识不该互相提醒。
func TestEvolveProcessorSkipsEmptyContent(t *testing.T) {
	f := newEvolveFixture()
	f.knowledge.items[10].Content = ""
	f.knowledge.duplicate = f.knowledge.items[11]

	_, err := f.processor.Process(context.Background(), f.task)

	require.NoError(t, err)
	require.Empty(t, f.alerts.upserted)
}
