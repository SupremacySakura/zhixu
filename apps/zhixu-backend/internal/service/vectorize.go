package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"zhixu-backend/internal/log"
	"zhixu-backend/internal/model"

	"gorm.io/gorm"
)

type VectorizeStore interface {
	Backfill(context.Context) error
	Claim(context.Context, string, time.Time) (*model.VectorizeTask, error)
	Prepare(context.Context, *model.VectorizeTask, []model.KnowledgeChunk, string) error
	Renew(context.Context, *model.VectorizeTask) error
	Chunks(context.Context, uint, uint) ([]model.KnowledgeChunk, error)
	SaveBatch(context.Context, *model.VectorizeTask, []uint, string) error
	Finish(context.Context, *model.VectorizeTask, string) error
	Wait(context.Context, *model.VectorizeTask, string) error
	WithLiveTask(context.Context, *model.VectorizeTask, func() error) error
}

type VectorEmbedder interface {
	Embed(context.Context, *ChatCapability, []string) ([][]float64, error)
}

type VectorDatabase interface {
	EnsureCollection(context.Context, string, uint) error
	Upsert(context.Context, string, []VectorPoint) error
}

type VectorizeService struct {
	Store     VectorizeStore
	Knowledge KnowledgeStore
	Chat      *ChatService
	Embedder  VectorEmbedder
	Vectors   VectorDatabase
}

func vectorCollection(user uint, version uint64) string {
	return fmt.Sprintf("zhixu_u%d_v%d", user, version)
}

func (s *VectorizeService) ProcessNext(ctx context.Context) (bool, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return false, err
	}
	task, err := s.Store.Claim(ctx, hex.EncodeToString(random), time.Now())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	item, err := s.Knowledge.Get(ctx, task.UserID, task.KnowledgeID)
	if err != nil {
		return true, err
	}
	chunks, err := ChunkMarkdown(item.Content, ChunkSize, ChunkOverlap)
	if err != nil || len(chunks) == 0 {
		return true, s.Store.Finish(ctx, task, "未生成有效分块")
	}
	cap, version, configVersion, resolveErr := s.Chat.ResolveEmbedding(ctx, task.UserID)
	if resolveErr != nil {
		return true, s.Store.Wait(ctx, task, "未配置可用的 Embedding 模型或凭据，任务等待模型设置")
	}
	if task.Collection != "" && (task.ModelID != cap.Model.ID || task.EmbeddingVersion != version) {
		return true, s.Store.Finish(ctx, task, "Embedding 配置已变化，旧任务已停止；请等待后续重建索引")
	}
	task.ModelID, task.EmbeddingVersion, task.ConfigVersion = cap.Model.ID, version, configVersion
	// Keep legacy c-named collections for existing tasks, including partial batches.
	if task.Collection == "" {
		task.Collection = vectorCollection(task.UserID, version)
	}
	name := cap.Model.ModelID
	for i := range chunks {
		chunks[i].UserID, chunks[i].KnowledgeID = task.UserID, task.KnowledgeID
		chunks[i].EmbeddingVersion = task.EmbeddingVersion
	}
	if err := s.Store.Prepare(ctx, task, chunks, name); err != nil {
		return true, err
	}
	// Check Qdrant before any paid invocation, and require acknowledged writes.
	if err := s.Vectors.EnsureCollection(ctx, task.Collection, cap.Model.Dimension); err != nil {
		return true, s.Store.Finish(ctx, task, err.Error())
	}
	chunks, err = s.Store.Chunks(ctx, task.UserID, task.KnowledgeID)
	if err != nil {
		return true, err
	}
	pending := make([]model.KnowledgeChunk, 0)
	failure := ""
	for _, chunk := range chunks {
		if chunk.State == "pending" {
			pending = append(pending, chunk)
		} else if chunk.State == "failed" {
			failure = "部分分块未能完成向量化，请在分块结构中查看原因"
		}
	}
	if failure != "" {
		return true, s.Store.Finish(ctx, task, failure)
	}
	for start := 0; start < len(pending); start += 16 {
		if ctx.Err() != nil {
			return true, ctx.Err()
		}
		if err := s.Store.Renew(ctx, task); err != nil {
			return true, err
		}
		batch := pending[start:min(start+16, len(pending))]
		texts, ids := make([]string, len(batch)), make([]uint, len(batch))
		for i, chunk := range batch {
			texts[i], ids[i] = chunk.Content, chunk.ID
		}
		// Persist before the paid invocation. A crash/timeout must not replay an
		// ambiguous charge when the lease is reclaimed; success overwrites it.
		if err := s.Store.SaveBatch(ctx, task, ids, "调用结果未确认，为避免重复计费不自动重试"); err != nil {
			return true, err
		}
		callCtx, cancel := context.WithTimeout(ctx, 110*time.Second)
		vectors, embedErr := s.Embedder.Embed(callCtx, cap, texts)
		cancel()
		message := ""
		if ctx.Err() != nil {
			return true, ctx.Err()
		}
		if embedErr != nil {
			message = embedErr.Error()
		} else if ValidateEmbeddings(vectors, len(batch), int(cap.Model.Dimension)) != nil {
			message = "Embedding 数量或维度与配置不一致"
		} else {
			if err := s.Store.Renew(ctx, task); err != nil {
				return true, err
			}
			points := make([]VectorPoint, len(batch))
			for i, chunk := range batch {
				points[i] = VectorPoint{ID: chunk.ID, Vector: vectors[i], Payload: map[string]any{
					"user_id": task.UserID, "knowledge_id": task.KnowledgeID, "chunk_id": chunk.ID,
					"seq": chunk.Seq, "embedding_version": task.EmbeddingVersion, "section": chunk.Section,
				}}
			}
			// Serialize the external write with deletion: Renew alone leaves a race.
			writeCtx, cancelWrite := context.WithTimeout(ctx, 20*time.Second)
			err := s.Store.WithLiveTask(writeCtx, task, func() error {
				return s.Vectors.Upsert(writeCtx, task.Collection, points)
			})
			cancelWrite()
			if err != nil {
				message = err.Error()
			}
		}
		if ctx.Err() != nil {
			return true, ctx.Err()
		}
		if err := s.Store.SaveBatch(ctx, task, ids, message); err != nil {
			return true, err
		}
		if message != "" {
			failure = message
			// Stop after an upstream failure instead of repeating paid requests against an unhealthy service.
			break
		}
	}
	return true, s.Store.Finish(ctx, task, failure)
}

func (s *VectorizeService) Run(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		if err := s.Store.Backfill(ctx); err != nil && ctx.Err() == nil {
			log.Error().Err(err).Msg("向量任务补建失败")
		}
		for ctx.Err() == nil {
			processed, err := s.ProcessNext(ctx)
			if err != nil && ctx.Err() == nil {
				log.Error().Err(err).Msg("向量任务处理失败")
			}
			if !processed || err != nil {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
