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

// evolutionStoreFake 用内存切片模拟提醒仓储：只实现服务真正用到的方法。
type evolutionStoreFake struct {
	alerts     []model.EvolutionAlert
	listCalls  int
	filter     repository.EvolutionAlertFilter
	resolveErr error
	applyCalls int
}

func (f *evolutionStoreFake) List(_ context.Context, userID uint, filter repository.EvolutionAlertFilter) ([]model.EvolutionAlert, int64, error) {
	f.listCalls++
	f.filter = filter
	out := make([]model.EvolutionAlert, 0, len(f.alerts))
	for _, alert := range f.alerts {
		if alert.UserID != userID {
			continue
		}
		if filter.Type != "" && alert.AlertType != filter.Type {
			continue
		}
		if filter.Open && !alert.Status.Open() {
			continue
		}
		if !filter.Open && filter.Status != "" && alert.Status != filter.Status {
			continue
		}
		out = append(out, alert)
	}
	return out, int64(len(out)), nil
}

func (f *evolutionStoreFake) Get(_ context.Context, userID, id uint) (*model.EvolutionAlert, error) {
	for i := range f.alerts {
		if f.alerts[i].ID == id && f.alerts[i].UserID == userID {
			return &f.alerts[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *evolutionStoreFake) Summary(_ context.Context, userID uint) ([]repository.EvolutionAlertTypeCount, error) {
	counts := map[string]*repository.EvolutionAlertTypeCount{}
	for _, alert := range f.alerts {
		if alert.UserID != userID || !alert.Status.Open() {
			continue
		}
		key := string(alert.Status) + "|" + string(alert.AlertType)
		row, ok := counts[key]
		if !ok {
			row = &repository.EvolutionAlertTypeCount{Status: alert.Status, AlertType: alert.AlertType}
			counts[key] = row
		}
		row.Count++
	}
	out := make([]repository.EvolutionAlertTypeCount, 0, len(counts))
	for _, row := range counts {
		out = append(out, *row)
	}
	return out, nil
}

// Resolve 复刻仓储语义：已不在待办视野时返回 changed=false 且不执行副作用。
func (f *evolutionStoreFake) Resolve(_ context.Context, userID, id uint, action model.EvolutionAction, note string, expected model.EvolutionAlertStatus, apply func(*gorm.DB, *model.EvolutionAlert) error) (*model.EvolutionAlert, bool, error) {
	if f.resolveErr != nil {
		return nil, false, f.resolveErr
	}
	alert, err := f.Get(context.Background(), userID, id)
	if err != nil {
		return nil, false, err
	}
	if expected != "" && alert.Status != expected {
		return nil, false, repository.ErrEvolutionConflict
	}
	if !alert.Status.Open() {
		return alert, false, nil
	}
	if apply != nil {
		f.applyCalls++
		if err := apply(nil, alert); err != nil {
			return nil, false, err
		}
	}
	if action == model.EvolutionActionDefer {
		alert.Status = model.EvolutionAlertDeferred
		alert.ResolvedAt = nil
		alert.ResolvedAction = ""
		alert.ResolutionNote = note
		return alert, true, nil
	}
	now := time.Now()
	alert.Status = model.EvolutionAlertResolved
	alert.ResolvedAt = &now
	alert.ResolvedAction = action
	alert.ResolutionNote = note
	return alert, true, nil
}

type evolutionKnowledgeFake struct {
	items  map[uint]*model.KnowledgeItem
	gotIDs []uint
}

func (f *evolutionKnowledgeFake) Get(_ context.Context, userID, id uint) (*model.KnowledgeItem, error) {
	item, ok := f.items[id]
	if !ok || item.UserID != userID {
		return nil, gorm.ErrRecordNotFound
	}
	return item, nil
}

func (f *evolutionKnowledgeFake) ListByIDs(_ context.Context, userID uint, ids []uint) ([]model.KnowledgeItem, error) {
	f.gotIDs = append(f.gotIDs, ids...)
	out := make([]model.KnowledgeItem, 0, len(ids))
	for _, id := range ids {
		if item, ok := f.items[id]; ok && item.UserID == userID {
			out = append(out, *item)
		}
	}
	return out, nil
}

type evolutionArchiverFake struct {
	archived []uint
	err      error
}

func (f *evolutionArchiverFake) Archive(_ context.Context, _ uint, id uint) (*time.Time, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.archived = append(f.archived, id)
	now := time.Now()
	return &now, nil
}

type evolutionOutdatedCall struct {
	id     uint
	at     *time.Time
	reason string
}

type evolutionOutdatedFake struct {
	calls []evolutionOutdatedCall
	err   error
}

func (f *evolutionOutdatedFake) SetOutdated(_ context.Context, _ uint, id uint, at *time.Time, reason string) error {
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, evolutionOutdatedCall{id: id, at: at, reason: reason})
	return nil
}

type evolutionLogFake struct{ entries []*model.OperationLogModel }

func (f *evolutionLogFake) CreateOperationLog(entry *model.OperationLogModel) error {
	f.entries = append(f.entries, entry)
	return nil
}

func (f *evolutionLogFake) PruneOperationLogs(uint, int) (int64, error) { return 0, nil }

type evolutionFixture struct {
	service   *EvolutionService
	store     *evolutionStoreFake
	knowledge *evolutionKnowledgeFake
	archiver  *evolutionArchiverFake
	outdated  *evolutionOutdatedFake
	logs      *evolutionLogFake
}

func newEvolutionFixture() *evolutionFixture {
	now := time.Now()
	store := &evolutionStoreFake{}
	knowledge := &evolutionKnowledgeFake{items: map[uint]*model.KnowledgeItem{
		10: {ID: 10, UserID: 2, Title: "新导入", SourceType: "file", Status: model.KnowledgeReady, Version: 1},
		11: {ID: 11, UserID: 2, Title: "原件", SourceType: "manual", Status: model.KnowledgeReady, Version: 3},
		12: {ID: 12, UserID: 2, Title: "已归档", Status: model.KnowledgeReady, ArchivedAt: &now},
		13: {ID: 13, UserID: 2, Title: "旧版本", Status: model.KnowledgeReady, OutdatedAt: &now, OutdatedReason: "已被新版本取代"},
	}}
	f := &evolutionFixture{
		store: store, knowledge: knowledge, archiver: &evolutionArchiverFake{},
		outdated: &evolutionOutdatedFake{}, logs: &evolutionLogFake{},
	}
	f.service = &EvolutionService{
		Alerts: store, Knowledge: knowledge, Archiver: f.archiver, Outdated: f.outdated, Logs: f.logs,
	}
	return f
}

// addAlert 直接构造一条提醒行；ID 与 ID 数组都是常量，序列化不会失败，无需返回错误。
func (f *evolutionFixture) addAlert(id uint, alertType model.EvolutionAlertType, status model.EvolutionAlertStatus, subjects []uint) *model.EvolutionAlert {
	encoded, _ := json.Marshal(subjects)
	payload, _ := json.Marshal(EvolutionPayload{
		Type: alertType, Reason: "正文完全一致", SuggestedAction: model.EvolutionActionArchiveOne,
		Evidence: []EvolutionEvidence{{KnowledgeID: subjects[0], Quote: "同一段正文"}},
	})
	alert := model.EvolutionAlert{
		ID: id, UserID: 2, AlertType: alertType, Status: status,
		Severity: model.EvolutionSeverityReview, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		SubjectIDs: string(encoded), Payload: string(payload),
		DedupeKey: EvolutionDedupeKey(alertType, subjects, []EvolutionEvidence{{KnowledgeID: subjects[0], Quote: "同一段正文"}}),
	}
	f.store.alerts = append(f.store.alerts, alert)
	return &f.store.alerts[len(f.store.alerts)-1]
}

// 列表：把涉及知识补成标题与状态，一次批量查询，缺失的 ID 保留占位。
func TestEvolutionListDecoratesSubjects(t *testing.T) {
	f := newEvolutionFixture()
	f.addAlert(1, model.EvolutionAlertTypeDuplicate, model.EvolutionAlertPending, []uint{10, 11})
	f.addAlert(2, model.EvolutionAlertTypeDuplicate, model.EvolutionAlertPending, []uint{10, 999})

	views, total, err := f.service.List(context.Background(), 2, 1, 20, "", "", true)

	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.True(t, f.store.filter.Open)
	require.Len(t, views, 2)
	require.Equal(t, "新导入", views[0].Subjects[0].Title)
	require.Equal(t, "file", views[0].Subjects[0].SourceType)
	require.Equal(t, "原件", views[0].Subjects[1].Title)
	require.False(t, views[0].Subjects[0].Archived)
	// 涉及知识已被删除时保留占位，让用户看到「这条提醒指向了一条不存在的知识」。
	require.Equal(t, uint(999), views[1].Subjects[1].ID)
	require.Empty(t, views[1].Subjects[1].Title)
	require.Equal(t, "正文完全一致", views[0].Payload.Reason)
}

func TestEvolutionListRejectsInvalidFilter(t *testing.T) {
	f := newEvolutionFixture()
	for _, tc := range []struct {
		name  string
		page  int
		size  int
		kind  model.EvolutionAlertType
		state model.EvolutionAlertStatus
	}{
		{"page zero", 0, 20, "", ""},
		{"page too large", 1000001, 20, "", ""},
		{"size zero", 1, 0, "", ""},
		{"size too large", 1, 101, "", ""},
		{"unknown type", 1, 20, model.EvolutionAlertType("dupe"), ""},
		{"unknown status", 1, 20, "", model.EvolutionAlertStatus("done")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := f.service.List(context.Background(), 2, tc.page, tc.size, tc.kind, tc.state, false)
			require.ErrorIs(t, err, ErrEvolutionInvalid)
		})
	}
}

// 摘要：待处理与暂不处理分开计数，已处理/已忽略不进首页卡片。
func TestEvolutionSummaryCountsOpenOnly(t *testing.T) {
	f := newEvolutionFixture()
	f.addAlert(1, model.EvolutionAlertTypeDuplicate, model.EvolutionAlertPending, []uint{10, 11})
	f.addAlert(2, model.EvolutionAlertTypeDuplicate, model.EvolutionAlertDeferred, []uint{10, 12})
	f.addAlert(3, model.EvolutionAlertTypeOutdated, model.EvolutionAlertPending, []uint{13, 11})
	f.addAlert(4, model.EvolutionAlertTypeConflict, model.EvolutionAlertResolved, []uint{10, 13})
	f.addAlert(5, model.EvolutionAlertTypeConflict, model.EvolutionAlertDismissed, []uint{10, 12})

	view, err := f.service.Summary(context.Background(), 2)

	require.NoError(t, err)
	require.Equal(t, int64(2), view.Pending)
	require.Equal(t, int64(1), view.Deferred)
	require.Equal(t, int64(2), view.ByType.Duplicate)
	require.Equal(t, int64(1), view.ByType.Outdated)
	require.Equal(t, int64(0), view.ByType.Conflict)
}

// 归档其一：保留先入库的原件，归档 ID 较大的那条（与触发方向无关）。
func TestEvolutionResolveArchiveOneKeepsOlder(t *testing.T) {
	f := newEvolutionFixture()
	f.addAlert(1, model.EvolutionAlertTypeDuplicate, model.EvolutionAlertPending, []uint{10, 11})

	view, changed, err := f.service.Resolve(context.Background(), 2, 1, EvolutionResolveInput{Action: model.EvolutionActionArchiveOne, Note: "重复导入"})

	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []uint{11}, f.archiver.archived)
	require.Equal(t, model.EvolutionAlertResolved, view.Status)
	require.Equal(t, model.EvolutionActionArchiveOne, view.ResolvedAction)
	require.Equal(t, "重复导入", view.ResolutionNote)
	require.NotNil(t, view.ResolvedAt)
	require.Len(t, f.logs.entries, 1)
	require.Equal(t, model.OperationLogActionEvolutionResolve, f.logs.entries[0].Action)
	require.Equal(t, model.OperationLogTargetTypeEvolution, f.logs.entries[0].TargetType)
	require.Contains(t, f.logs.entries[0].Detail, "archive_one")
}

