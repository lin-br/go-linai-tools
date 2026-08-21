package main

import (
	"context"
	"fmt"
	"math"

	"github.com/lin-br/go-linai-tools/internal/core/domain"
	"github.com/lin-br/go-linai-tools/internal/core/ports/outbound"
	"github.com/lin-br/go-linai-tools/internal/rag/chunk"
	"github.com/lin-br/go-linai-tools/internal/rag/search"
	"github.com/lin-br/go-linai-tools/internal/rag/store"
)

// ragSearcher is a search.Searcher that embeds the query when no vector is
// supplied, runs hybrid retrieval, and optionally reranks. It depends on the
// outbound.Embedder and outbound.Reranker ports so any provider (OpenRouter,
// direct Voyage, direct Cohere) can be injected.
type ragSearcher struct {
	emb            outbound.Embedder
	embeddingModel string
	hybrid         *search.HybridSearcher
	rerank         outbound.Reranker
	rerankModel    string
}

var _ search.Searcher = (*ragSearcher)(nil)

func (r *ragSearcher) Search(ctx context.Context, query string, queryVec []float32, k int) ([]search.Result, error) {
	vec := queryVec
	if vec == nil {
		embResp, err := r.emb.Embed(ctx, &domain.EmbeddingRequest{
			Model: r.embeddingModel,
			Input: []string{query},
			//InputType: "search_query",
			// Nvidia embeddings only support "query" and "passage"
			InputType: "query",
		})
		if err != nil {
			return nil, fmt.Errorf("embed query: %w", err)
		}
		if len(embResp.Data) == 0 || len(embResp.Data[0].Embedding) == 0 {
			return nil, nil
		}
		vec = normalize(embResp.Data[0].Embedding)
	}

	cand := k * 4
	if cand < k {
		cand = k
	}
	results, err := r.hybrid.Search(ctx, query, vec, cand)
	if err != nil {
		return nil, err
	}
	if r.rerank != nil && len(results) > 0 {
		docs := make([]domain.RerankDocument, len(results))
		for i, res := range results {
			docs[i] = domain.RerankDocument{Text: res.Content}
		}
		topN := k
		rerankResp, rerr := r.rerank.Rerank(ctx, &domain.RerankRequest{
			Model:     r.rerankModel,
			Query:     query,
			Documents: docs,
			TopN:      topN,
		})
		if rerr != nil {
			return nil, rerr
		}
		out := make([]search.Result, 0, len(rerankResp.Results))
		for _, rr := range rerankResp.Results {
			if rr.Index < 0 || rr.Index >= len(results) {
				return nil, fmt.Errorf("rerank: invalid result index %d (have %d)", rr.Index, len(results))
			}
			orig := results[rr.Index]
			orig.Score = rr.RelevanceScore
			out = append(out, orig)
		}
		return out, nil
	}
	if len(results) > k {
		results = results[:k]
	}
	return results, nil
}

// normalize returns an L2-normalized copy of v. A zero vector is returned as a
// copy of the input so callers cannot mutate the decoded slice. This keeps
// cosine-similarity semantics with the pgvector L2-distance store regardless of
// whether the embedding provider normalizes.
func normalize(v []float32) []float32 {
	out := make([]float32, len(v))
	copy(out, v)
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		return out
	}
	inv := float32(1.0 / math.Sqrt(sum))
	for i, x := range out {
		out[i] = x * inv
	}
	return out
}

// extractEmbeddingVectors flattens an EmbeddingResponse into a [][]float32
// slice matching the input order, for store.InsertChunks compatibility.
func extractEmbeddingVectors(resp *domain.EmbeddingResponse) [][]float32 {
	out := make([][]float32, len(resp.Data))
	for i, d := range resp.Data {
		out[i] = d.Embedding
	}
	return out
}

// toChunks converts store.Chunk rows to chunk.Chunk for BM25 indexing.
func toChunks(rows []store.Chunk) []chunk.Chunk {
	out := make([]chunk.Chunk, len(rows))
	for i, r := range rows {
		out[i] = chunk.Chunk{
			ID:         r.ID,
			Content:    r.Content,
			SourcePath: r.SourcePath,
			Metadata:   r.Metadata,
		}
	}
	return out
}
