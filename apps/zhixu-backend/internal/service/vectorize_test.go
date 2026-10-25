package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type vectorStoreFake struct {
	task        *model.VectorizeTask
	chunks      []model.KnowledgeChunk
	saved       []uint
	savedErr    string
	finished    string
	prepareName string
	waiting     string
	liveErr     error
	saveErr     error
}

func (f *vectorStoreFake) Backfill(context.Context) error { return nil }

func (f *vectorStoreFake) Claim(context.Context, string, time.Time) (*model.VectorizeTask, error) {
	if f.task == nil {
		return nil, gorm.ErrRecordNotFound
	}
	task := *f.task
	return &task, nil
}

func (f *vectorStoreFake) Prepare(_ context.Context, task *model.VectorizeTask, chunks []model.KnowledgeChunk, name string) error {
	f.prepareName = name
	for i := range chunks {
		chunks[i].ID = uint(i + 1)
		chunks[i].UserID, chunks[i].KnowledgeID = task.UserID, task.KnowledgeID
	}
	if len(f.chunks) == 0 {
		f.chunks = append([]model.KnowledgeChunk(nil), chunks...)
	}
	return nil
}

func (f *vectorStoreFake) Wait(_ context.Context, _ *model.VectorizeTask, message string) error {
	f.waiting = message
	return nil
}
func (f *vectorStoreFake) WithLiveTask(_ context.Context, _ *model.VectorizeTask, action func() error) error {
	if f.liveErr != nil {
		return f.liveErr
	}
	return action()
}

func (f *vectorStoreFake) Renew(context.Context, *model.VectorizeTask) error { return nil }

func (f *vectorStoreFake) Chunks(context.Context, uint, uint) ([]model.KnowledgeChunk, error) {
	return append([]model.KnowledgeChunk(nil), f.chunks...), nil
}

func (f *vectorStoreFake) SaveBatch(_ context.Context, _ *model.VectorizeTask, ids []uint, message string) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.savedErr = message
	if message == "" {
		f.saved = append(f.saved, ids...)
	}
	for i := range f.chunks {
		for _, id := range ids {
			if f.chunks[i].ID == id {
				f.chunks[i].State = "ready"
				if message != "" {
					f.chunks[i].State = "failed"
				}
			}
		}
	}
	return nil
}

func (f *vectorStoreFake) Finish(_ context.Context, _ *model.VectorizeTask, message string) error {
	f.finished = message
	return nil
}

type vectorKnowledgeFake struct {
	KnowledgeStore
	item *model.KnowledgeItem
}

func (f *vectorKnowledgeFake) Get(context.Context, uint, uint) (*model.KnowledgeItem, error) {
	return f.item, nil
}

type vectorEmbedderFake struct {
	texts []string
	err   error
}

func (f *vectorEmbedderFake) Embed(_ context.Context, _ *ChatCapability, texts []string) ([][]float64, error) {
	f.texts = append(f.texts, texts...)
	if f.err != nil {
		return nil, f.err
	}
	vectors := make([][]float64, len(texts))
	for i := range vectors {
		vectors[i] = []float64{1, float64(i)}
	}
	return vectors, nil
}

type vectorDatabaseFake struct {
	collection string
	dimension  uint
	points     []VectorPoint
	err        error
}

func (f *vectorDatabaseFake) EnsureCollection(_ context.Context, collection string, dimension uint) error {
	f.collection, f.dimension = collection, dimension
	return f.err
}

func (f *vectorDatabaseFake) Upsert(_ context.Context, _ string, points []VectorPoint) error {
	f.points = append(f.points, points...)
	return f.err
}

type embeddingConfigFake struct{ state *repository.AIConfigState }

func (f *embeddingConfigFake) Read(context.Context, uint) (*repository.AIConfigState, error) {
	return f.state, nil
}

func (f *embeddingConfigFake) Mutate(context.Context, uint, func(*repository.AIConfigState) error) (*repository.AIConfigState, error) {
	return f.state, nil
}

func newVectorizeFixture(err error) (*VectorizeService, *vectorStoreFake, *vectorEmbedderFake, *vectorDatabaseFake) {
	store := &vectorStoreFake{task: &model.VectorizeTask{ID: 1, UserID: 7, KnowledgeID: 9, Status: model.VectorRunning}}
	embedder := &vectorEmbedderFake{err: err}
	vectors := &vectorDatabaseFake{}
	service := &VectorizeService{
		Store:     store,
		Knowledge: &vectorKnowledgeFake{item: &model.KnowledgeItem{ID: 9, UserID: 7, Content: "# 标题\n" + strings.Repeat("知序", 300)}},
		Chat: &ChatService{Store: &embeddingConfigFake{state: &repository.AIConfigState{
			Providers: []model.ModelProvider{{ID: 3, ProviderID: "openai", BaseURL: "https://api.example.com/v1", AuthType: "none"}},
			Models:    []model.ProviderModel{{ID: 4, ProviderID: 3, ModelID: "text-embedding-3-small", Kind: "embedding", Dimension: 2}},
			Settings:  model.UserAISettings{EmbeddingModelID: 4, EmbeddingVersion: 3, ConfigVersion: 5},
		}}},
		Embedder: embedder,
		Vectors:  vectors,
	}
	return service, store, embedder, vectors
}

