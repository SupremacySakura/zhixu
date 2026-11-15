package main

import (
	"strings"
	"testing"
)

// sampleOutcomes 是一份可以手算核对的小样本：4 条正样本相似度 0.55–0.80，
// 1 条过时 0.65，1 条对照 0.62。期望值全部由人工推得，不经过被测函数，
// 避免用同一套逻辑算出期望再断言自己。
func sampleOutcomes() []pairOutcome {
	return []pairOutcome{
		{SID: "D1", PairType: "duplicate", Expected: "duplicate", Predicted: "duplicate", Judged: true, Similarity: 0.70},
		{SID: "D2", PairType: "duplicate", Expected: "duplicate", Predicted: "none", Similarity: 0.60},
		{SID: "C1", PairType: "conflict", Expected: "conflict", Predicted: "conflict", Judged: true, Similarity: 0.80},
		{SID: "C2", PairType: "conflict", Expected: "conflict", Predicted: "none", Similarity: 0.55},
		{SID: "O1", PairType: "outdated", Expected: "outdated", Predicted: "outdated", Judged: true, Similarity: 0.65},
		{SID: "N1", PairType: "negative", Expected: "none", Predicted: "none", Similarity: 0.62},
	}
}

func TestEstimatedCorrectCountsOnlySamplesTheGateCanResolve(t *testing.T) {
	outcomes := sampleOutcomes()

	cases := []struct {
		gate float64
		want int
	}{
		{0.50, 5}, // 5 条正样本全部进入判定，但对照 0.62 也被放进来，故 5/6
		{0.65, 4}, // 0.70/0.80/0.65 三条正样本进入，对照被挡住，故 4/6
		{0.80, 2}, // 只有 0.80 的冲突样本进入，对照被挡住，故 2/6
	}
	for _, c := range cases {
		if got := estimatedCorrect(outcomes, c.gate); got != c.want {
			t.Errorf("门槛 %.2f: estimatedCorrect = %d, 期望 %d", c.gate, got, c.want)
		}
	}
}

func TestSweepReportPrintsEveryGateWithEstimate(t *testing.T) {
	report := sweepReport(sampleOutcomes())

	// 门槛 0.65：重复 1/2（D1 过、D2 不过）、冲突 1/2、过时 1/1、对照 0/1，估计 4/6。
	if !strings.Contains(report, "| 0.65 | 1/2 | 1/2 | 1/1 | 0/1 | 0.667 |") {
		t.Errorf("0.65 行与手算不符:\n%s", report)
	}
	// 门槛 0.80：只有 C1 过门槛，其余全部被挡，估计 2/6（C1 与 N1）。
	if !strings.Contains(report, "| 0.80 | 0/2 | 1/2 | 0/1 | 0/1 | 0.333 |") {
		t.Errorf("0.80 行与手算不符:\n%s", report)
	}
	if strings.Contains(report, "冲突阈值") || strings.Contains(report, "重复阈值") {
		t.Errorf("表头仍是旧的三档阈值口径:\n%s", report)
	}
}

func TestReportsStateRecomputeProvenance(t *testing.T) {
	for name, report := range map[string]string{
		"sweep": sweepReport(sampleOutcomes()),
		"grid":  gridReport(sampleOutcomes()),
	} {
		if !strings.Contains(report, "模型判定未重跑") {
			t.Errorf("%s 报告缺少「模型判定未重跑」的来源说明:\n%s", name, report)
		}
	}
}

func TestGridReportComparesNewAndLegacyDefaultGates(t *testing.T) {
	report := gridReport(sampleOutcomes())

	// 手算：最优 5/6 出现在 0.50–0.61 与 0.63–0.64；0.65 为 4/6；0.80 为 2/6。
	if !strings.Contains(report, "最优估计准确率 0.833（5/6）") {
		t.Errorf("最优估计准确率与手算不符:\n%s", report)
	}
	if !strings.Contains(report, "当前默认门槛 0.65 的估计准确率 0.667（4/6）") {
		t.Errorf("新默认门槛的估计准确率与手算不符:\n%s", report)
	}
	if !strings.Contains(report, "旧默认门槛 0.80 的估计准确率 0.333（2/6）") {
		t.Errorf("旧默认门槛的估计准确率与手算不符:\n%s", report)
	}
}

func TestDistributionOverlapReportsContainment(t *testing.T) {
	report := distributionOverlap(sampleOutcomes())

	if !strings.Contains(report, "重复区间 [0.600, 0.700] 被冲突区间 [0.550, 0.800] 完整包含") {
		t.Errorf("区间包含关系描述与手算不符:\n%s", report)
	}
	if !strings.Contains(report, "类型由模型判定") {
		t.Errorf("重叠结论仍停留在「按相似度分派类型」口径:\n%s", report)
	}
}

func TestThresholdNoteCarriesEffectiveGate(t *testing.T) {
	cases := []struct {
		name       string
		expected   string
		similarity float64
		gate       float64
		want       string
	}{
		{"低于门槛", "duplicate", 0.60, 0.65, "相似度低于候选门槛 0.65"},
		{"正样本进入判定", "duplicate", 0.70, 0.65, "达到候选门槛 0.65，已进入模型判定"},
		{"对照样本误入", "none", 0.70, 0.65, "达到候选门槛 0.65，进入模型判定，存在误报风险"},
	}
	for _, c := range cases {
		if got := thresholdNote(c.expected, c.similarity, c.gate); !strings.Contains(got, c.want) {
			t.Errorf("%s: thresholdNote = %q, 期望包含 %q", c.name, got, c.want)
		}
	}
}

func TestEvolutionTableAnnotatesGate(t *testing.T) {
	gate := 0.65
	table := evolutionTable(sampleOutcomes(), &gate)

	// 手算：D1/C1/O1/N1 判对，共 4/6；产生提醒 3/6。
	if !strings.Contains(table, "总体准确率 0.667（4/6，候选门槛 0.65）") {
		t.Errorf("总体准确率或门槛注记与手算不符:\n%s", table)
	}
	if !strings.Contains(table, "产生提醒的样本 3/6") {
		t.Errorf("提醒数注记与手算不符:\n%s", table)
	}
	if plain := evolutionTable(sampleOutcomes(), nil); strings.Contains(plain, "候选门槛") {
		t.Errorf("pairwise 汇总不该出现候选门槛:\n%s", plain)
	}
}
