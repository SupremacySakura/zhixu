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

// evolveKnowledgeFake 提供触发知识、指纹命中结果与候选详情；命中用 duplicate==nil 表示「没有精确重复」。
type evolveKnowledgeFake struct {
	items      map[uint]*model.KnowledgeItem
	duplicate  *model.KnowledgeItem
	dupErr     error
	gotHash    string
	gotExclude uint
	findCalls  int
	listErr    error
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

func (f *evolveKnowledgeFake) ListByIDs(_ context.Context, userID uint, ids []uint) ([]model.KnowledgeItem, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []model.KnowledgeItem
	for _, id := range ids {
		if item, ok := f.items[id]; ok && item.UserID == userID {
			out = append(out, *item)
		}
	}
	return out, nil
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

// evolveRelationFake 返回已建立的关系；只有「本条更新了对方」的方向会成为强候选。
type evolveRelationFake struct {
	rows    []repository.RelationRecord
	err     error
	listFor uint
}

func (f *evolveRelationFake) ListByKnowledge(_ context.Context, _ uint, knowledgeID uint) ([]repository.RelationRecord, error) {
	f.listFor = knowledgeID
	if f.err != nil {
		return nil, f.err
	}
	return f.rows, nil
}

type evolveSearcherFake struct {
	matches    []VectorMatch
	err        error
	collection string
	topK       int
}

func (f *evolveSearcherFake) Search(_ context.Context, collection string, _ uint, _ []float64, topK int, _ float64) ([]VectorMatch, error) {
	f.collection, f.topK = collection, topK
	if f.err != nil {
		return nil, f.err
	}
	return f.matches, nil
}

// evolveClientFake 记录送给模型的候选，返回预置结论；测试从不产生真实付费请求。
type evolveClientFake struct {
	judgements []EvolveJudgement
	err        error
	calls      int
	candidates []EvolveCandidate
	excerpt    string
}

func (f *evolveClientFake) Evolve(_ context.Context, _ *ChatCapability, _ uint, _ uint, _ string, excerpt string, candidates []EvolveCandidate) ([]EvolveJudgement, error) {
	f.calls++
	f.candidates = candidates
	f.excerpt = excerpt
	if f.err != nil {
		return nil, f.err
	}
	return f.judgements, nil
}

type evolveFixture struct {
	processor *EvolveProcessor
	knowledge *evolveKnowledgeFake
	alerts    *evolveAlertFake
	resolver  *relationTaskResolverFake
	embedder  *vectorEmbedderFake
	searcher  *evolveSearcherFake
	relations *evolveRelationFake
	client    *evolveClientFake
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
	f := &evolveFixture{
		knowledge: knowledge,
		alerts:    &evolveAlertFake{},
		resolver:  &relationTaskResolverFake{chat: &ChatCapability{Model: model.ProviderModel{ModelID: "chat-1"}}, embedding: &ChatCapability{Model: model.ProviderModel{ModelID: "embed-1"}}},
		embedder:  &vectorEmbedderFake{},
		searcher:  &evolveSearcherFake{},
		relations: &evolveRelationFake{},
		client:    &evolveClientFake{},
	}
	f.processor = &EvolveProcessor{
		Knowledge: knowledge, Alerts: f.alerts, Chat: f.resolver, Embedding: f.resolver,
		Embedder: f.embedder, Vectors: f.searcher, Relations: f.relations, Client: f.client,
	}
	f.task = &model.AiTask{
		ID: 3, UserID: 2, TaskType: model.AiTaskTypeEvolve, TargetID: 10,
		Status: model.AiTaskRunning, Payload: string(payload),
	}
	return f
}

// addCandidate 在 fake 里追加一条未归档知识，并让它以给定相似度被向量召回。
func (f *evolveFixture) addCandidate(id uint, title, content string, score float64) {
	f.knowledge.items[id] = &model.KnowledgeItem{
		ID: id, UserID: 2, Title: title, Content: content,
		ContentHash: "hash-" + title, Status: model.KnowledgeReady, Version: 1,
	}
	f.searcher.matches = append(f.searcher.matches, VectorMatch{KnowledgeID: id, Score: score})
}

func (f *evolveFixture) duplicateJudgement(targetID uint, quote string) EvolveJudgement {
	return EvolveJudgement{
		TargetID: targetID,
		Type:     model.EvolutionAlertTypeDuplicate,
		Reason:   "两条知识说的是同一件事，只是措辞不同。",
		Evidence: []EvolutionEvidence{
			{KnowledgeID: 10, Quote: quote},
			{KnowledgeID: targetID, Quote: quote},
		},
		Severity: model.EvolutionSeverityStrong,
	}
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
	// 精确重复是确定性判定，不该为同一件事再付一次模型调用。
	require.Equal(t, 0, f.client.calls)
}

// 语义近重复：指纹没命中时走模型，相似度够高的候选给出的重复结论落库。
func TestEvolveProcessorRecordsSemanticDuplicate(t *testing.T) {
	f := newEvolveFixture()
	f.addCandidate(21, "同一件事的另一种说法", "HNSW 用分层图做近似最近邻检索。", 0.93)
	// 两侧引文各自能在自己的正文里定位，才算证据齐全；因此不能复用同一个引文。
	f.client.judgements = []EvolveJudgement{{
		TargetID: 21,
		Type:     model.EvolutionAlertTypeDuplicate,
		Reason:   "两条知识说的是同一件事，只是措辞不同。",
		Evidence: []EvolutionEvidence{
			{KnowledgeID: 10, Quote: "同一段正文"},
			{KnowledgeID: 21, Quote: "近似最近邻检索"},
		},
		Severity: model.EvolutionSeverityStrong,
	}}

	payload, err := f.processor.Process(context.Background(), f.task)

	require.NoError(t, err)
	require.Equal(t, 1, f.client.calls)
	require.Len(t, f.client.candidates, 1)
	require.Equal(t, EvolveOriginVectorNeighbor, f.client.candidates[0].Origin)
	require.InDelta(t, 0.93, f.client.candidates[0].Score, 0.0001)
	require.Len(t, f.alerts.upserted, 1)
	alert := f.alerts.upserted[0]
	require.Equal(t, model.EvolutionAlertTypeDuplicate, alert.AlertType)
	require.Equal(t, model.EvolutionSeverityStrong, alert.Severity)
	require.Equal(t, []uint{10, 21}, alert.SubjectIDList())
	require.Contains(t, string(payload), "\"alert_count\":1")
}

// 相似度分派：0.75~0.90 只接受冲突结论，本迭代不消费冲突，因此重复结论被丢弃。
func TestEvolveProcessorDropsConclusionOutsideBand(t *testing.T) {
	f := newEvolveFixture()
	f.addCandidate(21, "中等相似", "HNSW 与 IVF 的取舍。", 0.82)
	f.client.judgements = []EvolveJudgement{f.duplicateJudgement(21, "HNSW")}

	payload, err := f.processor.Process(context.Background(), f.task)

	require.NoError(t, err)
	require.Empty(t, f.alerts.upserted)
	require.Contains(t, string(payload), "\"alert_count\":0")
}

// 低于冲突阈值：不接受任何结论，候选也不会进模型批次。
func TestEvolveProcessorIgnoresWeakCandidates(t *testing.T) {
	f := newEvolveFixture()
	f.addCandidate(21, "弱相关", "HNSW", 0.62)

	_, err := f.processor.Process(context.Background(), f.task)

	require.NoError(t, err)
	require.Equal(t, 0, f.client.calls, "没有候选就不该发起付费调用")
	require.Empty(t, f.alerts.upserted)
}

// updates 关系指向的目标是强候选：它只接受过时结论，重复结论一律丢弃。
func TestEvolveProcessorTreatsUpdateRelationAsOutdatedOnly(t *testing.T) {
	f := newEvolveFixture()
	f.addCandidate(21, "被更新的旧文", "旧版本的结论。", 0.95)
	f.relations.rows = []repository.RelationRecord{{
		KnowledgeRelation: model.KnowledgeRelation{
			SourceID: 10, TargetID: 21, RelationType: model.RelationTypeUpdates, Status: model.RelationConfirmed,
		},
		CounterpartID: 21,
	}}
	f.client.judgements = []EvolveJudgement{f.duplicateJudgement(21, "旧版本的结论")}

	_, err := f.processor.Process(context.Background(), f.task)

	require.NoError(t, err)
	require.Len(t, f.client.candidates, 1)
	require.Equal(t, EvolveOriginUpdateRelation, f.client.candidates[0].Origin)
	require.Empty(t, f.alerts.upserted, "强候选只接受过时结论")
}

// 强候选给出过时结论时落一条过时提醒，建议动作是「标记过时」。
func TestEvolveProcessorRecordsOutdatedFromUpdateRelation(t *testing.T) {
	f := newEvolveFixture()
	f.addCandidate(21, "被更新的旧文", "旧版本的结论。", 0.95)
	f.relations.rows = []repository.RelationRecord{{
		KnowledgeRelation: model.KnowledgeRelation{
			SourceID: 10, TargetID: 21, RelationType: model.RelationTypeUpdates, Status: model.RelationConfirmed,
		},
		CounterpartID: 21,
	}}
	f.client.judgements = []EvolveJudgement{{
		TargetID: 21,
		Type:     model.EvolutionAlertTypeOutdated,
		Reason:   "本次知识给出了更晚的结论。",
		Evidence: []EvolutionEvidence{
			{KnowledgeID: 10, Quote: "同一段正文"},
			{KnowledgeID: 21, Quote: "旧版本的结论"},
		},
		Severity: model.EvolutionSeverityStrong,
	}}

	_, err := f.processor.Process(context.Background(), f.task)

	require.NoError(t, err)
	require.Len(t, f.alerts.upserted, 1)
	alert := f.alerts.upserted[0]
	require.Equal(t, model.EvolutionAlertTypeOutdated, alert.AlertType)
	require.Equal(t, model.EvolutionSeverityStrong, alert.Severity)
	require.Equal(t, []uint{10, 21}, alert.SubjectIDList(), "触发知识在前，被比对的在后")
	payload := ParseEvolutionPayload(alert.Payload)
	require.Equal(t, model.EvolutionActionMarkOutdated, payload.SuggestedAction)
	require.Len(t, payload.Evidence, 2)
}

// 弱信号（向量近邻）判过时必须有双方原文对照：只有一侧引文就不落库。
func TestEvolveProcessorDropsWeakOutdatedWithoutTwoSidedEvidence(t *testing.T) {
	f := newEvolveFixture()
	f.addCandidate(21, "另一篇", "旧版本的结论。", 0.85)
	f.client.judgements = []EvolveJudgement{{
		TargetID: 21,
		Type:     model.EvolutionAlertTypeOutdated,
		Reason:   "候选的结论已被取代。",
		Evidence: []EvolutionEvidence{{KnowledgeID: 21, Quote: "旧版本的结论"}},
		Severity: model.EvolutionSeverityStrong,
	}}

	_, err := f.processor.Process(context.Background(), f.task)

	require.NoError(t, err)
	require.Empty(t, f.alerts.upserted, "弱信号缺一侧证据不足以判定过时")
}

// 弱信号两侧证据都在时正常落库。
func TestEvolveProcessorRecordsWeakOutdatedWithTwoSidedEvidence(t *testing.T) {
	f := newEvolveFixture()
	f.addCandidate(21, "另一篇", "旧版本的结论。", 0.85)
	f.client.judgements = []EvolveJudgement{{
		TargetID: 21,
		Type:     model.EvolutionAlertTypeOutdated,
		Reason:   "候选的结论已被取代。",
		Evidence: []EvolutionEvidence{
			{KnowledgeID: 10, Quote: "同一段正文"},
			{KnowledgeID: 21, Quote: "旧版本的结论"},
		},
		Severity: model.EvolutionSeverityStrong,
	}}

	_, err := f.processor.Process(context.Background(), f.task)

	require.NoError(t, err)
	require.Len(t, f.alerts.upserted, 1)
	require.Equal(t, model.EvolutionAlertTypeOutdated, f.alerts.upserted[0].AlertType)
}

// 同一知识的不同版本之间永不提醒：检查只针对别的条目，自身 ID 不参与候选。
func TestEvolveProcessorNeverAlertsOnOwnVersions(t *testing.T) {
	f := newEvolveFixture()
	// 向量召回返回本条知识自己（同一 knowledge_id 的历史版本共用同一行），必须被过滤。
	f.searcher.matches = []VectorMatch{{KnowledgeID: 10, Score: 0.99}}

	_, err := f.processor.Process(context.Background(), f.task)

	require.NoError(t, err)
	require.Empty(t, f.client.candidates)
	require.Equal(t, 0, f.client.calls)
	require.Empty(t, f.alerts.upserted)
}

// 反方向的 updates（本条是被更新的旧文）不构成强候选，否则会产出方向相反的提醒。
func TestEvolveProcessorIgnoresReverseUpdateRelation(t *testing.T) {
	f := newEvolveFixture()
	f.addCandidate(21, "更新的新文", "新结论。", 0.50)
	f.relations.rows = []repository.RelationRecord{{
		KnowledgeRelation: model.KnowledgeRelation{
			SourceID: 21, TargetID: 10, RelationType: model.RelationTypeUpdates, Status: model.RelationConfirmed,
		},
		CounterpartID: 21,
	}}

	_, err := f.processor.Process(context.Background(), f.task)

	require.NoError(t, err)
	require.Empty(t, f.client.candidates)
	require.Equal(t, 0, f.client.calls)
}

// 证据校验：引文在正文里定位不到，整条提醒不落库。
func TestEvolveProcessorDropsAlertWithoutLocatableEvidence(t *testing.T) {
	f := newEvolveFixture()
	f.addCandidate(21, "另一篇", "HNSW 用分层图做近似最近邻检索。", 0.93)
	judgement := f.duplicateJudgement(21, "这段引文根本不在任何正文里")
	f.client.judgements = []EvolveJudgement{judgement}

	_, err := f.processor.Process(context.Background(), f.task)

	require.NoError(t, err)
	require.Empty(t, f.alerts.upserted)
}

// 一侧证据定位不到时保留另一侧，并把严重度降级为复核。
func TestEvolveProcessorKeepsPartialEvidenceAndDowngrades(t *testing.T) {
	f := newEvolveFixture()
	f.addCandidate(21, "另一篇", "HNSW 用分层图做近似最近邻检索。", 0.93)
	f.client.judgements = []EvolveJudgement{{
		TargetID: 21,
		Type:     model.EvolutionAlertTypeDuplicate,
		Reason:   "同一件事",
		Evidence: []EvolutionEvidence{
			{KnowledgeID: 10, Quote: "同一段正文"},
			{KnowledgeID: 21, Quote: "正文里没有这句"},
		},
		Severity: model.EvolutionSeverityStrong,
	}}

	_, err := f.processor.Process(context.Background(), f.task)

	require.NoError(t, err)
	require.Len(t, f.alerts.upserted, 1)
	alert := f.alerts.upserted[0]
	require.Equal(t, model.EvolutionSeverityReview, alert.Severity, "证据不全时降级")
	parsed := ParseEvolutionPayload(alert.Payload)
	require.Len(t, parsed.Evidence, 1)
	require.Equal(t, uint(10), parsed.Evidence[0].KnowledgeID)
}

// 同一对知识只留一条提醒：模型重复给出同一目标的结论时不会落两条。
func TestEvolveProcessorDeduplicatesSameTarget(t *testing.T) {
	f := newEvolveFixture()
	f.addCandidate(21, "另一篇", "HNSW 用分层图做近似最近邻检索。", 0.93)
	f.client.judgements = []EvolveJudgement{
		f.duplicateJudgement(21, "近似最近邻检索"),
		f.duplicateJudgement(21, "分层图"),
	}

	_, err := f.processor.Process(context.Background(), f.task)

	require.NoError(t, err)
	require.Len(t, f.alerts.upserted, 1)
}

// 候选上限：向量召回按相似度降序，只取前八条送模型。
func TestEvolveProcessorCapsCandidates(t *testing.T) {
	f := newEvolveFixture()
	for i := uint(0); i < 10; i++ {
		f.addCandidate(30+i, "候选", "HNSW 内容", 0.90-float64(i)*0.005)
	}

	_, err := f.processor.Process(context.Background(), f.task)

	require.NoError(t, err)
	require.Len(t, f.client.candidates, EvolveCandidateMax)
	require.Equal(t, uint(30), f.client.candidates[0].KnowledgeID, "相似度最高的排最前")
	require.Equal(t, EvolveVectorTopK, f.searcher.topK)
}

// 候选全是归档知识时等同于没有候选：不发起付费调用。
func TestEvolveProcessorSkipsArchivedCandidates(t *testing.T) {
	f := newEvolveFixture()
	now := time.Now()
	f.knowledge.items[21] = &model.KnowledgeItem{ID: 21, UserID: 2, Title: "已归档", Content: "x", Status: model.KnowledgeReady, ArchivedAt: &now}
	f.searcher.matches = append(f.searcher.matches, VectorMatch{KnowledgeID: 21, Score: 0.95})

	_, err := f.processor.Process(context.Background(), f.task)

	require.NoError(t, err)
	require.Equal(t, 0, f.client.calls)
	require.Empty(t, f.alerts.upserted)
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

// 模型侧错误分类：结构非法与请求被拒是终态，其余留给重试。
func TestEvolveProcessorModelErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		name     string
		err      error
		terminal bool
	}{
		{"invalid output", ErrEvolveInvalidOutput, true},
		{"rejected", ErrEvolveRejected, true},
		{"transient", errors.New("AI 演化判定调用超时，请稍后重试"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newEvolveFixture()
			f.addCandidate(21, "另一篇", "HNSW 内容", 0.93)
			f.client.err = tc.err

			_, err := f.processor.Process(context.Background(), f.task)

			require.Error(t, err)
			var terminal *AiTaskTerminalError
			require.Equal(t, tc.terminal, errors.As(err, &terminal))
		})
	}
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

// 模型能力缺失时给中文提示，而不是拿空能力去发起调用。
func TestEvolveProcessorMissingCapabilities(t *testing.T) {
	f := newEvolveFixture()
	f.resolver.chatErr = errors.New("no chat model")
	_, err := f.processor.Process(context.Background(), f.task)
	require.Equal(t, "未配置可用的对话模型，请前往模型设置", requireTerminalError(t, err).Message)

	f = newEvolveFixture()
	f.resolver.embedErr = errors.New("no embedding model")
	_, err = f.processor.Process(context.Background(), f.task)
	require.Equal(t, "未配置可用的 Embedding 模型，请前往模型设置", requireTerminalError(t, err).Message)
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
