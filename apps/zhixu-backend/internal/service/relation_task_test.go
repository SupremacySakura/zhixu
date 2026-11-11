package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// relationTaskResolverFake 分别提供对话与嵌入能力，且 model_id 不同：
// 任何拿错能力的调用都会被测试直接暴露。
type relationTaskResolverFake struct {
	chat       *ChatCapability
	embedding  *ChatCapability
	chatErr    error
	embedErr   error
	chatCalls  int
	embedCalls int
}

func (f *relationTaskResolverFake) ResolveChatModel(context.Context, uint) (*ChatCapability, error) {
	f.chatCalls++
	if f.chatErr != nil {
		return nil, f.chatErr
	}
	return f.chat, nil
}

func (f *relationTaskResolverFake) ResolveEmbedding(context.Context, uint) (*ChatCapability, uint64, uint64, error) {
	f.embedCalls++
	if f.embedErr != nil {
		return nil, 0, 0, f.embedErr
	}
	return f.embedding, 3, 1, nil
}

// relationTaskEmbedderFake 记录向量化时拿到的能力，证明嵌入走的是 embedding 一侧。
type relationTaskEmbedderFake struct {
	vectorEmbedderFake
	caps []*ChatCapability
}

func (f *relationTaskEmbedderFake) Embed(ctx context.Context, cap *ChatCapability, texts []string) ([][]float64, error) {
	f.caps = append(f.caps, cap)
	return f.vectorEmbedderFake.Embed(ctx, cap, texts)
}

type relationTaskKnowledgeFake struct {
	items     map[uint]*model.KnowledgeItem
	listCalls int
	listedIDs []uint
}

func (f *relationTaskKnowledgeFake) Get(_ context.Context, userID, id uint) (*model.KnowledgeItem, error) {
	item, ok := f.items[id]
	if !ok || item.UserID != userID {
		return nil, gorm.ErrRecordNotFound
	}
	return item, nil
}

func (f *relationTaskKnowledgeFake) ListByIDs(_ context.Context, userID uint, ids []uint) ([]model.KnowledgeItem, error) {
	f.listCalls++
	f.listedIDs = append(f.listedIDs, ids...)
	var out []model.KnowledgeItem
	for _, id := range ids {
		if item, ok := f.items[id]; ok && item.UserID == userID {
			out = append(out, *item)
		}
	}
	return out, nil
}

type relationTaskPairFake struct {
	rows      []repository.RelationRecord
	created   []*model.KnowledgeRelation
	listErr   error
	createErr error
}

func (f *relationTaskPairFake) ListByKnowledge(context.Context, uint, uint) ([]repository.RelationRecord, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.rows, nil
}

func (f *relationTaskPairFake) CreateSuggestion(_ context.Context, relation *model.KnowledgeRelation) (bool, error) {
	if f.createErr != nil {
		return false, f.createErr
	}
	copied := *relation
	copied.ID = uint(100 + len(f.created))
	f.created = append(f.created, &copied)
	return true, nil
}

type relationTaskSearcherFake struct {
	matches    []VectorMatch
	err        error
	collection string
	user       uint
	vector     []float64
	topK       int
	minScore   float64
}

func (f *relationTaskSearcherFake) Search(_ context.Context, collection string, user uint, vector []float64, topK int, minScore float64) ([]VectorMatch, error) {
	f.collection, f.user = collection, user
	f.vector, f.topK, f.minScore = append([]float64(nil), vector...), topK, minScore
	if f.err != nil {
		return nil, f.err
	}
	return append([]VectorMatch(nil), f.matches...), nil
}

type relationTaskSuggesterFake struct {
	relations      []RelateRelation
	err            error
	calls          int
	gotTaskID      uint
	gotKnowledgeID uint
	gotTitle       string
	gotExcerpt     string
	gotCandidates  []RelateCandidate
	gotCap         *ChatCapability
}