// 触发方向反过来（[0] 是旧条目）时同样归档 ID 较大的一条。
func TestEvolutionResolveArchiveOneIgnoresSubjectOrder(t *testing.T) {
	f := newEvolutionFixture()
	f.addAlert(1, model.EvolutionAlertTypeDuplicate, model.EvolutionAlertPending, []uint{11, 10})

	_, _, err := f.service.Resolve(context.Background(), 2, 1, EvolutionResolveInput{Action: model.EvolutionActionArchiveOne})

	require.NoError(t, err)
	require.Equal(t, []uint{11}, f.archiver.archived)
}

// 暂不处理：状态变 deferred、不写裁决时间，也不产生操作日志。
func TestEvolutionResolveDeferKeepsAlertOpen(t *testing.T) {
	f := newEvolutionFixture()
	f.addAlert(1, model.EvolutionAlertTypeDuplicate, model.EvolutionAlertPending, []uint{10, 11})

	view, changed, err := f.service.Resolve(context.Background(), 2, 1, EvolutionResolveInput{Action: model.EvolutionActionDefer})

	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, model.EvolutionAlertDeferred, view.Status)
	require.Nil(t, view.ResolvedAt)
	require.Empty(t, view.ResolvedAction)
	require.Empty(t, f.archiver.archived)
}

