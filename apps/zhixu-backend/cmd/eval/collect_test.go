package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/service"
)

// testAlert 构造一条带结论集合的提醒行。结论的方向由各自的 TriggerID 表达，
// 与生产落库的形状一致：SubjectIDs[0] 是最近一次触发的知识，但方向以结论自身为准。
func testAlert(triggerID, candidateID uint, conclusions ...service.EvolutionConclusion) model.EvolutionAlert {
	payload := service.EvolutionPayload{Conclusions: conclusions}
	if len(conclusions) > 0 {
		primary := conclusions[0]
		payload.Type = primary.Type
		payload.Reason = primary.Reason
		payload.Evidence = primary.Evidence
		payload.SuggestedAction = primary.SuggestedAction
		payload.RevisedTitle = primary.RevisedTitle
		payload.RevisedContent = primary.RevisedContent
	}
	body, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	alert := model.EvolutionAlert{
		UserID: 2, AlertType: payload.Type, Status: model.EvolutionAlertPending,
		Severity: model.EvolutionSeverityReview, Payload: string(body),
	}
	if err := alert.SetSubjectIDs([]uint{triggerID, candidateID}); err != nil {
		panic(err)
	}
	return alert
}

// 样本对：E01 标注为 trigger=10 检查 candidate=11。
func samplePairs() []pairRecord {
	return []pairRecord{{
		SID: "E01", PairType: "duplicate", Expected: "duplicate",
		TriggerTitle: "新", CandidateTitle: "旧", TriggerID: 10, CandidateID: 11,
	}}
}

// 结论来自标注方向时，评测应把它算作本方向的结论。
func TestCollectOutcomesUsesConclusionDirection(t *testing.T) {
	alerts := []model.EvolutionAlert{
		testAlert(10, 11, service.EvolutionConclusion{
			Type: model.EvolutionAlertTypeDuplicate, TriggerID: 10, Reason: "信息重合",
		}),
	}

	outcomes := collectOutcomes(alerts, samplePairs())

	if len(outcomes) != 1 {
		t.Fatalf("期望 1 条样本结果，得到 %d", len(outcomes))
	}
	got := outcomes[0]
	if got.Predicted != "duplicate" || !got.Judged {
		t.Errorf("期望判为 duplicate 且已判定，得到 predicted=%q judged=%v", got.Predicted, got.Judged)
	}
	if !got.DirectionMatch {
		t.Errorf("结论来自标注方向，direction_match 应为 true")
	}
}

// 只有反向结论时，样本仍算「产生了提醒」，但要标出方向不匹配，
// 否则读者会把反向的正确结论误当成模型在标注方向上判对了。
func TestCollectOutcomesMarksReverseOnlyConclusion(t *testing.T) {
	alerts := []model.EvolutionAlert{
		testAlert(11, 10, service.EvolutionConclusion{
			Type: model.EvolutionAlertTypeConflict, TriggerID: 11, Reason: "取值互斥",
		}),
	}

	got := collectOutcomes(alerts, samplePairs())[0]

	if got.Predicted != "conflict" || !got.Judged {
		t.Errorf("反向结论仍应计入提醒，得到 predicted=%q judged=%v", got.Predicted, got.Judged)
	}
	if got.DirectionMatch {
		t.Errorf("只有反向结论时 direction_match 应为 false")
	}
}

// 一条提醒同时承载两个方向的结论时，两个样本方向都能各自取到自己的结论，
// 不再像按行取数那样被 SubjectIDs 的顺序绑死。
func TestCollectOutcomesReadsBothDirectionsFromOneAlert(t *testing.T) {
	alerts := []model.EvolutionAlert{
		testAlert(10, 11,
			service.EvolutionConclusion{Type: model.EvolutionAlertTypeDuplicate, TriggerID: 10, Reason: "信息重合"},
			service.EvolutionConclusion{Type: model.EvolutionAlertTypeOutdated, TriggerID: 11, Reason: "已被取代"},
		),
	}
	pairs := []pairRecord{
		{SID: "E01", PairType: "duplicate", Expected: "duplicate", TriggerID: 10, CandidateID: 11},
		{SID: "E01R", PairType: "outdated", Expected: "outdated", TriggerID: 11, CandidateID: 10},
	}

	outcomes := collectOutcomes(alerts, pairs)

	if outcomes[0].Predicted != "duplicate" || !outcomes[0].DirectionMatch {
		t.Errorf("正向样本应取到 duplicate 且方向匹配，得到 %q / %v", outcomes[0].Predicted, outcomes[0].DirectionMatch)
	}
	if outcomes[1].Predicted != "outdated" || !outcomes[1].DirectionMatch {
		t.Errorf("反向样本应取到 outdated 且方向匹配，得到 %q / %v", outcomes[1].Predicted, outcomes[1].DirectionMatch)
	}
}

// 历史提醒的载荷只有顶层字段、没有结论集合（迁移前的旧数据）。
// 取数不能因此把它们当成「没产生提醒」，否则重跑评测会凭空多出一批漏报。
func TestCollectOutcomesFallsBackToTopLevelPayload(t *testing.T) {
	legacy := model.EvolutionAlert{
		UserID: 2, AlertType: model.EvolutionAlertTypeOutdated,
		Status: model.EvolutionAlertPending, Severity: model.EvolutionSeverityReview,
		Payload: `{"type":"outdated","reason":"已被取代","evidence":[]}`,
	}
	if err := legacy.SetSubjectIDs([]uint{10, 11}); err != nil {
		t.Fatalf("构造历史提醒失败: %v", err)
	}

	got := collectOutcomes([]model.EvolutionAlert{legacy}, samplePairs())[0]

	if !got.Judged {
		t.Fatalf("历史提醒应被计为产生了提醒，实际 judged=%v", got.Judged)
	}
	if !got.DirectionMatch {
		t.Errorf("历史提醒的两侧顺序就是方向，direction_match 应为 true")
	}
}