func (f *relationTaskSuggesterFake) Relate(_ context.Context, snapshot *ChatCapability, taskID, knowledgeID uint, title, excerpt string, candidates []RelateCandidate) ([]RelateRelation, error) {
	f.calls++
	f.gotCap = snapshot
	f.gotTaskID, f.gotKnowledgeID = taskID, knowledgeID
	f.gotTitle, f.gotExcerpt = title, excerpt
	f.gotCandidates = append([]RelateCandidate(nil), candidates...)
	if f.err != nil {
		return nil, f.err
	}
	return f.relations, nil
}

type relationTaskFixture struct {
	processor *RelationProcessor
	resolver  *relationTaskResolverFake
	knowledge *relationTaskKnowledgeFake
	pairs     *relationTaskPairFake
	embedder  *relationTaskEmbedderFake
	searcher  *relationTaskSearcherFake
	suggester *relationTaskSuggesterFake
	task      *model.AiTask
}

func newRelationTaskFixture() *relationTaskFixture {
	now := time.Now()
	f := &relationTaskFixture{
		resolver: &relationTaskResolverFake{
			chat: &ChatCapability{
				Model:    model.ProviderModel{ID: 11, ModelID: "deepseek-chat", Kind: "chat"},
				Provider: model.ModelProvider{ID: 3, Protocol: "chat", BaseURL: "https://api.example.com/v1"},
				Key:      "sk-chat",
			},
			embedding: &ChatCapability{
				Model:    model.ProviderModel{ID: 12, ModelID: "text-embedding-3-small", Kind: "embedding", Dimension: 2},
				Provider: model.ModelProvider{ID: 3, Protocol: "chat", BaseURL: "https://api.example.com/v1"},
				Key:      "sk-embed",
			},
		},
		knowledge: &relationTaskKnowledgeFake{items: map[uint]*model.KnowledgeItem{}},
		pairs:     &relationTaskPairFake{},
		embedder:  &relationTaskEmbedderFake{},
		searcher:  &relationTaskSearcherFake{},
		suggester: &relationTaskSuggesterFake{},
	}
	f.knowledge.items[12] = &model.KnowledgeItem{
		ID: 12, UserID: 2, Title: "源标题", Summary: "源摘要", Content: "源正文",
		Status: model.KnowledgeReady, Version: 5,
	}
	f.knowledge.items[21] = &model.KnowledgeItem{ID: 21, UserID: 2, Title: "候选标题", Summary: "候选摘要", Content: "候选正文", Status: model.KnowledgeReady}
	f.knowledge.items[22] = &model.KnowledgeItem{ID: 22, UserID: 2, Title: "正文候选", Content: "候选正文二", Status: model.KnowledgeReady}
	f.knowledge.items[24] = &model.KnowledgeItem{ID: 24, UserID: 2, Title: "已归档候选", Summary: "归档", Status: model.KnowledgeReady, ArchivedAt: &now}
	f.knowledge.items[25] = &model.KnowledgeItem{ID: 25, UserID: 2, Title: "已有关系候选", Summary: "已有", Status: model.KnowledgeReady}
	f.knowledge.items[26] = &model.KnowledgeItem{ID: 26, UserID: 2, Title: "更新候选", Summary: "更新摘要", Status: model.KnowledgeReady}
	f.knowledge.items[27] = &model.KnowledgeItem{ID: 27, UserID: 2, Title: "已拒绝候选", Summary: "拒绝", Status: model.KnowledgeReady}

	f.pairs.rows = []repository.RelationRecord{
		{
			KnowledgeRelation: model.KnowledgeRelation{ID: 31, UserID: 2, SourceID: 12, TargetID: 25, RelationType: model.RelationTypeRelated, Status: model.RelationSuggested, Source: model.RelationSourceAI},
			CounterpartID:     25, CounterpartTitle: "已有关系候选",
		},
		{
			KnowledgeRelation: model.KnowledgeRelation{ID: 32, UserID: 2, SourceID: 12, TargetID: 27, RelationType: model.RelationTypeContrasts, Status: model.RelationRejected, Source: model.RelationSourceAI},
			CounterpartID:     27, CounterpartTitle: "已拒绝候选",
		},
	}
	f.searcher.matches = []VectorMatch{
		{ChunkID: 1, KnowledgeID: 12, Score: 0.99},
		{ChunkID: 2, KnowledgeID: 21, Score: 0.82},
		{ChunkID: 3, KnowledgeID: 21, Score: 0.90},
		{ChunkID: 4, KnowledgeID: 22, Score: 0.60},
		{ChunkID: 5, KnowledgeID: 23, Score: 0.54},
		{ChunkID: 6, KnowledgeID: 24, Score: 0.70},
		{ChunkID: 7, KnowledgeID: 25, Score: 0.71},
		{ChunkID: 8, KnowledgeID: 26, Score: 0.72},
		{ChunkID: 9, KnowledgeID: 27, Score: 0.75},
	}
	f.suggester.relations = []RelateRelation{
		{TargetID: 21, RelationType: model.RelationTypeRelated, Confidence: 0.78, Evidence: RelateEvidence{Reason: "主题相近", SourceQuote: "源片段", TargetQuote: "候选片段"}},
		{TargetID: 22, RelationType: model.RelationTypeDependsOn, Confidence: 0.55, Evidence: RelateEvidence{Reason: "置信度不足"}},
		{TargetID: 26, RelationType: model.RelationTypeUpdates, Confidence: 0.90, Evidence: RelateEvidence{Reason: "更新版本"}},
		{TargetID: 26, RelationType: model.RelationTypeUpdates, Confidence: 0.80, Evidence: RelateEvidence{Reason: "同对同类型重复"}},
		{TargetID: 99, RelationType: model.RelationTypeRelated, Confidence: 0.92, Evidence: RelateEvidence{Reason: "不在候选"}},
		{TargetID: 21, RelationType: model.RelationTypeRelated, Confidence: 0.95, Evidence: RelateEvidence{Reason: "最高分"}},
		{TargetID: 21, RelationType: model.RelationType("friend"), Confidence: 0.93, Evidence: RelateEvidence{Reason: "非法类型"}},
	}
	f.task = &model.AiTask{ID: 5, UserID: 2, TaskType: model.AiTaskTypeRelation, TargetID: 12, Status: model.AiTaskRunning}
	f.processor = &RelationProcessor{
		Chat: f.resolver, Embedding: f.resolver, Knowledge: f.knowledge, Relations: f.pairs,
		Embedder: f.embedder, Vectors: f.searcher, Client: f.suggester,
	}
	return f
}

