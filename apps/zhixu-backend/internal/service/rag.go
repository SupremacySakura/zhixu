package service

import (
	"context"
	"errors"
	"sort"
	"strings"
	"unicode/utf8"

	"zhixu-backend/internal/log"
	"zhixu-backend/internal/model"
)

const (
	RagDefaultTopK = 5
	RagMaxTopK     = 50
	// RagDefaultMinScore filters out low-similarity matches so off-topic questions
	// count as "nothing found" instead of forcing weak chunks into the prompt.
	RagDefaultMinScore = 0.5
	// RagMinScoreFloor keeps the threshold meaningful; lower values would re-admit
	// the noise the threshold exists to remove.
	RagMinScoreFloor = 0.05
	RagMaxCandidates = 320
	// RagMaxChunkChars caps each returned passage so a single result cannot flood the model context.
	RagMaxChunkChars = 1024
	// RagRRFK damps rank differences in reciprocal rank fusion. 60 is the value
	// from the original RRF paper and keeps a single strong leg from dominating.
	RagRRFK = 60
	// RagKeywordCandidates caps the keyword leg before fusion. Keyword ranking is
	// cheap but noisy at long tails, and 50 candidates comfortably covers topK.
	RagKeywordCandidates = 50
)

// Retrieval modes. Empty mode means hybrid, the default used by the chat tool.
const (
	RagModeHybrid   = "hybrid"
	RagModeSemantic = "semantic"
	RagModeKeyword  = "keyword"
	// RagSourceBoth marks a source that surfaced through both legs.
	RagSourceBoth = "both"
)

// ValidRagMode reports whether a caller-supplied retrieval mode is supported.
func ValidRagMode(mode string) bool {
	switch mode {
	case "", RagModeHybrid, RagModeSemantic, RagModeKeyword:
		return true
	}
	return false
}

type RagChunkStore interface {
	ChunksByIDs(context.Context, uint, []uint) ([]model.KnowledgeChunk, error)
	// SearchChunksKeyword is the keyword leg of hybrid retrieval: ngram full-text
	// matches with the same user and readiness isolation as the chunk hydration.
	SearchChunksKeyword(context.Context, uint, string, int) ([]model.KnowledgeChunk, error)
	Collections(context.Context, uint, uint64) ([]string, error)
	KnowledgeTitles(context.Context, uint, []uint) (map[uint]string, error)
}

