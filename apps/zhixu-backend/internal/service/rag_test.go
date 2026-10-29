package service

import (
	"context"
	"errors"
	"testing"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"

	"github.com/stretchr/testify/require"
)

type ragChunkStoreFake struct {
	chunks      []model.KnowledgeChunk
	titles      map[uint]string
	err         error
	collections []string
}

func (f *ragChunkStoreFake) Collections(context.Context, uint, uint64) ([]string, error) {
	return f.collections, f.err
}

func (f *ragChunkStoreFake) ChunksByIDs(_ context.Context, _ uint, ids []uint) ([]model.KnowledgeChunk, error) {
	if f.err != nil {
		return nil, f.err
	}
	chunks := []model.KnowledgeChunk{}
	for _, chunk := range f.chunks {
		for _, id := range ids {
			if chunk.ID == id {
				chunks = append(chunks, chunk)
				break
			}
		}
	}
	return chunks, nil
}

func (f *ragChunkStoreFake) KnowledgeTitles(_ context.Context, _ uint, _ []uint) (map[uint]string, error) {
	return f.titles, nil
}

type ragVectorFake struct {
	matches      []VectorMatch
	byCollection map[string][]VectorMatch
	calls        int
	err          error
	last         struct {
		collection string
		user       uint
		topK       int
		minScore   float64
		dimension  int
	}
}

// The fake deliberately ignores minScore, like a legacy collection without a
// threshold index would, so the Go-side backstop filter is what tests exercise.
func (f *ragVectorFake) Search(_ context.Context, collection string, user uint, vector []float64, topK int, minScore float64) ([]VectorMatch, error) {
	f.last.collection, f.last.user, f.last.topK, f.last.minScore, f.last.dimension = collection, user, topK, minScore, len(vector)
	f.calls++
	matches := f.matches
	if f.byCollection != nil {
		matches = f.byCollection[collection]
	}
	return matches[:min(topK, len(matches))], f.err
}

func newRagFixture(chunks []model.KnowledgeChunk) (*RagSearcher, *ragVectorFake) {
	vectors := &ragVectorFake{}
	return &RagSearcher{
		Chat: &ChatService{Store: &embeddingConfigFake{state: &repository.AIConfigState{
			Providers: []model.ModelProvider{{ID: 3, ProviderID: "openai", BaseURL: "https://api.example.com/v1", AuthType: "none"}},
			Models:    []model.ProviderModel{{ID: 4, ProviderID: 3, ModelID: "text-embedding-3-small", Kind: "embedding", Dimension: 2}},
			Settings: model.UserAISettings{
				EmbeddingModelID: 4, EmbeddingVersion: 3, ConfigVersion: 5,
				RagTopK: RagDefaultTopK, RagMinScore: RagDefaultMinScore,
			},
		}}},
		Embedder: &vectorEmbedderFake{},
		Vectors:  vectors,
		Chunks: &ragChunkStoreFake{
			chunks: chunks,
			titles: map[uint]string{9: "发布流程"},
		},
	}, vectors
}

func TestRagSearchReturnsRankedSources(t *testing.T) {
	searcher, vectors := newRagFixture([]model.KnowledgeChunk{
		{ID: 11, KnowledgeID: 9, UserID: 7, Section: "部署 / 回滚", Content: "回滚步骤与注意事项", State: "ready"},
		{ID: 12, KnowledgeID: 9, UserID: 7, Section: "部署", Content: "发布前的检查清单", State: "ready"},
		{ID: 13, KnowledgeID: 9, UserID: 7, Section: "旧任务", Content: "仍在等待向量化", State: "pending"},
	})
	vectors.matches = []VectorMatch{{ChunkID: 12, Score: 0.91}, {ChunkID: 11, Score: 0.83}, {ChunkID: 13, Score: 0.4}, {ChunkID: 99, Score: 0.1}}
	result, err := searcher.Search(context.Background(), RagQuery{UserID: 7, Query: "回滚步骤"})
	require.NoError(t, err)
	require.True(t, result.Available)
	require.Equal(t, "zhixu_u7_v3", vectors.last.collection)
	require.Equal(t, uint(7), vectors.last.user)
	require.Equal(t, RagDefaultTopK, vectors.last.topK)
	require.Equal(t, RagDefaultMinScore, vectors.last.minScore)
	require.Equal(t, 2, vectors.last.dimension)
	require.Len(t, result.Sources, 2)
	require.Equal(t, "部署", result.Sources[0].Section)
	require.Equal(t, "发布流程", result.Sources[0].Title)
	require.InDelta(t, 0.91, result.Sources[0].Score, 1e-9)
	require.Equal(t, uint(9), result.Sources[0].KnowledgeID)
	require.Equal(t, "回滚步骤与注意事项", result.Sources[1].Content)
}