func requireTerminalError(t *testing.T, err error) *AiTaskTerminalError {
	t.Helper()
	var terminal *AiTaskTerminalError
	require.ErrorAs(t, err, &terminal)
	return terminal
}

// 全链路：一次嵌入取 top50，归并去重后过滤自身/归档/已有关系/低分，再落建议。
func TestRelationProcessorCreatesSuggestions(t *testing.T) {
	f := newRelationTaskFixture()

	payload, err := f.processor.Process(context.Background(), f.task)

	require.NoError(t, err)
	require.Len(t, f.embedder.texts, 1)
	require.Len(t, f.embedder.caps, 1)
	require.Equal(t, "embedding", f.embedder.caps[0].Model.Kind)
	require.Equal(t, "text-embedding-3-small", f.embedder.caps[0].Model.ModelID)
	require.Contains(t, f.embedder.texts[0], "源标题")
	require.Contains(t, f.embedder.texts[0], "源摘要")
	require.NotContains(t, f.embedder.texts[0], "源正文", "有摘要时不嵌入正文")
	require.Equal(t, "zhixu_u2_v3", f.searcher.collection)
	require.Equal(t, uint(2), f.searcher.user)
	require.Equal(t, 50, f.searcher.topK)
	require.Equal(t, float64(0), f.searcher.minScore)
	require.Len(t, f.searcher.vector, 2)

	require.Equal(t, 1, f.suggester.calls)
	require.Equal(t, uint(5), f.suggester.gotTaskID)
	require.Equal(t, uint(12), f.suggester.gotKnowledgeID)
	require.Equal(t, "源标题", f.suggester.gotTitle)
	require.Equal(t, "源摘要", f.suggester.gotExcerpt)
	require.Equal(t, "chat", f.suggester.gotCap.Model.Kind)
	require.Equal(t, "deepseek-chat", f.suggester.gotCap.Model.ModelID)
	require.Len(t, f.suggester.gotCandidates, 3)
	require.Equal(t, uint(21), f.suggester.gotCandidates[0].KnowledgeID)
	require.InDelta(t, 0.90, f.suggester.gotCandidates[0].Score, 1e-9)
	require.Equal(t, "候选摘要", f.suggester.gotCandidates[0].Excerpt)
	require.Equal(t, uint(26), f.suggester.gotCandidates[1].KnowledgeID)
	require.Equal(t, uint(22), f.suggester.gotCandidates[2].KnowledgeID)
	require.Equal(t, "候选正文二", f.suggester.gotCandidates[2].Excerpt)

	require.Len(t, f.pairs.created, 2)
	first := f.pairs.created[0]
	require.Equal(t, uint(2), first.UserID)
	require.Equal(t, uint(12), first.SourceID)
	require.Equal(t, uint(21), first.TargetID)
	require.Equal(t, model.RelationTypeRelated, first.RelationType)
	require.Equal(t, model.RelationSuggested, first.Status)
	require.Equal(t, model.RelationSourceAI, first.Source)
	require.InDelta(t, 0.95, first.Confidence, 1e-9)
	var evidence RelateEvidence
	require.NoError(t, json.Unmarshal([]byte(first.Evidence), &evidence))
	require.Equal(t, "最高分", evidence.Reason)
	require.Equal(t, uint(26), f.pairs.created[1].TargetID)
	require.Equal(t, model.RelationTypeUpdates, f.pairs.created[1].RelationType)

	var result RelationPayload
	require.NoError(t, json.Unmarshal(payload, &result))
	require.Equal(t, uint(12), result.KnowledgeID)
	require.Equal(t, uint(5), result.BaseVersion)
	require.Equal(t, 2, result.SuggestionCount)
}

