package search

import (
	"context"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/lin-br/go-linai-tools/internal/rag/chunk"
)

// DefaultRRF is the reciprocal rank fusion constant used when NewHybridSearcher
// is called with kRRF == 0.
const DefaultRRF = 60

// HybridSearcher merges BM25 and vector results via Reciprocal Rank Fusion.
type HybridSearcher struct {
	bm25   *BM25Searcher
	vector *VectorSearcher
	kRRF   int
}

// Compile-time interface check.
var _ Searcher = (*HybridSearcher)(nil)

// NewHybridSearcher wires the two retrievers. kRRF defaults to DefaultRRF (60)
// when zero is passed.
func NewHybridSearcher(bm25 *BM25Searcher, vector *VectorSearcher, kRRF int) *HybridSearcher {
	if kRRF == 0 {
		kRRF = DefaultRRF
	}
	return &HybridSearcher{bm25: bm25, vector: vector, kRRF: kRRF}
}

// KRRF exposes the effective RRF constant, used to assert the default-60 rule.
func (h *HybridSearcher) KRRF() int { return h.kRRF }

// Index forwards chunks to the BM25 searcher so keyword search is fresh.
func (h *HybridSearcher) Index(ctx context.Context, chunks []chunk.Chunk) error {
	return h.bm25.Index(ctx, chunks)
}

// Search fetches k*4 candidates from each retriever, fuses ranks with RRF, and
// returns the top k. Ties in fused score are broken by better (lower) vector
// rank.
func (h *HybridSearcher) Search(ctx context.Context, query string, queryVec []float32, k int) ([]Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cand := k * 4
	if cand < k {
		cand = k
	}

	bmResults, err := h.bm25.Search(ctx, query, queryVec, cand)
	if err != nil {
		return nil, fmt.Errorf("search: hybrid bm25: %w", err)
	}
	vecResults, err := h.vector.Search(ctx, query, queryVec, cand)
	if err != nil {
		return nil, fmt.Errorf("search: hybrid vector: %w", err)
	}

	type entry struct {
		r          Result
		fused      float64
		vectorRank int
	}
	maxRank := len(vecResults) + 1
	byID := make(map[uuid.UUID]*entry, len(bmResults)+len(vecResults))

	for rank, r := range bmResults {
		e := &entry{r: r, vectorRank: maxRank}
		e.fused += 1.0 / float64(rank+1+h.kRRF)
		byID[r.ID] = e
	}
	for rank, r := range vecResults {
		e, ok := byID[r.ID]
		if !ok {
			e = &entry{r: r, vectorRank: rank + 1}
			byID[r.ID] = e
		} else {
			e.vectorRank = rank + 1
		}
		e.fused += 1.0 / float64(rank+1+h.kRRF)
	}

	entries := make([]*entry, 0, len(byID))
	for _, e := range byID {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].fused != entries[j].fused {
			return entries[i].fused > entries[j].fused
		}
		return entries[i].vectorRank < entries[j].vectorRank
	})

	if k > len(entries) {
		k = len(entries)
	}
	out := make([]Result, 0, k)
	for i := 0; i < k; i++ {
		entries[i].r.Score = entries[i].fused
		out = append(out, entries[i].r)
	}
	return out, nil
}
