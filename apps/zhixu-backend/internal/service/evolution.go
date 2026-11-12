package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"
	"zhixu-backend/internal/utils"

	"gorm.io/gorm"
)

var (
	ErrEvolutionInvalid = errors.New("invalid evolution request")
	// ErrEvolutionNotFound 表示提醒不存在或不属于当前用户。
	ErrEvolutionNotFound = errors.New("evolution alert not found")
	// ErrEvolutionUnsupported 表示动作合法但对应的副作用要等后续迭代才接上。
	// 重复提醒只有归档、保留、核实、忽略四种动作，正常流程走不到这里。
	ErrEvolutionUnsupported = errors.New("该裁决动作尚未启用")
)

// 演化检测阈值。与关系推荐常量同风格，先进代码不进配置：调参只改这里，不动结构。
const (
	// EvolveDuplicateMinScore 只接受重复结论的相似度下限。
	EvolveDuplicateMinScore = 0.90
	// EvolveConflictMinScore 冲突判定的下限，与重复阈值之间留出带宽。
	EvolveConflictMinScore = 0.75
	// EvolveOutdatedMinScore 过时判定的弱信号候选门槛。
	EvolveOutdatedMinScore = 0.80
	// EvolveVectorTopK 向量召回条数：比关系推荐的 50 小，因为一次要给三类结论分派。
	EvolveVectorTopK = 30
	// EvolveCandidateMax 送模型的候选上限。
	EvolveCandidateMax = 8
	// EvolveExcerptMaxRunes 候选摘录长度，与关系推荐对齐。
	EvolveExcerptMaxRunes = 1200
	// EvolveQuoteMaxRunes 证据引文长度，与 AI 契约对齐。
	EvolveQuoteMaxRunes = 200
	// EvolveRevisedContentMaxRunes 修订稿长度上限，超过则丢弃修订稿。
	EvolveRevisedContentMaxRunes = 20000
	// EvolveAlertMaxPerTask 一次任务最多落几条提醒。
	EvolveAlertMaxPerTask = 5
)

// EvolutionEvidence 是一条证据：某条知识正文里的原话。
// 引文必须能在正文中定位，定位不到就不该作为证据展示给用户。
type EvolutionEvidence struct {
	KnowledgeID uint   `json:"knowledge_id"`
	Quote       string `json:"quote"`
}

// EvolutionPayload 是提醒的结构化判定结果，整体存在提醒行的 payload 列里。
// 冲突场景额外带修订稿；修订稿缺失时前端把「采纳修订」置灰。
type EvolutionPayload struct {
	Type            model.EvolutionAlertType `json:"type"`
	Reason          string                   `json:"reason"`
	Evidence        []EvolutionEvidence      `json:"evidence"`
	SuggestedAction model.EvolutionAction    `json:"suggested_action"`
	RevisedTitle    string                   `json:"revised_title,omitempty"`
	RevisedContent  string                   `json:"revised_content,omitempty"`
}

// ParseEvolutionPayload 解析载荷；脏数据返回空载荷，不让列表整体失败。
func ParseEvolutionPayload(raw string) EvolutionPayload {
	var payload EvolutionPayload
	if raw == "" {
		return payload
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return EvolutionPayload{}
	}
	return payload
}

// evolutionEvidenceFingerprint 把证据数组压成一个稳定字符串：
// 先按知识 ID、再按引文排序，避免同一组证据因顺序不同算出两个指纹。
func evolutionEvidenceFingerprint(evidence []EvolutionEvidence) string {
	items := make([]EvolutionEvidence, len(evidence))
	copy(items, evidence)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].KnowledgeID != items[j].KnowledgeID {
			return items[i].KnowledgeID < items[j].KnowledgeID
		}
		return items[i].Quote < items[j].Quote
	})
	var builder strings.Builder
	for _, item := range items {
		builder.WriteString(fmt.Sprintf("%d:%s\n", item.KnowledgeID, item.Quote))
	}
	return builder.String()
}