// 去重键包含关系类型：同一对知识允许同时保留 related 与 depends_on，同类型重复仍只留最高分。
func TestRelationProcessorKeepsDifferentRelationTypesForSamePair(t *testing.T) {
	f := newRelationTaskFixture()
	f.pairs.rows = nil
	f.suggester.relations = []RelateRelation{
		{TargetID: 21, RelationType: model.RelationTypeRelated, Confidence: 0.90, Evidence: RelateEvidence{Reason: "主题相近"}},
		{TargetID: 21, RelationType: model.RelationTypeDependsOn, Confidence: 0.85, Evidence: RelateEvidence{Reason: "依赖关系"}},
		{TargetID: 21, RelationType: model.RelationTypeRelated, Confidence: 0.80, Evidence: RelateEvidence{Reason: "同类型重复"}},
	}

	payload, err := f.processor.Process(context.Background(), f.task)

	require.NoError(t, err)
	require.Len(t, f.pairs.created, 2)
	require.Equal(t, model.RelationTypeRelated, f.pairs.created[0].RelationType)
	require.Equal(t, model.RelationTypeDependsOn, f.pairs.created[1].RelationType)
	var result RelationPayload
	require.NoError(t, json.Unmarshal(payload, &result))
	require.Equal(t, 2, result.SuggestionCount)
}

