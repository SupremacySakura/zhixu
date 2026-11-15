// Command eval 是毕业论文实验的评测入口，直接复用生产链路而不是另写一套检索逻辑：
//
//	eval seed      --user <用户名> --data <数据集目录>
//	eval clean     --user <用户名> --data <数据集目录>
//	eval retrieval --user <用户名> --data <数据集目录> [--out <输出目录>]
//	eval evolution --user <用户名> --data <数据集目录> --mode pairwise|collect
//	eval sweep     [--out <输出目录>]
//	eval check     --data <数据集目录>
//	eval grid      [--out <输出目录>]
//
// seed 通过 KnowledgeService.CreateManual 入库，分块、向量化与演化检查都走真实队列；
// retrieval 复用 service.RagSearcher 的 keyword/semantic/hybrid 三模式；
// evolution 的 pairwise 模式把标注好的候选对直接交给生产 EvolveClient 判定，
// collect 模式则读取真实流水线产出的 evolution_alerts，衡量端到端效果。
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"zhixu-backend/internal/config"
	"zhixu-backend/internal/database"
	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"
	"zhixu-backend/internal/service"
	"zhixu-backend/internal/utils"
)

type corpusItem struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

type retrievalQuestion struct {
	QID         string   `json:"qid"`
	Category    string   `json:"category"`
	Question    string   `json:"question"`
	GroundTruth []string `json:"ground_truth"`
	Anchor      string   `json:"anchor"`
}

type evolutionSample struct {
	SID               string   `json:"sid"`
	PairType          string   `json:"pair_type"`
	Expected          string   `json:"expected"`
	Trigger           string   `json:"trigger"`
	Candidate         string   `json:"candidate"`
	EvidenceTrigger   []string `json:"evidence_trigger"`
	EvidenceCandidate []string `json:"evidence_candidate"`
	Note              string   `json:"note"`
}