// 已处理过的提醒再次提交按幂等处理：200 + 当前提醒，不重复执行动作、不写日志。
func TestEvolutionResolveIsIdempotent(t *testing.T) {
	f := newEvolutionFixture()
	f.addAlert(1, model.EvolutionAlertTypeDuplicate, model.EvolutionAlertResolved, []uint{10, 11})

	view, changed, err := f.service.Resolve(context.Background(), 2, 1, EvolutionResolveInput{Action: model.EvolutionActionArchiveOne})

	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, model.EvolutionAlertResolved, view.Status)
	require.Empty(t, f.archiver.archived)
	require.Equal(t, 0, f.store.applyCalls)
	require.Empty(t, f.logs.entries)
}

// 动作与提醒类型不匹配一律拒绝：不能对重复提醒采纳修订。
func TestEvolutionResolveRejectsMismatchedAction(t *testing.T) {
	f := newEvolutionFixture()
	f.addAlert(1, model.EvolutionAlertTypeDuplicate, model.EvolutionAlertPending, []uint{10, 11})
	f.addAlert(2, model.EvolutionAlertTypeOutdated, model.EvolutionAlertPending, []uint{13, 11})
	f.addAlert(3, model.EvolutionAlertTypeConflict, model.EvolutionAlertPending, []uint{10, 13})

	for _, tc := range []struct {
		name   string
		id     uint
		action model.EvolutionAction
	}{
		{"duplicate with revision", 1, model.EvolutionActionAcceptRevision},
		{"outdated with archive", 2, model.EvolutionActionArchiveOne},
		{"conflict with archive", 3, model.EvolutionActionArchiveOne},
		{"unknown action", 1, model.EvolutionAction("merge")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := f.service.Resolve(context.Background(), 2, tc.id, EvolutionResolveInput{Action: tc.action})
			require.ErrorIs(t, err, ErrEvolutionInvalid)
		})
	}
}