// /relate 请求体必须携带对话模型：embedding 模型只能用于向量化，不能拿去调对话接口。
func TestRelationProcessorSendsChatModelToRelate(t *testing.T) {
	f := newRelationTaskFixture()
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Errorf("解码 /relate 请求体失败: %v", err)
		}
		_, _ = w.Write([]byte(`{"relations":[{"target_id":21,"relation_type":"related","confidence":0.90,"evidence":{"reason":"主题相近","source_quote":"源","target_quote":"候选"}}]}`))
	}))
	defer server.Close()
	f.processor.Client = &RelateClient{AIURL: server.URL, Secret: "internal-secret"}

	_, err := f.processor.Process(context.Background(), f.task)
	require.NoError(t, err)

	// 向量化拿到的必须是 embedding 能力。
	require.Len(t, f.embedder.caps, 1)
	require.Equal(t, "embedding", f.embedder.caps[0].Model.Kind)
	require.Equal(t, "text-embedding-3-small", f.embedder.caps[0].Model.ModelID)

	// /relate 请求体里的 model_id 必须来自 chat 能力。
	modelBody, ok := body["model"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "deepseek-chat", modelBody["model_id"])
	require.Equal(t, "sk-chat", modelBody["api_key"])
	require.NotEqual(t, "text-embedding-3-small", modelBody["model_id"])
}

// 候选最多 10 条，分数 <0.55 的连详情都不查。
func TestRelationProcessorCapsCandidatesAndDropsLowScores(t *testing.T) {
	f := newRelationTaskFixture()
	f.pairs.rows = nil
	f.suggester.relations = nil
	f.searcher.matches = []VectorMatch{{ChunkID: 1, KnowledgeID: 12, Score: 0.99}}
	for i := 0; i < 12; i++ {
		id := uint(101 + i)
		f.knowledge.items[id] = &model.KnowledgeItem{ID: id, UserID: 2, Title: fmt.Sprintf("候选%d", id), Summary: "摘要", Status: model.KnowledgeReady}
		f.searcher.matches = append(f.searcher.matches, VectorMatch{ChunkID: uint(10 + i), KnowledgeID: id, Score: 0.90 - float64(i)*0.01})
	}
	f.searcher.matches = append(f.searcher.matches, VectorMatch{ChunkID: 30, KnowledgeID: 120, Score: 0.54})

	payload, err := f.processor.Process(context.Background(), f.task)

	require.NoError(t, err)
	require.Len(t, f.suggester.gotCandidates, 10)
	for i, candidate := range f.suggester.gotCandidates {
		require.Equal(t, uint(101+i), candidate.KnowledgeID)
		require.GreaterOrEqual(t, candidate.Score, 0.55)
	}
	require.NotContains(t, f.knowledge.listedIDs, uint(120))

	var result RelationPayload
	require.NoError(t, json.Unmarshal(payload, &result))
	require.Zero(t, result.SuggestionCount)
}

// 没有摘要时嵌入正文前 1000 字；送 /relate 的判定摘录单独按 2000 字上限截断，
// 候选摘录仍按 1200 字上限。
func TestRelationProcessorSeparatesEmbeddingAndJudgementExcerpts(t *testing.T) {
	f := newRelationTaskFixture()
	f.pairs.rows = nil
	f.suggester.relations = nil
	f.knowledge.items[12].Summary = ""
	f.knowledge.items[12].Content = strings.Repeat("源", 2500)
	f.knowledge.items[21].Summary = ""
	f.knowledge.items[21].Content = strings.Repeat("候", 1500)
	f.searcher.matches = []VectorMatch{
		{ChunkID: 1, KnowledgeID: 12, Score: 0.99},
		{ChunkID: 2, KnowledgeID: 21, Score: 0.90},
		{ChunkID: 3, KnowledgeID: 22, Score: 0.80},
	}

	_, err := f.processor.Process(context.Background(), f.task)

	require.NoError(t, err)
	require.Len(t, f.embedder.texts, 1)
	embedBody := strings.TrimPrefix(f.embedder.texts[0], "源标题\n")
	require.Equal(t, RelationEmbeddingBodyMaxRunes, len([]rune(embedBody)))
	require.Equal(t, 2000, len([]rune(f.suggester.gotExcerpt)))
	require.LessOrEqual(t, len([]rune(f.suggester.gotExcerpt)), RelationKnowledgeExcerptMaxRunes)
	require.LessOrEqual(t, RelationKnowledgeExcerptMaxRunes, 2000, "判定侧摘录不得超过 /relate 契约")
	require.Len(t, f.suggester.gotCandidates, 2)
	require.Equal(t, 1200, len([]rune(f.suggester.gotCandidates[0].Excerpt)))
}

