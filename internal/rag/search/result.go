package search

import (
	"context"

	"github.com/google/uuid"
)

// Result is a single retrieval hit. Score is comparable within the producing
// retriever (BM25 TF-IDF-like, vector 1/(1+distance)); fused scores from
// HybridSearcher are RRF sums.
type Result struct {
	ID         uuid.UUID
	Content    string
	SourcePath string
	Metadata   map[string]any
	Score      float64
}

// Searcher retrieves the top-k chunks for a query. queryVec is the embedded
// query vector; retrievers that ignore it (BM25) accept nil.
type Searcher interface {
	Search(ctx context.Context, query string, queryVec []float32, k int) ([]Result, error)
}