// 后续迭代才接上的动作明确返回「尚未启用」，而不是静默成功。
func TestEvolutionResolveUnsupportedActions(t *testing.T) {
	f := newEvolutionFixture()
	f.addAlert(1, model.EvolutionAlertTypeConflict, model.EvolutionAlertPending, []uint{10, 13})
	f.addAlert(2, model.EvolutionAlertTypeConflict, model.EvolutionAlertPending, []uint{10, 11})

	_, _, err := f.service.Resolve(context.Background(), 2, 1, EvolutionResolveInput{Action: model.EvolutionActionAcceptRevision})
	require.ErrorIs(t, err, ErrEvolutionUnsupported)
	_, _, err = f.service.Resolve(context.Background(), 2, 2, EvolutionResolveInput{Action: model.EvolutionActionCreateRelation})
	require.ErrorIs(t, err, ErrEvolutionUnsupported)
}

// 标记过时落在被比对的那条知识（subject_ids[1]）上，理由取提醒载荷。
func TestEvolutionResolveMarkOutdatedMarksComparedKnowledge(t *testing.T) {
	f := newEvolutionFixture()
	f.addAlert(1, model.EvolutionAlertTypeOutdated, model.EvolutionAlertPending, []uint{10, 11})

	view, changed, err := f.service.Resolve(context.Background(), 2, 1, EvolutionResolveInput{Action: model.EvolutionActionMarkOutdated})

	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, model.EvolutionAlertResolved, view.Status)
	require.Len(t, f.outdated.calls, 1)
	require.Equal(t, uint(11), f.outdated.calls[0].id, "过时的是被比对的那条，不是本次触发的知识")
	require.NotNil(t, f.outdated.calls[0].at)
	require.NotEmpty(t, f.outdated.calls[0].reason)
	require.Len(t, f.logs.entries, 1, "裁决写一条操作日志")
}

// 标记已核实清除过时标记；重复提醒的核实不碰过时标记，避免顺手抹掉别的判定。
func TestEvolutionResolveMarkVerifiedClearsOnlyOutdatedAlerts(t *testing.T) {
	f := newEvolutionFixture()
	f.addAlert(1, model.EvolutionAlertTypeOutdated, model.EvolutionAlertPending, []uint{10, 11})
	f.addAlert(2, model.EvolutionAlertTypeDuplicate, model.EvolutionAlertPending, []uint{10, 13})

	_, _, err := f.service.Resolve(context.Background(), 2, 1, EvolutionResolveInput{Action: model.EvolutionActionMarkVerified})
	require.NoError(t, err)
	require.Len(t, f.outdated.calls, 1)
	require.Equal(t, uint(11), f.outdated.calls[0].id)
	require.Nil(t, f.outdated.calls[0].at, "核实即清除标记")

	_, _, err = f.service.Resolve(context.Background(), 2, 2, EvolutionResolveInput{Action: model.EvolutionActionMarkVerified})
	require.NoError(t, err)
	require.Len(t, f.outdated.calls, 1, "重复提醒的核实不该改过时标记")
}

