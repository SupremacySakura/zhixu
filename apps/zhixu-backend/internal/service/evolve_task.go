package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"zhixu-backend/internal/log"
	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"

	"gorm.io/gorm"
)

// 依赖保持最小接口，便于服务层测试注入 fake；形状与关系任务处理器一致。
// 判定用对话模型，候选召回用嵌入模型——两种能力缺一不可。
type evolveChatResolver interface {
	ResolveChatModel(context.Context, uint) (*ChatCapability, error)
}

type evolveEmbeddingResolver interface {
	ResolveEmbedding(context.Context, uint) (*ChatCapability, uint64, uint64, error)
}

// evolveKnowledgeSource 读取触发知识与候选知识（*KnowledgeService 满足）。
type evolveKnowledgeSource interface {
	Get(context.Context, uint, uint) (*model.KnowledgeItem, error)
	FindByContentHash(context.Context, uint, string, uint) (*model.KnowledgeItem, error)
	ListByIDs(context.Context, uint, []uint) ([]model.KnowledgeItem, error)
}

// evolveRelationStore 读取已建立的关系，用于把 updates 指向的目标当成强候选。
type evolveRelationStore interface {
	ListByKnowledge(context.Context, uint, uint) ([]repository.RelationRecord, error)
}

type evolveVectorSearcher interface {
	Search(context.Context, string, uint, []float64, int, float64) ([]VectorMatch, error)
}

type evolveSuggester interface {
	Evolve(context.Context, *ChatCapability, uint, uint, string, string, []EvolveCandidate) ([]EvolveJudgement, error)
}

// evolveAlertStore 写入提醒（*repository.EvolutionAlertRepository 满足）。
type evolveAlertStore interface {
	Upsert(context.Context, *model.EvolutionAlert) (repository.EvolutionUpsertOutcome, error)
}

// evolveCandidateItem 是一条候选的完整上下文：判定要用来源与相似度分派允许类型，
// 证据校验要用正文定位引文，因此不能只留 ID。
type evolveCandidateItem struct {
	item   model.KnowledgeItem
	score  float64
	origin string
}

// EvolveProcessor 是演化检查工作流的执行入口：一条触发知识产出 0~N 条提醒。
//
// 检测分两段：先按正文指纹做精确重复（确定性判定，不花钱），再生成候选集送模型一次判定。
// 本迭代只消费模型给出的重复结论；过时与冲突在后续迭代接入同一个处理器，
// 判定、证据校验与落库管线已经就位。
type EvolveProcessor struct {
	Knowledge evolveKnowledgeSource
	Alerts    evolveAlertStore
	Chat      evolveChatResolver
	Embedding evolveEmbeddingResolver
	Embedder  VectorEmbedder
	Vectors   evolveVectorSearcher
	Relations evolveRelationStore
	Client    evolveSuggester
}

func (p *EvolveProcessor) TaskType() string { return model.AiTaskTypeEvolve }

func (p *EvolveProcessor) Process(ctx context.Context, task *model.AiTask) (json.RawMessage, error) {
	if p == nil || p.Knowledge == nil || p.Alerts == nil || p.Chat == nil || p.Embedding == nil ||
		p.Embedder == nil || p.Vectors == nil || p.Relations == nil || p.Client == nil {
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

	// 精确重复是确定性判定：命中就不必再花钱问模型，直接给一条最明确的提醒。
	duplicate, err := p.Knowledge.FindByContentHash(ctx, task.UserID, item.ContentHash, item.ID)
	switch {
	case err == nil:
		created, err := p.recordDuplicate(ctx, item, duplicate)
		if err != nil {
			return nil, err
		}
		if created {
			return p.result(item.ID, 1)
		}
		return p.result(item.ID, 0)
	case errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, ErrKnowledgeNotFound):
	default:
		return nil, err
	}

	chat, err := p.Chat.ResolveChatModel(ctx, task.UserID)
	if err != nil {
		return nil, TerminalTaskError("未配置可用的对话模型，请前往模型设置")
	}
	candidates, err := p.candidates(ctx, task.UserID, item)
	if err != nil {
		return nil, err
	}
	// 没有任何候选就没有可比对的对象：任务成功但零提醒，不发起付费调用。
	if len(candidates) == 0 {
		return p.result(item.ID, 0)
	}

	judgeExcerpt := relationSourceBody(item, RelationKnowledgeExcerptMaxRunes)
	judgements, err := p.Client.Evolve(ctx, chat, task.ID, item.ID, item.Title, judgeExcerpt, p.clientCandidates(candidates))
	if err != nil {
		if errors.Is(err, ErrEvolveInvalidOutput) || errors.Is(err, ErrEvolveRejected) {
			return nil, TerminalTaskError(err.Error())
		}
		return nil, err
	}
	alerts := p.buildAlerts(item, candidates, judgements)
	created := 0
	for i := range alerts {
		outcome, err := p.Alerts.Upsert(ctx, &alerts[i])
		if err != nil {
			return nil, err
		}
		if outcome == repository.EvolutionAlertSuppressed {
			log.Info().Uint("knowledge_id", item.ID).Msg("同证据的演化提醒已处理过，本次不再提醒")
			continue
		}
		if outcome == repository.EvolutionAlertCreated || outcome == repository.EvolutionAlertRevived {
			created++
		}
	}
	return p.result(item.ID, created)
}