// EvolutionDedupeKey 计算提醒指纹：类型 + 排序后的涉及知识 ID + 证据引文指纹。
// 与 AI 任务的幂等键不同，这个指纹在终态也保留，用来抑制同一证据重复弹提醒；
// 引文随正文变化时指纹随之改变，于是同一条知识的新问题会被当成新提醒。
func EvolutionDedupeKey(alertType model.EvolutionAlertType, subjectIDs []uint, evidence []EvolutionEvidence) string {
	ids := make([]uint, len(subjectIDs))
	copy(ids, subjectIDs)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, fmt.Sprintf("%d", id))
	}
	raw := fmt.Sprintf("%s|%s|%s", alertType, strings.Join(parts, ","), evolutionEvidenceFingerprint(evidence))
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// validEvidence 过滤掉在正文里定位不到的引文。
// 正文与引文都先规范化再比较：换行符与行尾空白的差异不该让一条真实证据失效。
func validEvidence(body string, evidence []EvolutionEvidence) []EvolutionEvidence {
	if len(evidence) == 0 {
		return nil
	}
	normalizedBody := utils.NormalizeKnowledgeContent(body)
	kept := make([]EvolutionEvidence, 0, len(evidence))
	for _, item := range evidence {
		quote := utils.NormalizeKnowledgeContent(item.Quote)
		if quote == "" || !strings.Contains(normalizedBody, quote) {
			continue
		}
		kept = append(kept, EvolutionEvidence{KnowledgeID: item.KnowledgeID, Quote: quote})
	}
	return kept
}

// evolutionExcerpt 取正文前若干字符作为证据引文，按字符而非字节截断，避免切碎多字节字符。
func evolutionExcerpt(body string, limit int) string {
	normalized := utils.NormalizeKnowledgeContent(body)
	if utf8.RuneCountInString(normalized) <= limit {
		return normalized
	}
	return string([]rune(normalized)[:limit])
}

// EvolutionKnowledgeView 是提醒里一侧知识的展示信息。
type EvolutionKnowledgeView struct {
	ID         uint   `json:"id"`
	Title      string `json:"title"`
	SourceType string `json:"source_type"`
	Archived   bool   `json:"archived"`
	Outdated   bool   `json:"outdated"`
}

// EvolutionAlertView 是对外暴露的提醒快照：把有序 ID 数组与载荷展开成前端可直接渲染的形状。
type EvolutionAlertView struct {
	ID             uint                         `json:"id"`
	AlertType      model.EvolutionAlertType     `json:"alert_type"`
	Status         model.EvolutionAlertStatus   `json:"status"`
	Severity       model.EvolutionAlertSeverity `json:"severity"`
	Subjects       []EvolutionKnowledgeView     `json:"subjects"`
	Payload        EvolutionPayload             `json:"payload"`
	ResolvedAt     *time.Time                   `json:"resolved_at"`
	ResolvedAction model.EvolutionAction        `json:"resolved_action"`
	ResolutionNote string                       `json:"resolution_note"`
	CreatedAt      time.Time                    `json:"created_at"`
	UpdatedAt      time.Time                    `json:"updated_at"`
}

// EvolutionSummaryView 是首页卡片的数据源：两个待办总数加按类型分组计数。
type EvolutionSummaryView struct {
	Pending  int64 `json:"pending"`
	Deferred int64 `json:"deferred"`
	ByType   struct {
		Duplicate int64 `json:"duplicate"`
		Outdated  int64 `json:"outdated"`
		Conflict  int64 `json:"conflict"`
	} `json:"by_type"`
}

// EvolutionAlertStore 是提醒持久化接口（*repository.EvolutionAlertRepository 满足）。
type EvolutionAlertStore interface {
	List(context.Context, uint, repository.EvolutionAlertFilter) ([]model.EvolutionAlert, int64, error)
	Get(context.Context, uint, uint) (*model.EvolutionAlert, error)
	Summary(context.Context, uint) ([]repository.EvolutionAlertTypeCount, error)
	Resolve(context.Context, uint, uint, model.EvolutionAction, string, model.EvolutionAlertStatus, func(*gorm.DB, *model.EvolutionAlert) error) (*model.EvolutionAlert, bool, error)
}