func TestRagSearchRefillsDeletedHitsAcrossLegacyCollections(t *testing.T) {
	searcher, vectors := newRagFixture([]model.KnowledgeChunk{
		{ID: 11, KnowledgeID: 9, State: "ready", Content: "live one"},
		{ID: 12, KnowledgeID: 9, State: "ready", Content: "live two"},
	})
	searcher.Chunks.(*ragChunkStoreFake).collections = []string{"zhixu_u7_v3_c1", "zhixu_u7_v3_c2"}
	searcher.Chat.Store.(*embeddingConfigFake).state.Settings.RagTopK = 2
	vectors.byCollection = map[string][]VectorMatch{
		"zhixu_u7_v3_c1": {{ChunkID: 99, Score: 1}, {ChunkID: 98, Score: .99}, {ChunkID: 11, Score: .9}},
		"zhixu_u7_v3_c2": {{ChunkID: 99, Score: 1}, {ChunkID: 11, Score: .8}, {ChunkID: 12, Score: .7}},
	}
	result, err := searcher.Search(context.Background(), RagQuery{UserID: 7, Query: "live"})
	require.NoError(t, err)
	require.Len(t, result.Sources, 2)
	require.Equal(t, "live one", result.Sources[0].Content)
	require.Equal(t, "live two", result.Sources[1].Content)
	require.Greater(t, vectors.calls, 3)
	require.Len(t, searcher.Embedder.(*vectorEmbedderFake).texts, 1)
}

func TestRagCandidateCapStillReturnsLiveHits(t *testing.T) {
	searcher, vectors := newRagFixture([]model.KnowledgeChunk{{ID: 1, KnowledgeID: 9, State: "ready", Content: "live"}})
	vectors.matches = []VectorMatch{{ChunkID: 1, Score: 1}}
	for id := uint(2); id < 400; id++ {
		vectors.matches = append(vectors.matches, VectorMatch{ChunkID: id, Score: .5})
	}
	result, err := searcher.Search(context.Background(), RagQuery{UserID: 7, Query: "live"})
	require.NoError(t, err)
	require.Len(t, result.Sources, 1)
	require.Equal(t, RagMaxCandidates, vectors.last.topK)
	require.Len(t, searcher.Embedder.(*vectorEmbedderFake).texts, 1)
}

func TestRagSearchDropsMatchesBelowThreshold(t *testing.T) {
	searcher, vectors := newRagFixture([]model.KnowledgeChunk{
		{ID: 11, KnowledgeID: 9, State: "ready", Content: "强相关"},
		{ID: 12, KnowledgeID: 9, State: "ready", Content: "勉强相关"},
		{ID: 13, KnowledgeID: 9, State: "ready", Content: "几乎无关"},
	})
	searcher.Chat.Store.(*embeddingConfigFake).state.Settings.RagMinScore = 0.8
	vectors.matches = []VectorMatch{{ChunkID: 11, Score: 0.9}, {ChunkID: 12, Score: 0.79}, {ChunkID: 13, Score: 0.6}}
	result, err := searcher.Search(context.Background(), RagQuery{UserID: 7, Query: "回滚步骤"})
	require.NoError(t, err)
	require.Equal(t, 0.8, vectors.last.minScore)
	require.Len(t, result.Sources, 1)
	require.Equal(t, "强相关", result.Sources[0].Content)

	vectors.matches = []VectorMatch{{ChunkID: 12, Score: 0.79}, {ChunkID: 13, Score: 0.6}}
	result, err = searcher.Search(context.Background(), RagQuery{UserID: 7, Query: "完全无关的问题"})
	require.NoError(t, err)
	require.True(t, result.Available)
	require.Empty(t, result.Sources)
}