// candidates 合并两路候选：updates 关系指向的目标是用户显式声明过的强信号，排在最前；
// 向量近邻按相似度降序补齐。同一对知识只出现一次，强信号优先。
func (p *EvolveProcessor) candidates(ctx context.Context, userID uint, item *model.KnowledgeItem) ([]evolveCandidateItem, error) {
	relations, err := p.Relations.ListByKnowledge(ctx, userID, item.ID)
	if err != nil {
		return nil, err
	}
	// 只认「本条知识更新了对方」的方向：反方向的 updates 说明本条是被更新的旧知识，
	// 该由对方那次检查去判过时，这里重复判定只会产出方向相反的提醒。
	strongIDs := make([]uint, 0, len(relations))
	for _, relation := range relations {
		if relation.RelationType != model.RelationTypeUpdates || relation.SourceID != item.ID || relation.CounterpartID == 0 {
			continue
		}
		strongIDs = append(strongIDs, relation.CounterpartID)
	}

	// 语义重复与冲突都靠向量召回发现，因此嵌入能力是必需的。
	snapshot, version, _, err := p.Embedding.ResolveEmbedding(ctx, userID)
	if err != nil {
		return nil, TerminalTaskError("未配置可用的 Embedding 模型，请前往模型设置")
	}
	embeddingBody := relationSourceBody(item, RelationEmbeddingBodyMaxRunes)
	if embeddingBody == "" {
		return nil, TerminalTaskError("知识尚未就绪，无法执行演化检查")
	}
	embeddingText := truncateRunes(item.Title+"\n"+embeddingBody, RelationEmbeddingTextMaxRunes)
	embedCtx, cancel := context.WithTimeout(ctx, 110*time.Second)
	vectors, err := p.Embedder.Embed(embedCtx, snapshot, []string{embeddingText})
	cancel()
	if err != nil {
		var rejected *EmbeddingRejectedError
		if errors.As(err, &rejected) {
			return nil, TerminalTaskError(rejected.Error())
		}
		return nil, err
	}
	if len(vectors) != 1 || len(vectors[0]) == 0 {
		return nil, errors.New("Embedding 服务返回了空向量，请稍后重试")
	}
	matches, err := p.Vectors.Search(ctx, vectorCollection(userID, version), userID, vectors[0], EvolveVectorTopK, 0)
	if err != nil {
		return nil, err
	}
	// 一个知识有多个分块命中时只保留最高分，避免同一候选重复占用名额。
	best := make(map[uint]float64, len(matches))
	for _, match := range matches {
		if match.KnowledgeID == 0 || match.KnowledgeID == item.ID || match.Score < EvolveOutdatedMinScore {
			continue
		}
		if score, ok := best[match.KnowledgeID]; !ok || match.Score > score {
			best[match.KnowledgeID] = match.Score
		}
	}
	ranked := make([]uint, 0, len(best))
	for id := range best {
		ranked = append(ranked, id)
	}
	sort.Slice(ranked, func(i, j int) bool {
		if best[ranked[i]] == best[ranked[j]] {
			return ranked[i] < ranked[j]
		}
		return best[ranked[i]] > best[ranked[j]]
	})

	ids := make([]uint, 0, len(strongIDs)+len(ranked))
	ids = append(ids, strongIDs...)
	ids = append(ids, ranked...)
	details, err := p.Knowledge.ListByIDs(ctx, userID, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[uint]model.KnowledgeItem, len(details))
	for _, detail := range details {
		byID[detail.ID] = detail
	}
	out := make([]evolveCandidateItem, 0, EvolveCandidateMax)
	seen := make(map[uint]bool, EvolveCandidateMax)
	appendCandidate := func(id uint, score float64, origin string) {
		if len(out) == EvolveCandidateMax || seen[id] || id == item.ID {
			return
		}
		detail, ok := byID[id]
		// 已归档的知识不能再被裁决（既不能归档也不能标记过时），不进候选。
		if !ok || detail.ArchivedAt != nil {
			return
		}
		seen[id] = true
		out = append(out, evolveCandidateItem{item: detail, score: score, origin: origin})
	}
	for _, id := range strongIDs {
		appendCandidate(id, 0, EvolveOriginUpdateRelation)
	}
	for _, id := range ranked {
		appendCandidate(id, best[id], EvolveOriginVectorNeighbor)
	}
	return out, nil
}