func TestProcessNextWritesVectorsAndFinishes(t *testing.T) {
	service, store, embedder, vectors := newVectorizeFixture(nil)
	processed, err := service.ProcessNext(context.Background())
	require.NoError(t, err)
	require.True(t, processed)
	require.NotEmpty(t, embedder.texts)
	require.Equal(t, "text-embedding-3-small", store.prepareName)
	require.Equal(t, "zhixu_u7_v3", vectors.collection)
	require.Equal(t, uint(2), vectors.dimension)
	require.Len(t, vectors.points, len(embedder.texts))
	require.Len(t, store.saved, len(embedder.texts))
	require.Empty(t, store.savedErr)
	require.Empty(t, store.finished)
	for _, chunk := range store.chunks {
		require.Equal(t, "ready", chunk.State)
	}
}

func TestProcessNextStopsPaidCallsAfterEmbeddingFailure(t *testing.T) {
	service, store, embedder, vectors := newVectorizeFixture(errors.New("upstream unavailable"))
	processed, err := service.ProcessNext(context.Background())
	require.NoError(t, err)
	require.True(t, processed)
	require.NotEmpty(t, embedder.texts)
	require.Empty(t, vectors.points)
	require.Equal(t, "upstream unavailable", store.savedErr)
	require.Equal(t, "upstream unavailable", store.finished)
	for _, chunk := range store.chunks {
		require.Equal(t, "failed", chunk.State)
	}
}

func TestProcessNextDiscardsTaskWhenEmbeddingConfigChanged(t *testing.T) {
	service, store, embedder, _ := newVectorizeFixture(nil)
	store.task.Collection = "zhixu_u7_v1_c1"
	store.task.ModelID = 99
	processed, err := service.ProcessNext(context.Background())
	require.NoError(t, err)
	require.True(t, processed)
	require.Empty(t, embedder.texts)
	require.Contains(t, store.finished, "Embedding 配置已变化")
}

func TestProcessNextWaitsWithoutPaidCallsThenResumes(t *testing.T) {
	service, store, embedder, vectors := newVectorizeFixture(nil)
	config := service.Chat.Store.(*embeddingConfigFake)
	config.state.Settings.EmbeddingModelID = 0
	_, err := service.ProcessNext(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, store.waiting)
	require.Empty(t, store.chunks)
	require.Empty(t, store.finished)
	require.Empty(t, embedder.texts)
	require.Empty(t, vectors.points)
	config.state.Settings.EmbeddingModelID = 4
	_, err = service.ProcessNext(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, vectors.points)
}

func TestProcessNextKeepsLegacyCollectionAfterChatOnlySave(t *testing.T) {
	service, store, _, vectors := newVectorizeFixture(nil)
	store.task.Collection, store.task.ModelID = "zhixu_u7_v3_c1", 4
	store.task.EmbeddingVersion, store.task.ConfigVersion = 3, 1
	_, err := service.ProcessNext(context.Background())
	require.NoError(t, err)
	require.Equal(t, "zhixu_u7_v3_c1", vectors.collection)
	require.Empty(t, store.finished)
}

func TestProcessNextDeletedTaskCannotUpsert(t *testing.T) {
	service, store, _, vectors := newVectorizeFixture(nil)
	store.liveErr = repository.ErrKnowledgeConflict
	_, err := service.ProcessNext(context.Background())
	require.NoError(t, err)
	require.Empty(t, vectors.points)
	require.Contains(t, store.finished, "knowledge state conflict")
}

func TestProcessNextAmbiguousBatchNotChargedAgain(t *testing.T) {
	service, store, embedder, vectors := newVectorizeFixture(nil)
	// Durable marker left by a worker crashing after the provider accepted a call.
	store.chunks = []model.KnowledgeChunk{{ID: 1, State: "failed", ErrorMessage: "调用结果未确认，为避免重复计费不自动重试"}, {ID: 2, State: "pending"}}
	_, err := service.ProcessNext(context.Background())
	require.NoError(t, err)
	require.Empty(t, embedder.texts)
	require.Empty(t, vectors.points)
	require.NotEmpty(t, store.finished)
}

func TestProcessNextMarkerFailurePreventsPaidCall(t *testing.T) {
	service, store, embedder, vectors := newVectorizeFixture(nil)
	store.saveErr = errors.New("storage unavailable")
	_, err := service.ProcessNext(context.Background())
	require.ErrorContains(t, err, "storage unavailable")
	require.Empty(t, embedder.texts)
	require.Empty(t, vectors.points)
}

func TestProcessNextIdleQueue(t *testing.T) {
	service, store, _, _ := newVectorizeFixture(nil)
	store.task = nil
	processed, err := service.ProcessNext(context.Background())
	require.NoError(t, err)
	require.False(t, processed)
}
