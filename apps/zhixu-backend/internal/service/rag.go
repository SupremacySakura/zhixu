package service

import (
	"context"
	"errors"
	"sort"
	"strings"
	"unicode/utf8"

	"zhixu-backend/internal/model"
)

const (
	RagDefaultTopK   = 5
	RagMaxTopK       = 10
	RagMaxCandidates = 320
	// RagMaxChunkChars caps each returned passage so a single result cannot flood the model context.
	RagMaxChunkChars = 1024
)

type RagChunkStore interface {
	ChunksByIDs(context.Context, uint, []uint) ([]model.KnowledgeChunk, error)
	Collections(context.Context, uint, uint64) ([]string, error)
	KnowledgeTitles(context.Context, uint, []uint) (map[uint]string, error)
}

type RagSource struct {
	KnowledgeID uint    `json:"knowledge_id"`
	Title       string  `json:"title"`
	Section     string  `json:"section"`
	Content     string  `json:"content"`
	Score       float64 `json:"score"`
}

type RagResult struct {
	Available bool        `json:"available"`
	Sources   []RagSource `json:"sources"`
}

// RagPrepared carries the two capabilities one chat turn may need: the conversation
// model and, when retrieval is enabled and configured, the query embedding model.
type RagPrepared struct {
	Chat      *ChatCapability
	Embedding *ChatCapability
	UserID    uint
	Rag       bool
}

// Prepare resolves the conversation model and, when the caller requested retrieval,
// the embedding model. Retrieval stays optional: without a configured embedding
// model the turn silently falls back to a plain conversation.
func (s *ChatService) Prepare(ctx context.Context, user uint, input ChatInput) (*RagPrepared, error) {
	chat, err := s.Resolve(ctx, user, input)
	if err != nil {
		return nil, err
	}
	prepared := &RagPrepared{Chat: chat, UserID: user}
	if !input.RAG {
		return prepared, nil
	}
	embedding, _, _, err := s.ResolveEmbedding(ctx, user)
	if err != nil {
		return prepared, nil
	}
	prepared.Embedding, prepared.Rag = embedding, true
	return prepared, nil
}

// RagSearcher embeds a query and matches it against the caller's vector collection.
type RagSearcher struct {
	Chat     *ChatService
	Embedder VectorEmbedder
	Vectors  interface {
		Search(context.Context, string, uint, []float64, int) ([]VectorMatch, error)
	}
	Chunks RagChunkStore
}

// Query mirrors the user id carried by the one-time capability; the chat handler
// authorizes it against the authenticated caller before any retrieval happens.
type RagQuery struct {
	UserID uint
	Query  string
	TopK   int
}

func (s *RagSearcher) Search(ctx context.Context, query RagQuery) (*RagResult, error) {
	if query.TopK < 1 || query.TopK > RagMaxTopK {
		return nil, errors.New("检索数量超出允许范围")
	}
	if strings.TrimSpace(query.Query) == "" || utf8.RuneCountInString(query.Query) > 512 {
		return nil, errors.New("检索关键词为空或过长")
	}
	cap, version, _, err := s.Chat.ResolveEmbedding(ctx, query.UserID)
	if err != nil {
		return nil, errors.New("Embedding 配置不可用，请检查模型设置")
	}
	vectors, err := s.Embedder.Embed(ctx, cap, []string{query.Query})
	if err != nil || len(vectors) != 1 {
		return nil, errors.New("查询向量化失败，请检查 Mastra、Embedding 模型地址与凭据")
	}
	if ValidateEmbeddings(vectors, 1, int(cap.Model.Dimension)) != nil {
		return nil, errors.New("查询向量维度无效")
	}
	collections, err := s.Chunks.Collections(ctx, query.UserID, version)
	if err != nil {
		return nil, err
	}
	collections = append(collections, vectorCollection(query.UserID, version))
	unique := map[string]bool{}
	names := []string{}
	for _, name := range collections {
		if !unique[name] {
			unique[name] = true
			names = append(names, name)
		}
	}
	// Increase only unmetered Qdrant candidate requests; the query is embedded once.
	var matches []VectorMatch
	var chunks []model.KnowledgeChunk
	for limit := query.TopK; ; limit = min(limit*2, RagMaxCandidates) {
		best := map[uint]VectorMatch{}
		exhausted := true
		for _, name := range names {
			found, searchErr := s.Vectors.Search(ctx, name, query.UserID, vectors[0], limit)
			if searchErr != nil {
				return nil, searchErr
			}
			if len(found) >= limit {
				exhausted = false
			}
			for _, match := range found {
				old, ok := best[match.ChunkID]
				if !ok || match.Score > old.Score {
					best[match.ChunkID] = match
				}
			}
		}
		matches = matches[:0]
		ids := make([]uint, 0, len(best))
		for id, match := range best {
			ids = append(ids, id)
			matches = append(matches, match)
		}
		sort.Slice(matches, func(i, j int) bool {
			if matches[i].Score == matches[j].Score {
				return matches[i].ChunkID < matches[j].ChunkID
			}
			return matches[i].Score > matches[j].Score
		})
		chunks, err = s.Chunks.ChunksByIDs(ctx, query.UserID, ids)
		if err != nil {
			return nil, err
		}
		live := 0
		for _, chunk := range chunks {
			if chunk.State == "ready" {
				live++
			}
		}
		if live >= query.TopK || exhausted || limit == RagMaxCandidates {
			break
		}
	}
	knowledgeIDs := make([]uint, 0, len(chunks))
	seen := map[uint]bool{}
	for _, chunk := range chunks {
		if !seen[chunk.KnowledgeID] {
			seen[chunk.KnowledgeID] = true
			knowledgeIDs = append(knowledgeIDs, chunk.KnowledgeID)
		}
	}
	titles, err := s.Chunks.KnowledgeTitles(ctx, query.UserID, knowledgeIDs)
	if err != nil {
		return nil, err
	}
	byID := make(map[uint]model.KnowledgeChunk, len(chunks))
	for _, chunk := range chunks {
		byID[chunk.ID] = chunk
	}
	sources := make([]RagSource, 0, len(matches))
	for _, match := range matches {
		chunk, ok := byID[match.ChunkID]
		if !ok || chunk.State != "ready" || titles[chunk.KnowledgeID] == "" {
			continue
		}
		content := chunk.Content
		if utf8.RuneCountInString(content) > RagMaxChunkChars {
			content = string([]rune(content)[:RagMaxChunkChars])
		}
		sources = append(sources, RagSource{
			KnowledgeID: chunk.KnowledgeID, Title: titles[chunk.KnowledgeID],
			Section: chunk.Section, Content: content, Score: match.Score,
		})
		if len(sources) == query.TopK {
			break
		}
	}
	return &RagResult{Available: true, Sources: sources}, nil
}