// 反向结论计入总体准确率会高估端到端能力：那些样本在标注方向上根本没产出判定。
// 汇总表必须把反向结论单列，并给出剔除后的严格口径。
func TestEvolutionTableSeparatesReverseOnlyConclusions(t *testing.T) {
	gate := 0.65
	outcomes := []pairOutcome{
		// 本方向判对。
		{SID: "F1", PairType: "duplicate", Expected: "duplicate", Predicted: "duplicate", Judged: true, DirectionMatch: true},
		// 只有反向结论，且恰好与标注一致。
		{SID: "R1", PairType: "conflict", Expected: "conflict", Predicted: "conflict", Judged: true, DirectionMatch: false},
		// 只有反向结论，且与标注不一致。
		{SID: "R2", PairType: "outdated", Expected: "outdated", Predicted: "conflict", Judged: true, DirectionMatch: false},
		// 没有产出任何结论。
		{SID: "N1", PairType: "negative", Expected: "none", Predicted: "none", Judged: false},
	}

	table := evolutionTable(outcomes, &gate)

	// 含反向：F1、R1、N1 判对，共 3/4。
	if !strings.Contains(table, "总体准确率 0.750（3/4，候选门槛 0.65）") {
		t.Errorf("总体准确率与手算不符:\n%s", table)
	}
	// 剔除两条反向结论后：F1 与 N1 判对，2/2。
	if !strings.Contains(table, "2 条结论来自反向检查") {
		t.Errorf("未单列反向结论条数:\n%s", table)
	}
	if !strings.Contains(table, "剔除后准确率 1.000（2/2）") {
		t.Errorf("严格口径与手算不符:\n%s", table)
	}
	// 口径变了，旧报告的数字不能直接对比。这句必须出现在表里，而不是只写在论文正文。
	if !strings.Contains(table, "取数口径已改为按结论集合") {
		t.Errorf("汇总表缺少取数口径变化说明:\n%s", table)
	}
}

// evolveSource 读取 AI 服务侧的判定实现。两份上限分别写在这两个语言里，
// 没有任何编译期约束把它们绑在一起，只能靠这条测试在 CI 里拦住漂移。
func evolveSource(t *testing.T) string {
	t.Helper()
	// 测试的工作目录是包目录，仓库布局变动时从当前目录逐级向上找，避免写死层级。
	const relative = "apps/zhixu-ai/src/lib/evolve.ts"
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("获取工作目录失败: %v", err)
	}
	for {
		path := filepath.Join(dir, relative)
		if data, readErr := os.ReadFile(path); readErr == nil {
			return string(data)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("未能从 %s 向上找到 %s", dir, relative)
		}
		dir = parent
	}
}

// 候选上限两侧必须完全相等：Go 按它截断送模型的候选，AI 服务按它校验请求体。
// 任一侧单独改动都会让另一边拒收或静默截断，且不会有任何运行时告警。
func TestEvolveCandidateCapMatchesAIServiceContract(t *testing.T) {
	source := evolveSource(t)
	// 候选数组是唯一同时带 .min(1) 与 .max(N) 的字段，且紧跟在 origin 枚举之后。
	// 不能只匹配 .min(1).max(N)：标题长度字段也是这个形状，会误判成候选上限。
	match := regexp.MustCompile(
		`origin:\s*z\.enum\(\[[^\]]*\]\),\s*\}\)\s*\.strict\(\),\s*\)\s*\.min\(1\)\s*\.max\((\d+)\)`,
	).FindStringSubmatch(source)
	if match == nil {
		t.Fatalf("未能从 AI 服务判定实现里找到候选上限声明，契约可能已改动")
	}
	got, err := strconv.Atoi(match[1])
	if err != nil {
		t.Fatalf("解析候选上限失败: %v", err)
	}
	if got != service.EvolveCandidateMax {
		t.Errorf("候选上限不一致：AI 服务 %d，Go %d。改动上限时必须同步两侧",
			got, service.EvolveCandidateMax)
	}
}

// 提醒上限不能早于收敛生效。模型清洗阶段若按比 Go 收敛上限更小的数字截断，
// 第 N 条之后的结论会在按优先级排序之前就被丢掉——正是收敛要解决的问题。
// 因此：要么不截断（把上限交给收敛），要么截断值不小于 Go 的上限。
func TestEvolveAlertCapDoesNotTruncateBeforeConvergence(t *testing.T) {
	source := evolveSource(t)
	// 只认「按上限截断」这一形态：判等后立即 break。清洗阶段还有一处
	// `out.length === 0` 是「一条都没解析出来」的合法性判断，与截断无关，
	// 若一并匹配会把它当成「按 0 条截断」而误报。
	match := regexp.MustCompile(`out\.length\s*===\s*(\d+)\s*\)\s*(?:\{\s*)?break`).FindStringSubmatch(source)
	if match == nil {
		// 清洗阶段不截断：上限完全由 Go 侧收敛决定，这是符合预期的形态。
		return
	}
	got, err := strconv.Atoi(match[1])
	if err != nil {
		t.Fatalf("解析提醒上限失败: %v", err)
	}
	if got < service.EvolveAlertMaxPerTask {
		t.Errorf("AI 服务在清洗阶段按 %d 条截断，小于 Go 收敛上限 %d：结论会在排序前被丢弃",
			got, service.EvolveAlertMaxPerTask)
	}
}