// EvolutionKnowledgeSource 读取提醒涉及的知识（*KnowledgeService 满足）。
type EvolutionKnowledgeSource interface {
	Get(context.Context, uint, uint) (*model.KnowledgeItem, error)
	ListByIDs(context.Context, uint, []uint) ([]model.KnowledgeItem, error)
}

// EvolutionArchiver 是「归档其一」需要的知识归档能力（*KnowledgeService 满足）。
type EvolutionArchiver interface {
	Archive(context.Context, uint, uint) (*time.Time, error)
}

// EvolutionOutdatedMarker 是「标记过时 / 标记已核实」需要的知识过时标记能力（*KnowledgeService 满足）。
// at 为 nil 表示清除标记。
type EvolutionOutdatedMarker interface {
	SetOutdated(context.Context, uint, uint, *time.Time, string) error
}

// EvolutionService 负责提醒的读取、裁决与手动重跑；检测本身在 evolve 任务处理器里。
type EvolutionService struct {
	Alerts    EvolutionAlertStore
	Knowledge EvolutionKnowledgeSource
	// Archiver 为可选依赖：为 nil 时「归档其一」不可用（测试与未接知识服务的部署）。
	Archiver EvolutionArchiver
	// Outdated 为可选依赖：为 nil 时「标记过时 / 标记已核实」不可用。
	Outdated EvolutionOutdatedMarker
	// Logs 为可选依赖：为 nil 时跳过操作日志。
	Logs OperationLogWriter
	// Tasks 为可选依赖：为 nil 时「重新检查」不可用。
	Tasks *AiTaskService
}

// EvolutionResolveInput 是一次裁决请求；BaseStatus 可选，用于乐观校验。
type EvolutionResolveInput struct {
	Action     model.EvolutionAction
	Note       string
	BaseStatus model.EvolutionAlertStatus
}

// List 分页查询提醒，并把涉及知识补成可展示的标题与状态。
func (s *EvolutionService) List(ctx context.Context, userID uint, page, size int, alertType model.EvolutionAlertType, status model.EvolutionAlertStatus, open bool) ([]EvolutionAlertView, int64, error) {
	if page < 1 || page > 1000000 || size < 1 || size > 100 {
		return nil, 0, ErrEvolutionInvalid
	}
	if alertType != "" && !alertType.Valid() {
		return nil, 0, ErrEvolutionInvalid
	}
	if status != "" && !status.Valid() {
		return nil, 0, ErrEvolutionInvalid
	}
	alerts, total, err := s.Alerts.List(ctx, userID, repository.EvolutionAlertFilter{
		Page: page, Size: size, Type: alertType, Status: status, Open: open,
	})
	if err != nil {
		return nil, 0, err
	}
	views, err := s.views(ctx, userID, alerts)
	if err != nil {
		return nil, 0, err
	}
	return views, total, nil
}

// Summary 返回待处理与暂不处理的计数，供首页卡片使用。
func (s *EvolutionService) Summary(ctx context.Context, userID uint) (*EvolutionSummaryView, error) {
	rows, err := s.Alerts.Summary(ctx, userID)
	if err != nil {
		return nil, err
	}
	view := &EvolutionSummaryView{}
	for _, row := range rows {
		if row.Status == model.EvolutionAlertDeferred {
			view.Deferred += row.Count
		} else {
			view.Pending += row.Count
		}
		switch row.AlertType {
		case model.EvolutionAlertTypeDuplicate:
			view.ByType.Duplicate += row.Count
		case model.EvolutionAlertTypeOutdated:
			view.ByType.Outdated += row.Count
		case model.EvolutionAlertTypeConflict:
			view.ByType.Conflict += row.Count
		}
	}
	return view, nil
}