// clientCandidates 把内部候选转成 /evolve 请求体。
func (p *EvolveProcessor) clientCandidates(candidates []evolveCandidateItem) []EvolveCandidate {
	out := make([]EvolveCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, EvolveCandidate{
			KnowledgeID: candidate.item.ID,
			Title:       candidate.item.Title,
			Excerpt:     relationSourceBody(&candidate.item, EvolveExcerptMaxRunes),
			Score:       candidate.score,
			Origin:      candidate.origin,
		})
	}
	return out
}

// buildAlerts 把模型结论翻译成待落库的提醒：分派类型、校验证据、按上限截断。
func (p *EvolveProcessor) buildAlerts(trigger *model.KnowledgeItem, candidates []evolveCandidateItem, judgements []EvolveJudgement) []model.EvolutionAlert {
	byID := make(map[uint]evolveCandidateItem, len(candidates))
	for _, candidate := range candidates {
		byID[candidate.item.ID] = candidate
	}
	// 同一对知识只留一条结论，重复优先：它的处置（归档其一）比冲突更明确。
	ordered := make([]EvolveJudgement, len(judgements))
	copy(ordered, judgements)
	sort.SliceStable(ordered, func(i, j int) bool {
		return evolveTypePriority(ordered[i].Type) < evolveTypePriority(ordered[j].Type)
	})
	seen := make(map[uint]bool, len(ordered))
	alerts := make([]model.EvolutionAlert, 0, EvolveAlertMaxPerTask)
	for _, judgement := range ordered {
		if len(alerts) == EvolveAlertMaxPerTask {
			break
		}
		candidate, ok := byID[judgement.TargetID]
		if !ok || seen[judgement.TargetID] {
			continue
		}
		// 相似度分派：候选越像越可能重复，中等相似才可能是同一事实的不同说法。
		if !evolutionAllowedTypes(candidate.origin, candidate.score)[judgement.Type] {
			continue
		}
		alert, ok := p.buildAlert(trigger, candidate, judgement)
		if !ok {
			continue
		}
		seen[judgement.TargetID] = true
		alerts = append(alerts, alert)
	}
	// 证据充分的排在前面：循环已按上限截断，这里只调整落库顺序。
	sort.SliceStable(alerts, func(i, j int) bool {
		return evolveSeverityRank(alerts[i].Severity) < evolveSeverityRank(alerts[j].Severity)
	})
	return alerts
}

