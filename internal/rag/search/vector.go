package search

import (
	"context"
	"fmt"

	"github.com/lin-br/go-linai-tools/internal/rag/store"
)

// VectorReader is the subset of *store.Store that VectorSearcher needs. Defined
// as an interface so unit tests can inject a fake without a live Postgres; the
// concrete *store.Store satisfies it.
type VectorReader interface {
	Search(ctx context.Context, queryVec []float32, k int) ([]store.SearchResult, error)
}

// VectorSearcher delegates to a VectorReader and converts store.SearchResult
// into search.Result, mapping distance to Score = 1/(1+distance) (higher is
// better). The query string is ignored.
type VectorSearcher struct {
	store VectorReader
}

// Compile-time interface check.
var _ Searcher = (*VectorSearcher)(nil)

// NewVectorSearcher wraps a VectorReader. Accepts *store.Store in production
// (it satisfies VectorReader) or a fake in tests.
func NewVectorSearcher(s VectorReader) *VectorSearcher {
	return &VectorSearcher{store: s}
}

// Search returns the k nearest neighbors as search.Result values.
func (v *VectorSearcher) Search(ctx context.Context, _ string, queryVec []float32, k int) ([]Result, error) {
	if queryVec == nil {
		return nil, fmt.Errorf("search: vector searcher requires a query vector")
	}
	sr, err := v.store.Search(ctx, queryVec, k)
	if err != nil {
		return nil, fmt.Errorf("search: vector: %w", err)
	}
	out := make([]Result, len(sr))
	for i, r := range sr {
		out[i] = Result{
			ID:         r.ID,
			Content:    r.Content,
			SourcePath: r.SourcePath,
			Metadata:   r.Metadata,
			Score:      1.0 / (1.0 + r.Distance),
		}
	}
	return out, nil
}