// Resolve 对一条提醒执行裁决。已不在待办视野的提醒按幂等处理：
// 返回当前提醒、不执行动作、不写日志。
func (s *EvolutionService) Resolve(ctx context.Context, userID, alertID uint, input EvolutionResolveInput) (*EvolutionAlertView, bool, error) {
	if !input.Action.Valid() {
		return nil, false, ErrEvolutionInvalid
	}
	alert, err := s.Alerts.Get(ctx, userID, alertID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, ErrEvolutionNotFound
		}
		return nil, false, err
	}
	if !input.Action.AllowedFor(alert.AlertType) {
		return nil, false, fmt.Errorf("%w: 该动作不适用于这类提醒", ErrEvolutionInvalid)
	}
	apply, err := s.effectFor(ctx, input.Action)
	if err != nil {
		return nil, false, err
	}
	resolved, changed, err := s.Alerts.Resolve(ctx, userID, alertID, input.Action, strings.TrimSpace(input.Note), input.BaseStatus, apply)
	if err != nil {
		return nil, false, err
	}
	views, err := s.views(ctx, userID, []model.EvolutionAlert{*resolved})
	if err != nil {
		return nil, false, err
	}
	if changed {
		s.recordResolve(userID, resolved, input.Action)
	}
	return &views[0], changed, nil
}

// effectFor 把动作翻译成知识侧的副作用。没有副作用的动作（保留两者、标记已核实、忽略、暂不处理）返回 nil。
func (s *EvolutionService) effectFor(ctx context.Context, action model.EvolutionAction) (func(*gorm.DB, *model.EvolutionAlert) error, error) {
	switch action {
	case model.EvolutionActionArchiveOne:
		if s.Archiver == nil {
			return nil, fmt.Errorf("%w: 知识归档能力未配置", ErrEvolutionUnsupported)
		}
		return func(_ *gorm.DB, alert *model.EvolutionAlert) error {
			ids := alert.SubjectIDList()
			if len(ids) < 2 {
				return fmt.Errorf("%w: 提醒缺少涉及知识", ErrEvolutionInvalid)
			}
			// 归档后创建的那一条，保留先入库的原件：原件通常已整理、已建关系，
			// 而副本是刚导入的。按 ID 大小判定与触发方向无关，重复检查从任一侧发起结果一致。
			victim := ids[0]
			if ids[1] > victim {
				victim = ids[1]
			}
			if _, err := s.Archiver.Archive(ctx, alert.UserID, victim); err != nil {
				return err
			}
			return nil
		}, nil
	case model.EvolutionActionMarkOutdated:
		if s.Outdated == nil {
			return nil, fmt.Errorf("%w: 知识过时标记能力未配置", ErrEvolutionUnsupported)
		}
		return func(_ *gorm.DB, alert *model.EvolutionAlert) error {
			target, err := outdatedSubject(alert)
			if err != nil {
				return err
			}
			reason := ParseEvolutionPayload(alert.Payload).Reason
			if reason == "" {
				reason = "已被更新的一条知识取代。"
			}
			now := time.Now()
			return s.Outdated.SetOutdated(ctx, alert.UserID, target, &now, reason)
		}, nil
	case model.EvolutionActionMarkVerified:
		// 核实本身是「这条提醒我看过了」：没有标记能力时它仍然有意义，只是清不掉过时标记。
		if s.Outdated == nil {
			return nil, nil
		}
		return func(_ *gorm.DB, alert *model.EvolutionAlert) error {
			// 只有过时提醒的「标记已核实」才意味着「我看过了，这条不过时」；
			// 重复提醒的核实不该顺手抹掉别的判定留下的过时标记。
			if alert.AlertType != model.EvolutionAlertTypeOutdated {
				return nil
			}
			target, err := outdatedSubject(alert)
			if err != nil {
				return err
			}
			return s.Outdated.SetOutdated(ctx, alert.UserID, target, nil, "")
		}, nil
	case model.EvolutionActionAcceptRevision, model.EvolutionActionCreateRelation:
		// 两种动作依赖后续迭代的能力：采纳修订要走 AI 版本链路，矛盾关系随冲突识别落地。
		return nil, ErrEvolutionUnsupported
	default:
		return nil, nil
	}
}