// buildAlert 组装一条提醒；证据全被剔除时返回 false（整条不落库）。
func (p *EvolveProcessor) buildAlert(trigger *model.KnowledgeItem, candidate evolveCandidateItem, judgement EvolveJudgement) (model.EvolutionAlert, bool) {
	// 引文必须能在对应正文里定位：模型偶尔会改写或拼接引文，那样的证据给用户看没有价值。
	triggerEvidence := make([]EvolutionEvidence, 0, len(judgement.Evidence))
	candidateEvidence := make([]EvolutionEvidence, 0, len(judgement.Evidence))
	for _, evidence := range judgement.Evidence {
		switch evidence.KnowledgeID {
		case trigger.ID:
			triggerEvidence = append(triggerEvidence, evidence)
		case candidate.item.ID:
			candidateEvidence = append(candidateEvidence, evidence)
		}
	}
	// 证据顺序与 subject 顺序对齐：先本次触发的知识，再被比对的知识，前端左右并排才对得上。
	triggerValid := validEvidence(trigger.Content, triggerEvidence)
	candidateValid := validEvidence(candidate.item.Content, candidateEvidence)
	if len(triggerValid)+len(candidateValid) == 0 {
		return model.EvolutionAlert{}, false
	}
	// 过时只有「本条更新了对方」是用户显式声明过的强信号，可以直接判定；
	// 向量近邻只是相似，要说「候选已被取代」必须有双方原文对照，单侧证据不足以下这个结论。
	if judgement.Type == model.EvolutionAlertTypeOutdated && candidate.origin != EvolveOriginUpdateRelation &&
		(len(triggerValid) == 0 || len(candidateValid) == 0) {
		return model.EvolutionAlert{}, false
	}
	evidence := make([]EvolutionEvidence, 0, len(triggerValid)+len(candidateValid))
	evidence = append(evidence, triggerValid...)
	evidence = append(evidence, candidateValid...)
	severity := judgement.Severity
	if !severity.Valid() {
		severity = model.EvolutionSeverityReview
	}
	payload := EvolutionPayload{
		Type:     judgement.Type,
		Reason:   judgement.Reason,
		Evidence: evidence,
	}
	// 修订稿只在证据全部校验通过时保留：缺一条引文说明模型对这条知识的判断没有原文支撑，
	// 拿它去覆盖正文风险太大，降级为「仅提示、无修订稿」，前端会把采纳修订置灰。
	// 长度门槛在这里再守一道：客户端清洗已经挡过模型输出，但处理器可能被别的调用方直接喂判定。
	if len(evidence) == len(judgement.Evidence) && judgement.Type == model.EvolutionAlertTypeConflict &&
		evolveRevisionUsable(judgement.RevisedTitle, judgement.RevisedContent) {
		payload.RevisedTitle = judgement.RevisedTitle
		payload.RevisedContent = judgement.RevisedContent
	} else if len(evidence) < len(judgement.Evidence) || judgement.RevisedContent != "" {
		// 有修订稿却被丢弃时同样降级：让用户知道这条提醒只剩「仅提示」的分量。
		severity = model.EvolutionSeverityReview
	}
	// 建议动作跟着「修订稿是否留下」走：修订稿被丢弃后还推荐采纳修订，用户点下去只会拿到 400。
	payload.SuggestedAction = evolveSuggestedAction(payload)
	body, err := json.Marshal(payload)
	if err != nil {
		return model.EvolutionAlert{}, false
	}
	alert := model.EvolutionAlert{
		UserID:    trigger.UserID,
		AlertType: judgement.Type,
		Status:    model.EvolutionAlertPending,
		Severity:  severity,
		Payload:   string(body),
		// 指纹按排序后的 ID 计算：同一对知识无论从哪一侧发起检查都是同一条提醒。
		DedupeKey: EvolutionDedupeKey(judgement.Type, []uint{trigger.ID, candidate.item.ID}, evidence),
	}
	if err := alert.SetSubjectIDs([]uint{trigger.ID, candidate.item.ID}); err != nil {
		return model.EvolutionAlert{}, false
	}
	return alert, true
}

// evolveSuggestedAction 给出一条默认建议动作，供前端预选按钮。
func evolveSuggestedAction(payload EvolutionPayload) model.EvolutionAction {
	switch payload.Type {
	case model.EvolutionAlertTypeDuplicate:
		return model.EvolutionActionArchiveOne
	case model.EvolutionAlertTypeOutdated:
		return model.EvolutionActionMarkOutdated
	case model.EvolutionAlertTypeConflict:
		// 有修订稿时「采纳修订」是最省事的一步，没有就只能建矛盾关系。
		if payload.RevisedContent != "" {
			return model.EvolutionActionAcceptRevision
		}
		return model.EvolutionActionCreateRelation
	default:
		return model.EvolutionActionMarkVerified
	}
}

// evolveTypePriority 用于「同一对只留一条」的排序：重复优先于冲突，冲突优先于过时。
func evolveTypePriority(alertType model.EvolutionAlertType) int {
	switch alertType {
	case model.EvolutionAlertTypeDuplicate:
		return 0
	case model.EvolutionAlertTypeConflict:
		return 1
	case model.EvolutionAlertTypeOutdated:
		return 2
	default:
		return 3
	}
}

// evolveSeverityRank 用于截断排序：strong 先落库。
func evolveSeverityRank(severity model.EvolutionAlertSeverity) int {
	if severity == model.EvolutionSeverityStrong {
		return 0
	}
	return 1
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