type RagSource struct {
	KnowledgeID uint `json:"knowledge_id"`
	// ChunkID and Seq let the UI link a citation straight to the matching chunk
	// preview (readable chunk number) instead of only the document.
	ChunkID uint    `json:"chunk_id"`
	Seq     int     `json:"seq"`
	Title   string  `json:"title"`
	Section string  `json:"section"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
	// Source records which leg produced the passage: keyword, semantic or both.
	Source string `json:"source"`
}

type RagResult struct {
	Available bool        `json:"available"`
	Sources   []RagSource `json:"sources"`
}

// RagSearcher embeds a query and matches it against the caller's vector collection.
type RagSearcher struct {
	Chat     *ChatService
	Embedder VectorEmbedder
	Vectors  interface {
		Search(context.Context, string, uint, []float64, int, float64) ([]VectorMatch, error)
	}
	Chunks RagChunkStore
}

// Query mirrors the user id carried by the one-time capability; the chat handler
// authorizes it against the authenticated caller before any retrieval happens.
// Retrieval breadth and the similarity threshold come from the user's saved
// settings, never from the request body. Mode only selects which legs run and is
// never used to widen the result set.
type RagQuery struct {
	UserID uint
	Query  string
	Mode   string
}

// RagSettings resolves the effective retrieval breadth and similarity threshold,
// falling back to defaults for rows predating the columns or values out of range.
func (s *ChatService) RagSettings(ctx context.Context, user uint) (int, float64, error) {
	state, err := s.Store.Read(ctx, user)
	if err != nil {
		return 0, 0, err
	}
	topK := state.Settings.RagTopK
	if topK < 1 || topK > RagMaxTopK {
		topK = RagDefaultTopK
	}
	minScore := state.Settings.RagMinScore
	if minScore < RagMinScoreFloor || minScore > 1 {
		minScore = RagDefaultMinScore
	}
	return topK, minScore, nil
}

// Search dispatches on the retrieval mode: hybrid (default) fuses the keyword and
// semantic legs with reciprocal rank fusion, while single-leg modes exist for the
// three-mode comparison experiment.
func (s *RagSearcher) Search(ctx context.Context, query RagQuery) (*RagResult, error) {
	if strings.TrimSpace(query.Query) == "" || utf8.RuneCountInString(query.Query) > 512 {
		return nil, errors.New("检索关键词为空或过长")
	}
	if !ValidRagMode(query.Mode) {
		return nil, errors.New("检索模式无效")
	}
	topK, minScore, err := s.Chat.RagSettings(ctx, query.UserID)
	if err != nil {
		return nil, err
	}
	switch query.Mode {
	case RagModeSemantic:
		return s.searchSemantic(ctx, query, topK, minScore)
	case RagModeKeyword:
		return s.searchKeyword(ctx, query, topK)
	default:
		return s.searchHybrid(ctx, query, topK, minScore)
	}
}

// rankedChunk carries one candidate through fusion and hydration: score is the
// semantic similarity when the candidate came through the vector leg, and source
// records the winning leg(s).
type rankedChunk struct {
	chunkID uint
	score   float64
	source  string
}

func (s *RagSearcher) searchSemantic(ctx context.Context, query RagQuery, topK int, minScore float64) (*RagResult, error) {
	matches, err := s.semanticMatches(ctx, query, topK, minScore)
	if err != nil {
		return nil, err
	}
	ranked := make([]rankedChunk, 0, len(matches))
	for _, match := range matches {
		// The vector store already applied score_threshold; this guards legacy
		// collections and test doubles that may return weaker matches.
		if match.Score < minScore {
			continue
		}
		ranked = append(ranked, rankedChunk{chunkID: match.ChunkID, score: match.Score, source: RagModeSemantic})
	}
	return s.hydrate(ctx, query.UserID, ranked, topK)
}

func (s *RagSearcher) searchKeyword(ctx context.Context, query RagQuery, topK int) (*RagResult, error) {
	chunks, err := s.keywordChunks(ctx, query)
	if err != nil {
		return nil, err
	}
	ranked := make([]rankedChunk, 0, len(chunks))
	for _, chunk := range chunks {
		ranked = append(ranked, rankedChunk{chunkID: chunk.ID, source: RagModeKeyword})
	}
	return s.hydrate(ctx, query.UserID, ranked, topK)
}

// searchHybrid runs both legs and fuses their rankings. A leg that fails does not
// fail the request: the other leg still answers, matching the degradation rule in
// the P2 plan. Only losing both legs is an error.
func (s *RagSearcher) searchHybrid(ctx context.Context, query RagQuery, topK int, minScore float64) (*RagResult, error) {
	semantic, semanticErr := s.semanticMatches(ctx, query, topK, minScore)
	keyword, keywordErr := s.keywordChunks(ctx, query)
	if semanticErr != nil && keywordErr != nil {
		return nil, semanticErr
	}
	if semanticErr != nil {
		log.Warn().Err(semanticErr).Msg("语义检索不可用，本次降级为关键词检索")
	}
	if keywordErr != nil {
		log.Warn().Err(keywordErr).Msg("关键词检索不可用，本次降级为语义检索")
	}

	type fused struct {
		chunkID       uint
		semanticScore float64
		semantic      bool
		keyword       bool
		rrf           float64
	}
	byChunk := map[uint]*fused{}
	for rank, match := range semantic {
		if match.Score < minScore {
			continue
		}
		entry := byChunk[match.ChunkID]
		if entry == nil {
			entry = &fused{chunkID: match.ChunkID}
			byChunk[match.ChunkID] = entry
		}
		entry.semantic = true
		entry.semanticScore = match.Score
		entry.rrf += 1 / float64(RagRRFK+rank+1)
	}
	for rank, chunk := range keyword {
		entry := byChunk[chunk.ID]
		if entry == nil {
			entry = &fused{chunkID: chunk.ID}
			byChunk[chunk.ID] = entry
		}
		entry.keyword = true
		entry.rrf += 1 / float64(RagRRFK+rank+1)
	}
	ordered := make([]*fused, 0, len(byChunk))
	for _, entry := range byChunk {
		ordered = append(ordered, entry)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].rrf != ordered[j].rrf {
			return ordered[i].rrf > ordered[j].rrf
		}
		if ordered[i].semanticScore != ordered[j].semanticScore {
			return ordered[i].semanticScore > ordered[j].semanticScore
		}
		return ordered[i].chunkID < ordered[j].chunkID
	})
	ranked := make([]rankedChunk, 0, len(ordered))
	for _, entry := range ordered {
		source := RagModeSemantic
		switch {
		case entry.semantic && entry.keyword:
			source = RagSourceBoth
		case entry.keyword:
			source = RagModeKeyword
		}
		ranked = append(ranked, rankedChunk{chunkID: entry.chunkID, score: entry.semanticScore, source: source})
	}
	return s.hydrate(ctx, query.UserID, ranked, topK)
}

func (s *RagSearcher) keywordChunks(ctx context.Context, query RagQuery) ([]model.KnowledgeChunk, error) {
	return s.Chunks.SearchChunksKeyword(ctx, query.UserID, query.Query, RagKeywordCandidates)
}

// semanticMatches embeds the query once and widens the unmetered Qdrant candidate
// requests until enough live chunks turn up or the candidate cap is reached.
func (s *RagSearcher) semanticMatches(ctx context.Context, query RagQuery, topK int, minScore float64) ([]VectorMatch, error) {
	capability, version, _, err := s.Chat.ResolveEmbedding(ctx, query.UserID)
	if err != nil {
		return nil, errors.New("Embedding 配置不可用，请检查模型设置")
	}
	vectors, err := s.Embedder.Embed(ctx, capability, []string{query.Query})
	if err != nil || len(vectors) != 1 {
		return nil, errors.New("查询向量化失败，请检查 Mastra、Embedding 模型地址与凭据")
	}
	if ValidateEmbeddings(vectors, 1, int(capability.Model.Dimension)) != nil {
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
	for limit := topK; ; limit = min(limit*2, RagMaxCandidates) {
		best := map[uint]VectorMatch{}
		exhausted := true
		for _, name := range names {
			found, searchErr := s.Vectors.Search(ctx, name, query.UserID, vectors[0], limit, minScore)
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
		if live >= topK || exhausted || limit == RagMaxCandidates {
			break
		}
	}
	return matches, nil
}

// hydrate turns fused candidates into sources ordered by their fused ranking. The
// chunk and title lookups keep deleted, not-ready chunks and archived knowledge out
// of the answer, so a stale vector can never leak into the response.
func (s *RagSearcher) hydrate(ctx context.Context, user uint, ranked []rankedChunk, topK int) (*RagResult, error) {
	if len(ranked) == 0 {
		return &RagResult{Available: true, Sources: []RagSource{}}, nil
	}
	ids := make([]uint, 0, len(ranked))
	seen := map[uint]bool{}
	for _, candidate := range ranked {
		if !seen[candidate.chunkID] {
			seen[candidate.chunkID] = true
			ids = append(ids, candidate.chunkID)
		}
	}
	chunks, err := s.Chunks.ChunksByIDs(ctx, user, ids)
	if err != nil {
		return nil, err
	}
	knowledgeIDs := make([]uint, 0, len(chunks))
	seenKnowledge := map[uint]bool{}
	for _, chunk := range chunks {
		if !seenKnowledge[chunk.KnowledgeID] {
			seenKnowledge[chunk.KnowledgeID] = true
			knowledgeIDs = append(knowledgeIDs, chunk.KnowledgeID)
		}
	}
	titles, err := s.Chunks.KnowledgeTitles(ctx, user, knowledgeIDs)
	if err != nil {
		return nil, err
	}
	byID := make(map[uint]model.KnowledgeChunk, len(chunks))
	for _, chunk := range chunks {
		byID[chunk.ID] = chunk
	}
	sources := make([]RagSource, 0, min(topK, len(ranked)))
	for _, candidate := range ranked {
		chunk, ok := byID[candidate.chunkID]
		if !ok || chunk.State != "ready" || titles[chunk.KnowledgeID] == "" {
			continue
		}
		content := chunk.Content
		if utf8.RuneCountInString(content) > RagMaxChunkChars {
			content = string([]rune(content)[:RagMaxChunkChars])
		}
		sources = append(sources, RagSource{
			KnowledgeID: chunk.KnowledgeID, ChunkID: chunk.ID, Seq: chunk.Seq,
			Title:   titles[chunk.KnowledgeID],
			Section: chunk.Section, Content: content, Score: candidate.score, Source: candidate.source,
		})
		if len(sources) == topK {
			break
		}
	}
	return &RagResult{Available: true, Sources: sources}, nil
}