func TestRagSearchOutOfRangeSettingsFallBackToDefaults(t *testing.T) {
	searcher, vectors := newRagFixture([]model.KnowledgeChunk{
		{ID: 11, KnowledgeID: 9, State: "ready", Content: "live"},
	})
	searcher.Chat.Store.(*embeddingConfigFake).state.Settings.RagTopK = RagMaxTopK + 1
	searcher.Chat.Store.(*embeddingConfigFake).state.Settings.RagMinScore = 2
	vectors.matches = []VectorMatch{{ChunkID: 11, Score: 1}}
	result, err := searcher.Search(context.Background(), RagQuery{UserID: 7, Query: "live"})
	require.NoError(t, err)
	require.Equal(t, RagDefaultTopK, vectors.last.topK)
	require.Equal(t, RagDefaultMinScore, vectors.last.minScore)
	require.Len(t, result.Sources, 1)
}

func TestRagSearchRejectsInvalidInput(t *testing.T) {
	searcher, _ := newRagFixture(nil)
	_, err := searcher.Search(context.Background(), RagQuery{UserID: 7, Query: "  "})
	require.Error(t, err)
}

func TestRagSearchPropagatesVectorFailure(t *testing.T) {
	searcher, vectors := newRagFixture(nil)
	vectors.err = errors.New("qdrant down")
	_, err := searcher.Search(context.Background(), RagQuery{UserID: 7, Query: "回滚"})
	require.Error(t, err)
}

func TestRagSearchEmptyResultIsAvailable(t *testing.T) {
	searcher, _ := newRagFixture(nil)
	result, err := searcher.Search(context.Background(), RagQuery{UserID: 7, Query: "没有命中"})
	require.NoError(t, err)
	require.True(t, result.Available)
	require.Empty(t, result.Sources)
}

func TestChatPrepareResolvesEmbeddingOnlyWhenRequested(t *testing.T) {
	chat := &ChatService{Store: &embeddingConfigFake{state: &repository.AIConfigState{
		Providers: []model.ModelProvider{{ID: 3, ProviderID: "openai", BaseURL: "https://api.example.com/v1", AuthType: "none"}},
		Models: []model.ProviderModel{
			{ID: 2, ProviderID: 3, ModelID: "chat-model", Kind: "chat"},
			{ID: 4, ProviderID: 3, ModelID: "text-embedding-3-small", Kind: "embedding", Dimension: 2},
		},
		Settings: model.UserAISettings{ChatModelID: 2, EmbeddingModelID: 4},
	}}}
	input := ChatInput{Messages: []ChatMessage{{Role: "user", Content: "你好"}}}
	prepared, err := chat.Prepare(context.Background(), 7, input)
	require.NoError(t, err)
	require.False(t, prepared.Rag)
	require.Nil(t, prepared.Embedding)
	input.RAG = true
	prepared, err = chat.Prepare(context.Background(), 7, input)
	require.NoError(t, err)
	require.True(t, prepared.Rag)
	require.Equal(t, "text-embedding-3-small", prepared.Embedding.Model.ModelID)
}

func TestChatPrepareFallsBackWithoutEmbedding(t *testing.T) {
	chat := &ChatService{Store: &embeddingConfigFake{state: &repository.AIConfigState{
		Providers: []model.ModelProvider{{ID: 3, ProviderID: "openai", BaseURL: "https://api.example.com/v1", AuthType: "none"}},
		Models:    []model.ProviderModel{{ID: 2, ProviderID: 3, ModelID: "chat-model", Kind: "chat"}},
		Settings:  model.UserAISettings{ChatModelID: 2},
	}}}
	prepared, err := chat.Prepare(context.Background(), 7, ChatInput{RAG: true, Messages: []ChatMessage{{Role: "user", Content: "你好"}}})
	require.NoError(t, err)
	require.False(t, prepared.Rag)
	require.Nil(t, prepared.Embedding)
}