// pairRecord 把样本对映射到真实知识 ID，供 pairwise 与 collect 两种评测共用。
type pairRecord struct {
	SID            string `json:"sid"`
	PairType       string `json:"pair_type"`
	Expected       string `json:"expected"`
	TriggerTitle   string `json:"trigger_title"`
	CandidateTitle string `json:"candidate_title"`
	TriggerID      uint   `json:"trigger_id"`
	CandidateID    uint   `json:"candidate_id"`
	Note           string `json:"note"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	dataDir := fs.String("data", "实验", "数据集目录")
	username := fs.String("user", "", "已存在的用户名")
	outDir := fs.String("out", filepath.Join("实验", "结果"), "结果输出目录")
	topK := fs.Int("topk", 5, "Recall@K 的最大 K")
	mode := fs.String("mode", "pairwise", "evolution 模式：pairwise 或 collect")
	_ = fs.Parse(os.Args[2:])

	// check 只校验数据集，不连数据库，方便在数据定稿时随时跑一遍。
	if cmd == "check" {
		mustCheck(*dataDir)
		return
	}
	// sweep 只用已落盘的相似度做阈值敏感性分析，同样不需要数据库。
	if cmd == "sweep" {
		mustSweep(*outDir)
		return
	}
	// grid 在相似度上穷举阈值，回答「调参最多能到多少」这个问题。
	if cmd == "grid" {
		mustGrid(*outDir)
		return
	}

	cfg := config.MustLoad()
	database.InitDB(cfg)
	if *username == "" {
		fatal(errors.New("缺少 --user"))
	}
	user := mustUser(*username)

	switch cmd {
	case "seed":
		mustSeed(user, *dataDir, *outDir)
	case "clean":
		mustClean(user, *dataDir)
	case "retrieval":
		mustRetrieval(user, *dataDir, *outDir, *topK, cfg)
	case "evolution":
		mustEvolution(user, *dataDir, *outDir, *mode, cfg)
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `用法:
  eval seed      --user <用户名> [--data 实验] [--out 实验/结果]
  eval clean     --user <用户名> [--data 实验]
  eval retrieval --user <用户名> [--data 实验] [--out 实验/结果] [--topk 5]
  eval evolution --user <用户名> [--data 实验] [--out 实验/结果] --mode pairwise|collect
  eval sweep     [--out 实验/结果]
  eval grid      [--out 实验/结果]
  eval check     [--data 实验]`)
	os.Exit(2)
}

// ---------- 数据集读取 ----------

func readJSONL[T any](path string) ([]T, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var out []T
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var item T
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		out = append(out, item)
	}
	return out, scanner.Err()
}

func mustLoad[T any](path string) []T {
	items, err := readJSONL[T](path)
	if err != nil {
		fatal(err)
	}
	return items
}

// ---------- seed ----------

// mustCheck 校验数据集本身：ID 唯一、标注的知识存在、锚点与证据引文能逐字定位。
// 这一步不依赖数据库，数据集一改就能立刻发现问题，避免跑完实验才发现标注错位。
func mustCheck(dataDir string) {
	corpus := mustLoad[corpusItem](filepath.Join(dataDir, "检索语料.jsonl"))
	questions := mustLoad[retrievalQuestion](filepath.Join(dataDir, "检索问题.jsonl"))
	samples := mustLoad[evolutionSample](filepath.Join(dataDir, "演化样本.jsonl"))

	problems := 0
	fail := func(format string, args ...any) {
		problems++
		fmt.Printf("  ✗ "+format+"\n", args...)
	}

	bodies := map[string]string{}
	for _, item := range corpus {
		if _, ok := bodies[item.ID]; ok {
			fail("语料 ID 重复: %s", item.ID)
		}
		bodies[item.ID] = utils.NormalizeKnowledgeContent(item.Content)
		if strings.TrimSpace(item.Title) == "" || strings.TrimSpace(item.Content) == "" {
			fail("语料 %s 标题或正文为空", item.ID)
		}
	}

	categories := map[string]int{}
	for _, question := range questions {
		categories[question.Category]++
		if question.Category == "negative" {
			if len(question.GroundTruth) > 0 {
				fail("%s 标为 negative 却带 ground_truth", question.QID)
			}
			continue
		}
		if len(question.GroundTruth) == 0 {
			fail("%s 缺少 ground_truth", question.QID)
		}
		for _, code := range question.GroundTruth {
			if _, ok := bodies[code]; !ok {
				fail("%s 引用了不存在的语料 %s", question.QID, code)
			}
		}
		if question.Anchor != "" {
			anchor := utils.NormalizeKnowledgeContent(question.Anchor)
			located := false
			for _, code := range question.GroundTruth {
				if strings.Contains(bodies[code], anchor) {
					located = true
					break
				}
			}
			if !located {
				fail("%s 的锚点无法在标注语料中逐字定位: %q", question.QID, question.Anchor)
			}
		}
	}

	pairTypes := map[string]int{}
	for _, sample := range samples {
		pairTypes[sample.PairType]++
		trigger := utils.NormalizeKnowledgeContent(sample.Trigger)
		candidate := utils.NormalizeKnowledgeContent(sample.Candidate)
		if sample.PairType == "negative" {
			if sample.Expected != "none" {
				fail("%s 是 negative，expected 应为 none", sample.SID)
			}
			continue
		}
		if sample.Expected != sample.PairType {
			fail("%s 的 expected 与 pair_type 不一致", sample.SID)
		}
		if len(sample.EvidenceTrigger) == 0 || len(sample.EvidenceCandidate) == 0 {
			fail("%s 缺少任一侧的证据引文", sample.SID)
		}
		for _, quote := range sample.EvidenceTrigger {
			if !strings.Contains(trigger, utils.NormalizeKnowledgeContent(quote)) {
				fail("%s 触发侧引文无法逐字定位: %q", sample.SID, quote)
			}
		}
		for _, quote := range sample.EvidenceCandidate {
			if !strings.Contains(candidate, utils.NormalizeKnowledgeContent(quote)) {
				fail("%s 候选侧引文无法逐字定位: %q", sample.SID, quote)
			}
		}
	}

	fmt.Printf("语料 %d 篇，检索问题 %d 条 %v，演化样本 %d 组 %v\n",
		len(corpus), len(questions), categories, len(samples), pairTypes)
	if problems > 0 {
		fatal(fmt.Errorf("数据集校验未通过，共 %d 处问题", problems))
	}
	fmt.Println("数据集校验通过。")
}

func mustSeed(user *model.UserModel, dataDir, outDir string) {
	corpus := mustLoad[corpusItem](filepath.Join(dataDir, "检索语料.jsonl"))
	samples := mustLoad[evolutionSample](filepath.Join(dataDir, "演化样本.jsonl"))
	knowledge := service.NewKnowledgeService(repository.NewKnowledgeRepository(database.DB), "")

	titles := map[string]string{}
	for _, item := range corpus {
		if _, err := knowledge.CreateManual(context.Background(), user.ID, item.Title, item.Content); err != nil {
			fatal(fmt.Errorf("导入语料 %s: %w", item.ID, err))
		}
		titles[item.ID] = item.Title
		fmt.Printf("语料 %-4s 已入库: %s\n", item.ID, item.Title)
	}

	pairs := make([]pairRecord, 0, len(samples))
	for _, sample := range samples {
		triggerTitle := firstLineTitle(sample.Trigger)
		candidateTitle := firstLineTitle(sample.Candidate)
		for _, body := range []struct{ title, content string }{
			{triggerTitle, sample.Trigger},
			{candidateTitle, sample.Candidate},
		} {
			if _, err := knowledge.CreateManual(context.Background(), user.ID, body.title, body.content); err != nil {
				fatal(fmt.Errorf("导入样本 %s: %w", sample.SID, err))
			}
		}
		pairs = append(pairs, pairRecord{
			SID: sample.SID, PairType: sample.PairType, Expected: sample.Expected,
			TriggerTitle: triggerTitle, CandidateTitle: candidateTitle, Note: sample.Note,
		})
		fmt.Printf("样本 %-4s 已入库: %s / %s\n", sample.SID, triggerTitle, candidateTitle)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fatal(err)
	}
	writeJSON(filepath.Join(outDir, "seed-titles.json"), titles)
	writeJSON(filepath.Join(outDir, "pairs.json"), pairs)
	fmt.Printf("\n共导入 %d 篇语料、%d 组演化样本。\n", len(corpus), len(samples))
	fmt.Println("等待向量化队列跑完（知识列表里 vector_state 为 ready）后再执行 retrieval / evolution。")
}

// mustClean 只删除数据集里出现过的标题，绝不按 source_type 批量删：
// 测试账号里往往还有用户自己的知识，误删不可恢复。
func mustClean(user *model.UserModel, dataDir string) {
	corpus := mustLoad[corpusItem](filepath.Join(dataDir, "检索语料.jsonl"))
	samples := mustLoad[evolutionSample](filepath.Join(dataDir, "演化样本.jsonl"))

	titles := map[string]bool{}
	for _, item := range corpus {
		titles[item.Title] = true
	}
	for _, sample := range samples {
		titles[firstLineTitle(sample.Trigger)] = true
		titles[firstLineTitle(sample.Candidate)] = true
	}

	knowledge := service.NewKnowledgeService(repository.NewKnowledgeRepository(database.DB), "")
	deleted := 0
	for title := range titles {
		var rows []model.KnowledgeItem
		if err := database.DB.Where("user_id = ? AND title = ?", user.ID, title).Find(&rows).Error; err != nil {
			fatal(err)
		}
		for _, row := range rows {
			if err := knowledge.Delete(context.Background(), user.ID, row.ID); err != nil {
				fmt.Fprintf(os.Stderr, "删除 %q(%d) 失败: %v\n", title, row.ID, err)
				continue
			}
			deleted++
		}
	}
	fmt.Printf("已删除 %d 条评测知识（标题白名单匹配）。\n", deleted)
}

func firstLineTitle(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
		if line != "" {
			if runes := []rune(line); len(runes) > 200 {
				return string(runes[:200])
			}
			return line
		}
	}
	return "未命名"
}

// ---------- retrieval ----------

type questionOutcome struct {
	QID       string   `json:"qid"`
	Category  string   `json:"category"`
	Question  string   `json:"question"`
	Mode      string   `json:"mode"`
	Ranked    []string `json:"ranked"`
	FirstRank int      `json:"first_rank"`
	RR        float64  `json:"rr"`
	LatencyMS int      `json:"latency_ms"`
}

type modeSummary struct {
	Mode           string  `json:"mode"`
	Positive       int     `json:"positive"`
	Negative       int     `json:"negative"`
	RecallAt1      float64 `json:"recall_at_1"`
	RecallAt3      float64 `json:"recall_at_3"`
	RecallAt5      float64 `json:"recall_at_5"`
	MRR            float64 `json:"mrr"`
	MissRate       float64 `json:"miss_rate"`
	FalseAlarmRate float64 `json:"false_alarm_rate"`
	AvgLatencyMS   float64 `json:"avg_latency_ms"`
}

func mustRetrieval(user *model.UserModel, dataDir, outDir string, maxK int, cfg *config.Config) {
	corpus := mustLoad[corpusItem](filepath.Join(dataDir, "检索语料.jsonl"))
	questions := mustLoad[retrievalQuestion](filepath.Join(dataDir, "检索问题.jsonl"))

	idToCode := map[uint]string{}
	for _, item := range corpus {
		var row model.KnowledgeItem
		if err := database.DB.Where("user_id = ? AND title = ?", user.ID, item.Title).First(&row).Error; err == nil {
			idToCode[row.ID] = item.ID
		}
	}
	if len(idToCode) == 0 {
		fatal(errors.New("数据库里没有已入库的语料，请先执行 eval seed"))
	}

	chat := newChatService(cfg)
	searcher := &service.RagSearcher{
		Chat:     chat,
		Embedder: &service.MastraEmbedder{AIURL: cfg.ChatAIURL, Secret: cfg.InternalAPISecret},
		Vectors:  &service.QdrantClient{URL: cfg.QdrantURL, APIKey: cfg.QdrantAPIKey},
		Chunks:   &repository.RagRepository{DB: database.DB},
	}

	var outcomes []questionOutcome
	var summaries []modeSummary
	for _, mode := range []string{service.RagModeKeyword, service.RagModeSemantic, service.RagModeHybrid} {
		summary := modeSummary{Mode: mode}
		totalLatency := 0
		for _, question := range questions {
			start := time.Now()
			result, err := searcher.Search(context.Background(), service.RagQuery{
				UserID: user.ID, Query: question.Question, Mode: mode,
			})
			latency := int(time.Since(start).Milliseconds())
			totalLatency += latency
			if err != nil {
				fmt.Fprintf(os.Stderr, "[%s] %s 检索失败: %v\n", mode, question.QID, err)
				continue
			}

			outcome := questionOutcome{
				QID: question.QID, Category: question.Category, Question: question.Question,
				Mode: mode, FirstRank: -1, LatencyMS: latency,
			}
			seen := map[string]bool{}
			for _, source := range result.Sources {
				code := idToCode[source.KnowledgeID]
				if code == "" || seen[code] {
					continue
				}
				seen[code] = true
				outcome.Ranked = append(outcome.Ranked, code)
			}
			for i, code := range outcome.Ranked {
				if contains(question.GroundTruth, code) {
					outcome.FirstRank = i + 1
					break
				}
			}

			if question.Category == "negative" {
				summary.Negative++
				if len(outcome.Ranked) > 0 {
					summary.FalseAlarmRate++
				}
			} else {
				summary.Positive++
				switch {
				case outcome.FirstRank == 1:
					summary.RecallAt1++
					fallthrough
				case outcome.FirstRank > 0 && outcome.FirstRank <= 3:
					summary.RecallAt3++
					fallthrough
				case outcome.FirstRank > 0 && outcome.FirstRank <= maxK:
					summary.RecallAt5++
				}
				if outcome.FirstRank > 0 {
					outcome.RR = 1 / float64(outcome.FirstRank)
					summary.MRR += outcome.RR
				} else {
					summary.MissRate++
				}
			}
			outcomes = append(outcomes, outcome)
		}

		if summary.Positive > 0 {
			summary.RecallAt1 /= float64(summary.Positive)
			summary.RecallAt3 /= float64(summary.Positive)
			summary.RecallAt5 /= float64(summary.Positive)
			summary.MRR /= float64(summary.Positive)
			summary.MissRate /= float64(summary.Positive)
		}
		if summary.Negative > 0 {
			summary.FalseAlarmRate /= float64(summary.Negative)
		}
		summary.AvgLatencyMS = float64(totalLatency) / float64(len(questions))
		summaries = append(summaries, summary)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fatal(err)
	}
	writeJSON(filepath.Join(outDir, "retrieval-per-question.json"), outcomes)
	writeJSON(filepath.Join(outDir, "retrieval-summary.json"), summaries)
	table := retrievalTable(summaries)
	writeText(filepath.Join(outDir, "retrieval-summary.md"), table)
	fmt.Println(table)
}

func retrievalTable(summaries []modeSummary) string {
	var b strings.Builder
	b.WriteString("| 模式 | Recall@1 | Recall@3 | Recall@5 | MRR | 未命中率 | 无关问题误报率 | 平均耗时(ms) |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, s := range summaries {
		fmt.Fprintf(&b, "| %s | %.3f | %.3f | %.3f | %.3f | %.3f | %.3f | %.0f |\n",
			s.Mode, s.RecallAt1, s.RecallAt3, s.RecallAt5, s.MRR, s.MissRate, s.FalseAlarmRate, s.AvgLatencyMS)
	}
	return b.String()
}

// ---------- evolution ----------

type pairOutcome struct {
	SID         string `json:"sid"`
	PairType    string `json:"pair_type"`
	Expected    string `json:"expected"`
	Predicted   string `json:"predicted"`
	Judged      bool   `json:"judged"`
	EvidenceOK  bool   `json:"evidence_located"`
	TriggerID   uint   `json:"trigger_id"`
	CandidateID uint   `json:"candidate_id"`
	// Similarity 与生产一致：标题 + 正文前 1000 字的向量余弦相似度。
	// 它只决定这条候选能否进入判定，进入后的结论类型由模型判断。
	Similarity float64 `json:"similarity"`
	// ThresholdNote 说明相似度是否足以让候选进入模型判定。
	ThresholdNote string `json:"threshold_note"`
	Note        string `json:"note"`
}

func mustEvolution(user *model.UserModel, dataDir, outDir, mode string, cfg *config.Config) {
	pairs := loadPairs(user, dataDir, outDir)
	similarities := measureSimilarity(user, pairs, cfg)
	var outcomes []pairOutcome
	switch mode {
	case "pairwise":
		outcomes = judgePairs(user, pairs, cfg)
	case "collect":
		outcomes = collectAlerts(user, pairs)
	default:
		fatal(fmt.Errorf("未知 --mode %q，只支持 pairwise 或 collect", mode))
	}
	// 把相似度与阈值判断附在结果上：端到端漏检时，能直接看出是模型判错还是根本没进判定。
	for i := range outcomes {
		outcomes[i].Similarity = similarities[outcomes[i].SID]
		outcomes[i].ThresholdNote = thresholdNote(outcomes[i].Expected, outcomes[i].Similarity)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fatal(err)
	}
	// 两种模式分开落盘：pairwise 衡量模型判定，collect 衡量端到端，
	// 覆盖同一份文件会让后跑的一次把前一次的结论冲掉。
	writeJSON(filepath.Join(outDir, "evolution-"+mode+"-outcomes.json"), outcomes)
	table := evolutionTable(outcomes)
	writeText(filepath.Join(outDir, "evolution-"+mode+"-summary.md"), table)
	fmt.Println(table)
}

// measureSimilarity 复刻生产演化检查的打分方式，而不是简单算两条知识的余弦：
// 生产是把「标题 + 正文前 1000 字」编码成查询向量，再到分块集合里检索，
// 取该候选知识所有分块的最高分。两者口径不同，混用会把阈值判断算错。
func measureSimilarity(user *model.UserModel, pairs []pairRecord, cfg *config.Config) map[string]float64 {
	chat := newChatService(cfg)
	capability, version, _, err := chat.ResolveEmbedding(context.Background(), user.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "无法计算相似度（Embedding 不可用）: %v\n", err)
		return map[string]float64{}
	}
	embedder := &service.MastraEmbedder{AIURL: cfg.ChatAIURL, Secret: cfg.InternalAPISecret}
	vectors := &service.QdrantClient{URL: cfg.QdrantURL, APIKey: cfg.QdrantAPIKey}
	chunks := &repository.RagRepository{DB: database.DB}
	collections, err := chunks.Collections(context.Background(), user.ID, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取向量集合失败: %v\n", err)
		return map[string]float64{}
	}
	// 与 vectorCollection 保持一致：集合名固定为 zhixu_u<user>_v<version>。
	collections = append(collections, fmt.Sprintf("zhixu_u%d_v%d", user.ID, version))

	out := make(map[string]float64, len(pairs))
	for _, pair := range pairs {
		trigger := mustKnowledge(user.ID, pair.TriggerID)
		embedded, err := embedder.Embed(context.Background(), capability, []string{similarityText(trigger)})
		if err != nil || len(embedded) != 1 {
			fmt.Fprintf(os.Stderr, "%s 相似度计算失败: %v\n", pair.SID, err)
			continue
		}
		best := 0.0
		for _, collection := range collections {
			matches, err := vectors.Search(context.Background(), collection, user.ID, embedded[0], 30, 0)
			if err != nil {
				continue
			}
			for _, match := range matches {
				if match.KnowledgeID == pair.CandidateID && match.Score > best {
					best = match.Score
				}
			}
		}
		out[pair.SID] = best
	}
	return out
}

// similarityText 复刻 relationSourceBody 的取法：标题 + 换行 + 正文前 1000 字。
func similarityText(item *model.KnowledgeItem) string {
	body := []rune(utils.NormalizeKnowledgeContent(item.Content))
	if len(body) > 1000 {
		body = body[:1000]
	}
	return item.Title + "\n" + string(body)
}

// thresholdNote 说明这条样本在真实流水线里能否进入模型判定。
func thresholdNote(expected string, similarity float64) string {
	// 生产只有一道候选门槛：低于 EvolveCandidateMinScore 的近邻不会进入模型判定
	// （见 evolve_task.go 的 candidates），进入后的结论类型完全由模型决定。
	if similarity < service.EvolveCandidateMinScore {
		return fmt.Sprintf("相似度低于候选门槛 %.2f，未进入模型判定", service.EvolveCandidateMinScore)
	}
	if expected == "none" {
		return "对照样本进入模型判定，存在误报风险"
	}
	return "已进入模型判定，结论类型由模型判断"
}

// loadPairs 优先读取 seed 阶段写下的映射；缺失时按标题重新解析，便于单独重跑评测。
func loadPairs(user *model.UserModel, dataDir, outDir string) []pairRecord {
	path := filepath.Join(outDir, "pairs.json")
	if _, err := os.Stat(path); err == nil {
		var pairs []pairRecord
		if data, err := os.ReadFile(path); err == nil && json.Unmarshal(data, &pairs) == nil {
			for i := range pairs {
				pairs[i].TriggerID = mustKnowledgeID(user.ID, pairs[i].TriggerTitle)
				pairs[i].CandidateID = mustKnowledgeID(user.ID, pairs[i].CandidateTitle)
			}
			return pairs
		}
	}
	samples := mustLoad[evolutionSample](filepath.Join(dataDir, "演化样本.jsonl"))
	pairs := make([]pairRecord, 0, len(samples))
	for _, sample := range samples {
		triggerTitle := firstLineTitle(sample.Trigger)
		candidateTitle := firstLineTitle(sample.Candidate)
		pairs = append(pairs, pairRecord{
			SID: sample.SID, PairType: sample.PairType, Expected: sample.Expected,
			TriggerTitle: triggerTitle, CandidateTitle: candidateTitle,
			TriggerID:   mustKnowledgeID(user.ID, triggerTitle),
			CandidateID: mustKnowledgeID(user.ID, candidateTitle),
			Note:        sample.Note,
		})
	}
	return pairs
}

// judgePairs 把标注好的候选对直接交给生产 EvolveClient：相似度按 0.95 传入，
// 让所有候选都通过门槛，从而单独衡量模型判定本身的质量。
func judgePairs(user *model.UserModel, pairs []pairRecord, cfg *config.Config) []pairOutcome {
	chat := newChatService(cfg)
	client := &service.EvolveClient{AIURL: cfg.ChatAIURL, Secret: cfg.InternalAPISecret}
	capability, err := chat.ResolveChatModel(context.Background(), user.ID)
	if err != nil {
		fatal(fmt.Errorf("解析对话模型失败: %w", err))
	}

	outcomes := make([]pairOutcome, 0, len(pairs))
	for _, pair := range pairs {
		trigger := mustKnowledge(user.ID, pair.TriggerID)
		candidate := mustKnowledge(user.ID, pair.CandidateID)
		judgements, err := client.Evolve(context.Background(), capability, pair.TriggerID, pair.TriggerID,
			trigger.Title, excerpt(trigger.Content),
			[]service.EvolveCandidate{{
				KnowledgeID: candidate.ID, Title: candidate.Title,
				Excerpt: excerpt(candidate.Content), Score: 0.95, Origin: service.EvolveOriginVectorNeighbor,
			}})
		outcome := pairOutcome{
			SID: pair.SID, PairType: pair.PairType, Expected: pair.Expected,
			Predicted: "none", TriggerID: pair.TriggerID, CandidateID: pair.CandidateID, Note: pair.Note,
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s 判定失败: %v\n", pair.SID, err)
			outcomes = append(outcomes, outcome)
			continue
		}
		for _, judgement := range judgements {
			if judgement.TargetID != candidate.ID {
				continue
			}
			outcome.Judged = true
			outcome.Predicted = string(judgement.Type)
			outcome.EvidenceOK = evidenceLocated(trigger.Content, candidate.Content, judgement.Evidence)
			break
		}
		outcomes = append(outcomes, outcome)
	}
	return outcomes
}

// collectAlerts 读取真实流水线写下的提醒，衡量端到端效果（含候选召回与模型判定）。
func collectAlerts(user *model.UserModel, pairs []pairRecord) []pairOutcome {
	var alerts []model.EvolutionAlert
	if err := database.DB.Where("user_id = ?", user.ID).Find(&alerts).Error; err != nil {
		fatal(err)
	}
	byPair := map[string]string{}
	for _, alert := range alerts {
		ids := alert.SubjectIDList()
		if len(ids) != 2 {
			continue
		}
		key := pairKey(ids[0], ids[1])
		// 同一对可能有多条不同类型的提醒，重复优先与生产中一致。
		if current, ok := byPair[key]; !ok || alertPriority(string(alert.AlertType)) < alertPriority(current) {
			byPair[key] = string(alert.AlertType)
		}
	}
	outcomes := make([]pairOutcome, 0, len(pairs))
	for _, pair := range pairs {
		predicted, ok := byPair[pairKey(pair.TriggerID, pair.CandidateID)]
		if !ok {
			predicted = "none"
		}
		outcomes = append(outcomes, pairOutcome{
			SID: pair.SID, PairType: pair.PairType, Expected: pair.Expected,
			Predicted: predicted, Judged: ok,
			TriggerID: pair.TriggerID, CandidateID: pair.CandidateID, Note: pair.Note,
		})
	}
	return outcomes
}

func pairKey(a, b uint) string {
	if a > b {
		a, b = b, a
	}
	return fmt.Sprintf("%d-%d", a, b)
}

func alertPriority(alertType string) int {
	switch alertType {
	case "duplicate":
		return 0
	case "conflict":
		return 1
	case "outdated":
		return 2
	default:
		return 3
	}
}

// evidenceLocated 复刻生产侧 validEvidence 的校验口径：引文规范化后
// 必须能在它声明的那一侧正文里逐字找到，跨侧拼凑不算有效证据。
func evidenceLocated(trigger, candidate string, evidence []service.EvolutionEvidence) bool {
	if len(evidence) == 0 {
		return false
	}
	triggerBody := utils.NormalizeKnowledgeContent(trigger)
	candidateBody := utils.NormalizeKnowledgeContent(candidate)
	for _, item := range evidence {
		quote := utils.NormalizeKnowledgeContent(item.Quote)
		if quote == "" {
			return false
		}
		// 模型偶尔会把引文挂到另一侧，这里按正文实际包含关系判断。
		if !strings.Contains(triggerBody, quote) && !strings.Contains(candidateBody, quote) {
			return false
		}
	}
	return true
}

func evolutionTable(outcomes []pairOutcome) string {
	classes := []string{"duplicate", "outdated", "conflict", "none"}
	type stat struct{ tp, fp, fn int }
	stats := map[string]*stat{}
	for _, class := range classes {
		stats[class] = &stat{}
	}
	correct, judged := 0, 0
	for _, outcome := range outcomes {
		if outcome.Judged {
			judged++
		}
		if outcome.Predicted == outcome.Expected {
			correct++
		}
		if stats[outcome.Predicted] != nil {
			if outcome.Predicted == outcome.Expected {
				stats[outcome.Predicted].tp++
			} else {
				stats[outcome.Predicted].fp++
			}
		}
		if stats[outcome.Expected] != nil && outcome.Predicted != outcome.Expected {
			stats[outcome.Expected].fn++
		}
	}

	var b strings.Builder
	b.WriteString("| 类型 | 精确率 | 召回率 | F1 | 支持数 |\n")
	b.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, class := range classes {
		s := stats[class]
		precision := ratio(s.tp, s.tp+s.fp)
		recall := ratio(s.tp, s.tp+s.fn)
		f1 := 0.0
		if precision+recall > 0 {
			f1 = 2 * precision * recall / (precision + recall)
		}
		fmt.Fprintf(&b, "| %s | %.3f | %.3f | %.3f | %d |\n", class, precision, recall, f1, s.tp+s.fn)
	}
	fmt.Fprintf(&b, "\n总体准确率 %.3f（%d/%d），产生提醒的样本 %d/%d。\n",
		ratio(correct, len(outcomes)), correct, len(outcomes), judged, len(outcomes))
	return b.String()
}

// mustSweep 做阈值敏感性分析：保持模型判定不变，只看相似度阈值怎么影响
// 「能进入判定」的比例。端到端漏检里有多少是阈值造成的，这张表能直接回答。
func mustSweep(outDir string) {
	path := filepath.Join(outDir, "evolution-collect-outcomes.json")
	data, err := os.ReadFile(path)
	if err != nil {
		fatal(fmt.Errorf("读取 %s 失败，请先跑 evolution --mode collect: %w", path, err))
	}
	var outcomes []pairOutcome
	if err := json.Unmarshal(data, &outcomes); err != nil {
		fatal(err)
	}

	// 相似度只决定候选能否进入判定，敏感性分析因此只有一个维度：候选门槛。
	gates := []float64{0.60, 0.65, 0.70, 0.75, 0.80}

	var b strings.Builder
	b.WriteString("| 候选门槛 | 重复可判定 | 冲突可判定 | 过时可判定 | 对照误入 |\n")
	b.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, gate := range gates {
		counts := map[string][2]int{} // 类型 -> [可判定, 总数]
		for _, outcome := range outcomes {
			pair := counts[outcome.PairType]
			pair[1]++
			if passThreshold(outcome.PairType, outcome.Similarity, gate) {
				pair[0]++
			}
			counts[outcome.PairType] = pair
		}
		fmt.Fprintf(&b, "| %.2f | %s | %s | %s | %s |\n", gate,
			fraction(counts["duplicate"]), fraction(counts["conflict"]),
			fraction(counts["outdated"]), fraction(counts["negative"]))
	}
	writeText(filepath.Join(outDir, "evolution-threshold-sweep.md"), b.String())
	fmt.Println(b.String())
}

// passThreshold 判断样本是否达到候选门槛；达到后类型交给模型，不再按分数分派。
func passThreshold(_ string, similarity, gate float64) bool {
	return similarity >= gate
}

// mustGrid 穷举单一候选门槛，回答「只调门槛，估计准确率最多能到多少」。
// 估计口径假设进入判定的候选模型都能判对，模型本身的效果由 pairwise 实测。
func mustGrid(outDir string) {
	path := filepath.Join(outDir, "evolution-collect-outcomes.json")
	data, err := os.ReadFile(path)
	if err != nil {
		fatal(fmt.Errorf("读取 %s 失败，请先跑 evolution --mode collect: %w", path, err))
	}
	var outcomes []pairOutcome
	if err := json.Unmarshal(data, &outcomes); err != nil {
		fatal(err)
	}

	type result struct {
		gate    float64
		correct int
	}
	var best []result
	bestCorrect := -1
	for gate := 0.50; gate <= 0.95+1e-9; gate += 0.01 {
		correct := 0
		for _, outcome := range outcomes {
			if allowsType(outcome.Similarity, gate, outcome.Expected) {
				correct++
			}
		}
		if correct > bestCorrect {
			bestCorrect = correct
			best = []result{{gate, correct}}
		} else if correct == bestCorrect {
			best = append(best, result{gate, correct})
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "网格搜索范围：候选门槛 0.50–0.95，步长 0.01。\n\n")
	fmt.Fprintf(&b, "最优估计准确率 %.3f（%d/%d），达到该准确率的门槛共 %d 个。\n\n",
		ratio(bestCorrect, len(outcomes)), bestCorrect, len(outcomes), len(best))
	b.WriteString("| 候选门槛 | 估计准确率 |\n")
	b.WriteString("| --- | --- |\n")
	for i, item := range best {
		if i == 10 {
			fmt.Fprintf(&b, "| … | 其余 %d 个门槛相同 |\n", len(best)-10)
			break
		}
		fmt.Fprintf(&b, "| %.2f | %.3f |\n", item.gate,
			ratio(item.correct, len(outcomes)))
	}

	// 与生产当前门槛对比，量化「不调门槛会损失多少」。
	current := 0
	for _, outcome := range outcomes {
		if allowsType(outcome.Similarity, service.EvolveCandidateMinScore, outcome.Expected) {
			current++
		}
	}
	fmt.Fprintf(&b, "\n当前生产门槛 %.2f 的估计准确率 %.3f（%d/%d），与最优相差 %.3f。\n",
		service.EvolveCandidateMinScore,
		ratio(current, len(outcomes)), current, len(outcomes),
		ratio(bestCorrect-current, len(outcomes)))

	b.WriteString("\n## 分布重叠检查\n\n")
	b.WriteString(distributionOverlap(outcomes))

	writeText(filepath.Join(outDir, "evolution-threshold-grid.md"), b.String())
	fmt.Println(b.String())
}

// distributionOverlap 检查三类样本的相似度区间是否可分。
//
// 这决定了相似度能否单独区分类型：如果重复样本的最高分低于冲突样本的最低分，就能切开；
// 如果两类区间互相包含，那么任何按分数分派类型的规则都会在一类样本上出错。
func distributionOverlap(outcomes []pairOutcome) string {
	ranges := map[string][2]float64{}
	counts := map[string]int{}
	for _, outcome := range outcomes {
		low, ok := ranges[outcome.PairType]
		if !ok {
			ranges[outcome.PairType] = [2]float64{outcome.Similarity, outcome.Similarity}
		} else {
			if outcome.Similarity < low[0] {
				low[0] = outcome.Similarity
			}
			if outcome.Similarity > low[1] {
				low[1] = outcome.Similarity
			}
			ranges[outcome.PairType] = low
		}
		counts[outcome.PairType]++
	}

	var b strings.Builder
	b.WriteString("| 类型 | 样本数 | 相似度区间 |\n| --- | --- | --- |\n")
	for _, class := range []string{"duplicate", "conflict", "outdated", "negative"} {
		r, ok := ranges[class]
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "| %s | %d | %.3f–%.3f |\n", class, counts[class], r[0], r[1])
	}

	dup, dupOK := ranges["duplicate"]
	conflict, conflictOK := ranges["conflict"]
	if dupOK && conflictOK {
		if conflict[0] < dup[0] && conflict[1] > dup[1] {
			fmt.Fprintf(&b, "\n重复区间 [%.3f, %.3f] 被冲突区间 [%.3f, %.3f] 完整包含，"+
				"两者在相似度维度上不可分。任何按相似度分派类型的规则只要低到能覆盖全部重复样本，"+
				"就会把落在其上方的冲突样本误判成重复，反之亦然。\n",
				dup[0], dup[1], conflict[0], conflict[1])
		} else {
			b.WriteString("\n重复与冲突区间存在可分间隙，但类型仍由模型判定。\n")
		}
	}
	return b.String()
}

// allowsType 判断在给定候选门槛下，这条样本的期望结论是否有机会被判出。
// 门槛只决定是否送模型；进入后类型不再受相似度限制，因此正样本过了门槛就算「判得出」。
// 对照样本本身不产生提醒，过门槛即说明存在误报风险。
func allowsType(similarity, gate float64, expected string) bool {
	passed := similarity >= gate
	if expected == "none" {
		return !passed
	}
	return passed
}

func fraction(pair [2]int) string {
	if pair[1] == 0 {
		return "-"
	}
	return fmt.Sprintf("%d/%d", pair[0], pair[1])
}

func ratio(num, den int) float64 {
	if den == 0 {
		return 0
	}
	return float64(num) / float64(den)
}

// ---------- 工具 ----------

func mustUser(username string) *model.UserModel {
	var user model.UserModel
	if err := database.DB.Where("username = ?", username).First(&user).Error; err != nil {
		fatal(fmt.Errorf("找不到用户 %s: %w", username, err))
	}
	return &user
}

func mustKnowledgeID(userID uint, title string) uint {
	var row model.KnowledgeItem
	if err := database.DB.Where("user_id = ? AND title = ?", userID, title).First(&row).Error; err != nil {
		fatal(fmt.Errorf("找不到知识 %q: %w", title, err))
	}
	return row.ID
}

func mustKnowledge(userID, id uint) *model.KnowledgeItem {
	var row model.KnowledgeItem
	if err := database.DB.Where("user_id = ? AND id = ?", userID, id).First(&row).Error; err != nil {
		fatal(fmt.Errorf("读取知识 %d 失败: %w", id, err))
	}
	return &row
}

func mustCredentials(cfg *config.Config) *service.ModelCredentials {
	credentials, err := service.NewModelCredentials(cfg.ModelConfigEncryptionKey)
	if err != nil {
		fatal(err)
	}
	return credentials
}

// newChatService 与 cmd/server 的装配保持一致，尤其是私网地址策略：
// 少了 AllowPrivateNetworks，本地 Ollama 这类 127.0.0.1 的 Embedding 会被拒绝，
// 语义路会静默降级成关键词，评测结果就失真了。
func newChatService(cfg *config.Config) *service.ChatService {
	return &service.ChatService{
		Store:                   &repository.AIConfigRepository{DB: database.DB},
		Credentials:             mustCredentials(cfg),
		AllowedPrivateEndpoints: cfg.ModelAllowedPrivateEndpoints,
		AllowPrivateNetworks:    cfg.ModelAllowPrivateNetworks,
	}
}

func excerpt(content string) string {
	runes := []rune(utils.NormalizeKnowledgeContent(content))
	if len(runes) > 1200 {
		return string(runes[:1200])
	}
	return string(runes)
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func writeJSON(path string, value any) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		fatal(err)
	}
}

func writeText(path, body string) {
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "错误:", err)
	os.Exit(1)
}

