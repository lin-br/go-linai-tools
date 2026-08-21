package search

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/blevesearch/bleve/v2"
	"github.com/lin-br/go-linai-tools/internal/rag/chunk"
)

// bm25DocField is the indexed text field name used by BM25Searcher.
const bm25DocField = "content"

// BM25Searcher builds an in-memory Bleve index from chunks and serves keyword
// search with BM25 scoring. The index is rebuilt per process; the query vector
// argument is ignored (keyword retrieval only).
type BM25Searcher struct {
	mu   sync.Mutex
	idx  bleve.Index
	docs []chunk.Chunk
}

// Compile-time interface check.
var _ Searcher = (*BM25Searcher)(nil)

// NewBM25Searcher returns an empty BM25 searcher with no index yet.
func NewBM25Searcher() *BM25Searcher {
	return &BM25Searcher{}
}

// Index (re)builds the in-memory Bleve index from chunks, replacing any prior
// index. Subsequent Search calls query the new index.
func (b *BM25Searcher) Index(ctx context.Context, chunks []chunk.Chunk) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	mapping := bleve.NewIndexMapping()
	idx, err := bleve.NewMemOnly(mapping)
	if err != nil {
		return fmt.Errorf("search: bm25 build index: %w", err)
	}
	for i, c := range chunks {
		doc := map[string]string{bm25DocField: c.Content}
		if err := idx.Index(strconv.Itoa(i), doc); err != nil {
			_ = idx.Close()
			return fmt.Errorf("search: bm25 index doc %d: %w", i, err)
		}
	}
	b.mu.Lock()
	if b.idx != nil {
		_ = b.idx.Close()
	}
	b.idx = idx
	b.docs = chunks
	b.mu.Unlock()
	return nil
}

// Search runs a Bleve match query and returns the top k results ordered by
// BM25 score. Returns an empty slice when no index has been built.
func (b *BM25Searcher) Search(ctx context.Context, query string, _ []float32, k int) ([]Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b.mu.Lock()
	idx := b.idx
	docs := b.docs
	b.mu.Unlock()
	if idx == nil {
		return nil, nil
	}
	if k <= 0 {
		return nil, nil
	}

	q := bleve.NewMatchQuery(query)
	req := bleve.NewSearchRequest(q)
	req.Size = k

	result, err := idx.Search(req)
	if err != nil {
		return nil, fmt.Errorf("search: bm25 query: %w", err)
	}

	out := make([]Result, 0, len(result.Hits))
	for _, hit := range result.Hits {
		i, perr := strconv.Atoi(hit.ID)
		if perr != nil || i < 0 || i >= len(docs) {
			continue
		}
		c := docs[i]
		out = append(out, Result{
			ID:         c.ID,
			Content:    c.Content,
			SourcePath: c.SourcePath,
			Metadata:   c.Metadata,
			Score:      hit.Score,
		})
	}
	return out, nil
}
