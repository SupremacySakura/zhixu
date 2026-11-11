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
	task                          *model.VectorizeTask
	chunks                        []model.KnowledgeChunk
	saved                         []uint
	savedErr                      string
	finished                      string
	prepareName                   string
	waiting                       string
	liveErr                       error
	saveErr                       error
	requeueErr                    error
	requeueCalled                 bool
	requeueUser, requeueKnowledge uint
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

func (f *vectorStoreFake) Requeue(_ context.Context, user, knowledge uint) error {
	f.requeueCalled, f.requeueUser, f.requeueKnowledge = true, user, knowledge
	return f.requeueErr
}

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
	deleted    []uint
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

func (f *vectorDatabaseFake) DeletePoints(_ context.Context, _ string, ids []uint) error {
	f.deleted = append(f.deleted, ids...)
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
		Knowledge: &vectorKnowledgeFake{item: &model.KnowledgeItem{ID: 9, UserID: 7, Version: 5, Content: "# 标题\n" + strings.Repeat("知序", 300)}},
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

func TestProcessNextCleansStaleVectorsFromEdit(t *testing.T) {
	service, store, _, vectors := newVectorizeFixture(nil)
	store.task.StaleChunkIDs = "[11,12]"
	store.task.CleanupCollection = "zhixu_u7_v3"
	processed, err := service.ProcessNext(context.Background())
	require.NoError(t, err)
	require.True(t, processed)
	require.Equal(t, []uint{11, 12}, vectors.deleted)
}

func TestProcessNextCleansStaleBeforeEmbeddingFailure(t *testing.T) {
	service, store, _, vectors := newVectorizeFixture(errors.New("embedding down"))
	store.task.StaleChunkIDs = "[21]"
	store.task.CleanupCollection = "zhixu_u7_v3"
	processed, err := service.ProcessNext(context.Background())
	require.NoError(t, err)
	require.True(t, processed)
	require.Equal(t, []uint{21}, vectors.deleted)
	require.Equal(t, "embedding down", store.savedErr)
}

type relationEnqueuerFake struct {
	calls                     int
	userID, knowledgeID, base uint
	err                       error

	hasCalls           int
	hasExists          bool
	hasErr             error
	hasUser, hasTarget uint
	hasType            string
}

func (f *relationEnqueuerFake) EnqueueRelation(_ context.Context, userID, knowledgeID, baseVersion uint) error {
	f.calls++
	f.userID, f.knowledgeID, f.base = userID, knowledgeID, baseVersion
	return f.err
}

func (f *relationEnqueuerFake) HasTaskForTarget(_ context.Context, userID uint, taskType string, targetID uint) (bool, error) {
	f.hasCalls++
	f.hasUser, f.hasType, f.hasTarget = userID, taskType, targetID
	return f.hasExists, f.hasErr
}

// 向量化成功就绪是关联任务的唯一触发点：带上知识版本，并只入队一次。
func TestProcessNextEnqueuesRelationAfterVectorReady(t *testing.T) {
	service, store, _, _ := newVectorizeFixture(nil)
	relations := &relationEnqueuerFake{}
	service.Relations = relations

	processed, err := service.ProcessNext(context.Background())

	require.NoError(t, err)
	require.True(t, processed)
	require.Empty(t, store.finished)
	require.Equal(t, 1, relations.hasCalls)
	require.Equal(t, uint(7), relations.hasUser)
	require.Equal(t, model.AiTaskTypeRelation, relations.hasType)
	require.Equal(t, uint(9), relations.hasTarget)
	require.Equal(t, 1, relations.calls)
	require.Equal(t, uint(7), relations.userID)
	require.Equal(t, uint(9), relations.knowledgeID)
	require.Equal(t, uint(5), relations.base)
}

// 编辑正文会再次触发向量化，但规格不允许它自动重跑推荐：已有任意状态的关系任务就不再入队。
func TestProcessNextDoesNotEnqueueRelationWhenTaskAlreadyExists(t *testing.T) {
	service, store, _, _ := newVectorizeFixture(nil)
	relations := &relationEnqueuerFake{hasExists: true}
	service.Relations = relations

	processed, err := service.ProcessNext(context.Background())

	require.NoError(t, err)
	require.True(t, processed)
	require.Empty(t, store.finished)
	require.Equal(t, 1, relations.hasCalls)
	require.Zero(t, relations.calls)
}

// 查询失败时保守跳过自动入队，人工“查找相关知识”仍可显式触发。
func TestProcessNextDoesNotEnqueueRelationWhenTaskLookupFails(t *testing.T) {
	service, store, _, _ := newVectorizeFixture(nil)
	relations := &relationEnqueuerFake{hasErr: errors.New("db down")}
	service.Relations = relations

	processed, err := service.ProcessNext(context.Background())

	require.NoError(t, err)
	require.True(t, processed)
	require.Empty(t, store.finished)
	require.Equal(t, 1, relations.hasCalls)
	require.Zero(t, relations.calls)
}

func TestProcessNextDoesNotEnqueueRelationAfterFailure(t *testing.T) {
	service, store, _, _ := newVectorizeFixture(errors.New("embedding down"))
	relations := &relationEnqueuerFake{}
	service.Relations = relations

	processed, err := service.ProcessNext(context.Background())

	require.NoError(t, err)
	require.True(t, processed)
	require.Equal(t, "embedding down", store.finished)
	require.Zero(t, relations.calls)
}

// 入队失败只记日志，不能让已经成功的向量化任务回滚。
func TestProcessNextToleratesRelationEnqueueError(t *testing.T) {
	service, store, _, _ := newVectorizeFixture(nil)
	relations := &relationEnqueuerFake{err: errors.New("queue down")}
	service.Relations = relations

	processed, err := service.ProcessNext(context.Background())

	require.NoError(t, err)
	require.True(t, processed)
	require.Empty(t, store.finished)
	require.Equal(t, 1, relations.calls)
}