// 标题也占用嵌入文本预算：超长标题 + 1000 字正文不得把 /vectorize 的单条文本顶到上限外，
// 判定侧摘录仍按自己的 2000 字上限截断，不受影响。
func TestRelationProcessorKeepsEmbeddingTextWithinBudget(t *testing.T) {
	f := newRelationTaskFixture()
	f.pairs.rows = nil
	f.suggester.relations = nil
	f.knowledge.items[12].Title = strings.Repeat("题", 200)
	f.knowledge.items[12].Summary = ""
	f.knowledge.items[12].Content = strings.Repeat("源", 2500)
	f.searcher.matches = []VectorMatch{
		{ChunkID: 1, KnowledgeID: 12, Score: 0.99},
		{ChunkID: 2, KnowledgeID: 21, Score: 0.90},
		{ChunkID: 3, KnowledgeID: 22, Score: 0.80},
	}

	_, err := f.processor.Process(context.Background(), f.task)

	require.NoError(t, err)
	require.Len(t, f.embedder.texts, 1)
	require.LessOrEqual(t, len([]rune(f.embedder.texts[0])), RelationEmbeddingTextMaxRunes, "嵌入文本超上限会被 AI 侧整批 400 拒绝")
	require.True(t, strings.HasPrefix(f.embedder.texts[0], strings.Repeat("题", 200)+"\n"))
	require.Equal(t, 2000, len([]rune(f.suggester.gotExcerpt)))
}

// 嵌入请求被 AI 侧拒绝（400/403）时直接判终态，不再退避重试。
func TestRelationProcessorTerminalWhenEmbeddingRejected(t *testing.T) {
	f := newRelationTaskFixture()
	f.embedder.err = &EmbeddingRejectedError{Status: http.StatusBadRequest}

	_, err := f.processor.Process(context.Background(), f.task)

	terminal := requireTerminalError(t, err)
	require.Contains(t, terminal.Message, "HTTP 400")
	require.Zero(t, f.suggester.calls)
	require.Empty(t, f.pairs.created)
}

// 知识不存在/已归档/未就绪属于终态失败，不做付费调用也不重试。
func TestRelationProcessorTerminalKnowledgeFailures(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name  string
		setup func(*relationTaskFixture)
		want  string
	}{
		{"不存在", func(f *relationTaskFixture) { delete(f.knowledge.items, 12) }, "知识不存在或已删除"},
		{"已归档", func(f *relationTaskFixture) { f.knowledge.items[12].ArchivedAt = &now }, "已归档"},
		{"未就绪", func(f *relationTaskFixture) { f.knowledge.items[12].Status = "processing" }, "尚未就绪"},
		{"正文为空", func(f *relationTaskFixture) {
			f.knowledge.items[12].Summary = ""
			f.knowledge.items[12].Content = ""
		}, "尚未就绪"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRelationTaskFixture()
			tc.setup(f)
			_, err := f.processor.Process(context.Background(), f.task)
			terminal := requireTerminalError(t, err)
			require.Contains(t, terminal.Message, tc.want)
			require.Empty(t, f.embedder.texts)
			require.Zero(t, f.suggester.calls)
		})
	}
}

func TestRelationProcessorTerminalWhenChatModelMissing(t *testing.T) {
	f := newRelationTaskFixture()
	f.resolver.chatErr = errors.New("未配置模型")
	_, err := f.processor.Process(context.Background(), f.task)
	terminal := requireTerminalError(t, err)
	require.Contains(t, terminal.Message, "未配置可用的对话模型")
	require.Zero(t, f.resolver.embedCalls, "对话模型缺失时不应再解析 Embedding")
	require.Empty(t, f.embedder.texts)
	require.Zero(t, f.suggester.calls)
}