// 未接标记能力时「标记过时」明确不可用，而「标记已核实」仍可完成（它本身只是记录看过）。
func TestEvolutionResolveOutdatedRequiresMarker(t *testing.T) {
	f := newEvolutionFixture()
	f.service.Outdated = nil
	f.addAlert(1, model.EvolutionAlertTypeOutdated, model.EvolutionAlertPending, []uint{10, 11})
	f.addAlert(2, model.EvolutionAlertTypeOutdated, model.EvolutionAlertPending, []uint{10, 13})

	_, _, err := f.service.Resolve(context.Background(), 2, 1, EvolutionResolveInput{Action: model.EvolutionActionMarkOutdated})
	require.ErrorIs(t, err, ErrEvolutionUnsupported)

	_, changed, err := f.service.Resolve(context.Background(), 2, 2, EvolutionResolveInput{Action: model.EvolutionActionMarkVerified})
	require.NoError(t, err)
	require.True(t, changed)
}

// 提醒不存在（含别人的提醒）返回 NotFound。
func TestEvolutionResolveMissingAlert(t *testing.T) {
	f := newEvolutionFixture()
	f.addAlert(1, model.EvolutionAlertTypeDuplicate, model.EvolutionAlertPending, []uint{10, 11})

	_, _, err := f.service.Resolve(context.Background(), 3, 1, EvolutionResolveInput{Action: model.EvolutionActionKeepBoth})
	require.ErrorIs(t, err, ErrEvolutionNotFound)
}

// 手动重跑：知识未就绪/已归档给出中文提示，服务可用时返回 202 的任务视图。
func TestEvolutionRegenerateGuards(t *testing.T) {
	f := newEvolutionFixture()
	_, err := f.service.Regenerate(context.Background(), 2, 10)
	require.ErrorIs(t, err, ErrEvolutionUnsupported, "未接任务服务时明确不可用")
}

// 指纹：涉及知识顺序无关，引文变化即视为新提醒。
func TestEvolutionDedupeKeyStable(t *testing.T) {
	evidence := []EvolutionEvidence{{KnowledgeID: 10, Quote: "甲"}, {KnowledgeID: 11, Quote: "乙"}}
	reordered := []EvolutionEvidence{{KnowledgeID: 11, Quote: "乙"}, {KnowledgeID: 10, Quote: "甲"}}
	base := EvolutionDedupeKey(model.EvolutionAlertTypeDuplicate, []uint{10, 11}, evidence)
	require.Equal(t, base, EvolutionDedupeKey(model.EvolutionAlertTypeDuplicate, []uint{11, 10}, evidence))
	require.Equal(t, base, EvolutionDedupeKey(model.EvolutionAlertTypeDuplicate, []uint{10, 11}, reordered))
	require.NotEqual(t, base, EvolutionDedupeKey(model.EvolutionAlertTypeConflict, []uint{10, 11}, evidence))
	require.NotEqual(t, base, EvolutionDedupeKey(model.EvolutionAlertTypeDuplicate, []uint{10, 12}, evidence))
	changedQuote := []EvolutionEvidence{{KnowledgeID: 10, Quote: "甲改"}, {KnowledgeID: 11, Quote: "乙"}}
	require.NotEqual(t, base, EvolutionDedupeKey(model.EvolutionAlertTypeDuplicate, []uint{10, 11}, changedQuote))
	require.Len(t, base, 64)
}

// 证据校验：正文里定位不到的引文被剔除；全部定位不到则整条提醒不落库。
func TestEvolutionEvidenceValidation(t *testing.T) {
	body := "第一行\r\n第二行  \n"
	kept := validEvidence(body, []EvolutionEvidence{
		{KnowledgeID: 10, Quote: "第二行"},
		{KnowledgeID: 10, Quote: "不存在的引文"},
	})
	require.Len(t, kept, 1)
	require.Equal(t, "第二行", kept[0].Quote)
	require.Empty(t, validEvidence(body, []EvolutionEvidence{{KnowledgeID: 10, Quote: "编造的"}}))
}

// 摘录按字符截断，不切碎多字节字符。
func TestEvolutionExcerptTruncatesByRune(t *testing.T) {
	require.Equal(t, "一二三", evolutionExcerpt("一二三四五", 3))
	require.Equal(t, "abc", evolutionExcerpt("abc", 10))
}