// outdatedSubject 取过时提醒里被判定过时的那条知识：subject_ids[1] 是被比对的知识，
// 本次触发的知识（[0]）是新的一方。过时只做跨条目判定，因此这里必须落在 [1] 上。
func outdatedSubject(alert *model.EvolutionAlert) (uint, error) {
	ids := alert.SubjectIDList()
	if len(ids) < 2 {
		return 0, fmt.Errorf("%w: 提醒缺少涉及知识", ErrEvolutionInvalid)
	}
	return ids[1], nil
}

// recordResolve 写一条裁决操作日志；日志失败不影响裁决结果。
func (s *EvolutionService) recordResolve(userID uint, alert *model.EvolutionAlert, action model.EvolutionAction) {
	if s.Logs == nil {
		return
	}
	// 目标是提醒本身而不是知识：把提醒 ID 挂在 knowledge 类型下会让前端按 ID 打开错误的知识。
	writeOperationLog(s.Logs, userID, model.OperationLogActionEvolutionResolve, model.OperationLogTargetTypeEvolution,
		alert.ID, fmt.Sprintf("演化提醒 #%d", alert.ID), "", map[string]any{
			"alert_type": alert.AlertType, "action": action,
		})
}

// Regenerate 手动重跑一条知识的演化检查；已有活跃任务时复用，不重复入队。
func (s *EvolutionService) Regenerate(ctx context.Context, userID, knowledgeID uint) (*AiTaskView, error) {
	if s.Tasks == nil {
		return nil, fmt.Errorf("%w: 演化检查未启用", ErrEvolutionUnsupported)
	}
	item, err := s.Knowledge.Get(ctx, userID, knowledgeID)
	if err != nil {
		return nil, err
	}
	if item.ArchivedAt != nil {
		return nil, fmt.Errorf("%w: 知识已归档，无法检查", ErrEvolutionInvalid)
	}
	if item.Status != model.KnowledgeReady {
		return nil, fmt.Errorf("%w: 知识尚未就绪，无法检查", ErrEvolutionInvalid)
	}
	task, err := s.Tasks.EnqueueEvolveTask(ctx, userID, knowledgeID, item.Version)
	if err != nil {
		return nil, err
	}
	return NewAiTaskView(task), nil
}

// views 把提醒行补成展示视图：一次批量取知识，避免逐条查询。
func (s *EvolutionService) views(ctx context.Context, userID uint, alerts []model.EvolutionAlert) ([]EvolutionAlertView, error) {
	views := make([]EvolutionAlertView, 0, len(alerts))
	if len(alerts) == 0 {
		return views, nil
	}
	ids := make([]uint, 0, len(alerts)*2)
	for _, alert := range alerts {
		ids = append(ids, alert.SubjectIDList()...)
	}
	items, err := s.Knowledge.ListByIDs(ctx, userID, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[uint]model.KnowledgeItem, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	for _, alert := range alerts {
		view := EvolutionAlertView{
			ID: alert.ID, AlertType: alert.AlertType, Status: alert.Status, Severity: alert.Severity,
			Payload: ParseEvolutionPayload(alert.Payload), ResolvedAt: alert.ResolvedAt,
			ResolvedAction: alert.ResolvedAction, ResolutionNote: alert.ResolutionNote,
			CreatedAt: alert.CreatedAt, UpdatedAt: alert.UpdatedAt,
		}
		for _, id := range alert.SubjectIDList() {
			subject := EvolutionKnowledgeView{ID: id}
			if item, ok := byID[id]; ok {
				subject.Title, subject.SourceType = item.Title, item.SourceType
				subject.Archived, subject.Outdated = item.ArchivedAt != nil, item.OutdatedAt != nil
			}
			view.Subjects = append(view.Subjects, subject)
		}
		views = append(views, view)
	}
	return views, nil
}