func TestRelationProcessorTerminalWhenEmbeddingModelMissing(t *testing.T) {
	f := newRelationTaskFixture()
	f.resolver.embedErr = errors.New("未配置模型")
	_, err := f.processor.Process(context.Background(), f.task)
	terminal := requireTerminalError(t, err)
	require.Contains(t, terminal.Message, "未配置可用的 Embedding 模型")
	require.Empty(t, f.embedder.texts)
}

func TestRelationProcessorTerminalWhenCandidatesTooFew(t *testing.T) {
	f := newRelationTaskFixture()
	f.searcher.matches = []VectorMatch{
		{ChunkID: 1, KnowledgeID: 12, Score: 0.99},
		{ChunkID: 2, KnowledgeID: 23, Score: 0.54},
	}
	_, err := f.processor.Process(context.Background(), f.task)
	terminal := requireTerminalError(t, err)
	require.Contains(t, terminal.Message, "候选知识不足")
	require.Zero(t, f.suggester.calls)
}

// 422 是模型输出问题，重试无意义；其余 AI/Qdrant 错误保留可重试语义。
func TestRelationProcessorClassifiesAIErrors(t *testing.T) {
	t.Run("schema_invalid", func(t *testing.T) {
		f := newRelationTaskFixture()
		f.suggester.err = ErrRelateInvalidOutput
		_, err := f.processor.Process(context.Background(), f.task)
		terminal := requireTerminalError(t, err)
		require.Contains(t, terminal.Message, "AI 输出格式不符合要求")
		require.Empty(t, f.pairs.created)
	})
	t.Run("rejected", func(t *testing.T) {
		f := newRelationTaskFixture()
		f.suggester.err = fmt.Errorf("%w: 配置错误", ErrRelateRejected)
		_, err := f.processor.Process(context.Background(), f.task)
		requireTerminalError(t, err)
	})
	t.Run("upstream_timeout", func(t *testing.T) {
		f := newRelationTaskFixture()
		want := errors.New("AI 关联调用超时，请稍后重试")
		f.suggester.err = want
		_, err := f.processor.Process(context.Background(), f.task)
		var terminal *AiTaskTerminalError
		require.False(t, errors.As(err, &terminal))
		require.ErrorIs(t, err, want)
	})
}

func TestRelationProcessorKeepsTransientDependencyErrors(t *testing.T) {
	t.Run("embed", func(t *testing.T) {
		f := newRelationTaskFixture()
		want := errors.New("Embedding 调用失败，请检查 Mastra、模型地址、凭据及额度")
		f.embedder.err = want
		_, err := f.processor.Process(context.Background(), f.task)
		var terminal *AiTaskTerminalError
		require.False(t, errors.As(err, &terminal))
		require.ErrorIs(t, err, want)
	})
	t.Run("qdrant", func(t *testing.T) {
		f := newRelationTaskFixture()
		want := errors.New("向量检索服务不可用")
		f.searcher.err = want
		_, err := f.processor.Process(context.Background(), f.task)
		var terminal *AiTaskTerminalError
		require.False(t, errors.As(err, &terminal))
		require.ErrorIs(t, err, want)
	})
	t.Run("persist", func(t *testing.T) {
		f := newRelationTaskFixture()
		want := errors.New("db down")
		f.pairs.createErr = want
		_, err := f.processor.Process(context.Background(), f.task)
		require.ErrorIs(t, err, want)
	})
}

// 模型配置不全时处理器必须给出可读中文原因，而不是空指针。
func TestRelationProcessorWithoutDependencies(t *testing.T) {
	_, err := (&RelationProcessor{}).Process(context.Background(), &model.AiTask{ID: 1, UserID: 2, TargetID: 12})
	terminal := requireTerminalError(t, err)
	require.Contains(t, terminal.Message, "AI 关联未配置")
}
