package search

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/lin-br/go-linai-tools/internal/rag/chunk"
	"github.com/lin-br/go-linai-tools/internal/rag/store"
)

// fakeVectorReader returns preset store.SearchResults for VectorSearcher tests.
type fakeVectorReader struct {
	results []store.SearchResult
	err     error
}

func (f *fakeVectorReader) Search(_ context.Context, _ []float32, k int) ([]store.SearchResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := f.results
	if k < len(out) {
		out = out[:k]
	}
	return out, nil
}

func TestResultCarriesRequiredFields(t *testing.T) {
	id := uuid.New()
	r := Result{
		ID:         id,
		Content:    "c",
		SourcePath: "p.txt",
		Metadata:   map[string]any{"index": 0},
		Score:      0.5,
	}
	if r.ID != id || r.Content != "c" || r.SourcePath != "p.txt" || r.Score != 0.5 {
		t.Errorf("Result fields not preserved: %+v", r)
	}
	if r.Metadata["index"] != 0 {
		t.Errorf("metadata not preserved: %v", r.Metadata)
	}
}

func TestVectorSearcher_ConvertsScoreAndPreservesFields(t *testing.T) {
	id := uuid.New()
	sr := store.SearchResult{
		ID:         id,
		Content:    "hello",
		SourcePath: "h.txt",
		Metadata:   map[string]any{"index": 1},
		Distance:   0.5,
	}
	vs := NewVectorSearcher(&fakeVectorReader{results: []store.SearchResult{sr}})

	got, err := vs.Search(context.Background(), "ignored", []float32{1, 0}, 5)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	want := 1.0 / 1.5
	if got[0].Score < want-1e-9 || got[0].Score > want+1e-9 {
		t.Errorf("Score = %v, want %v", got[0].Score, want)
	}
	if got[0].Content != "hello" || got[0].SourcePath != "h.txt" {
		t.Errorf("content/source not preserved: %+v", got[0])
	}
	if got[0].Metadata["index"] != 1 {
		t.Errorf("metadata not preserved: %v", got[0].Metadata)
	}
}

func TestVectorSearcher_RequiresQueryVector(t *testing.T) {
	vs := NewVectorSearcher(&fakeVectorReader{})
	if _, err := vs.Search(context.Background(), "q", nil, 5); err == nil {
		t.Fatal("expected error for nil query vector, got nil")
	}
}

func TestBM25_EmptyIndexReturnsEmpty(t *testing.T) {
	b := NewBM25Searcher()
	got, err := b.Search(context.Background(), "anything", nil, 5)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestBM25_IndexAndSearchRoundTrip(t *testing.T) {
	b := NewBM25Searcher()
	budget := chunk.Chunk{ID: uuid.New(), Content: "budget budget budget plan", SourcePath: "a.txt", Metadata: map[string]any{"index": 0}}
	other := chunk.Chunk{ID: uuid.New(), Content: "weather climate report", SourcePath: "b.txt", Metadata: map[string]any{"index": 1}}
	if err := b.Index(context.Background(), []chunk.Chunk{budget, other}); err != nil {
		t.Fatalf("Index error: %v", err)
	}
	got, err := b.Search(context.Background(), "budget", nil, 2)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one hit for 'budget'")
	}
	if got[0].ID != budget.ID {
		t.Errorf("top hit ID = %v, want budget %v", got[0].ID, budget.ID)
	}
	if got[0].Metadata["index"] != 0 {
		t.Errorf("metadata not preserved: %v", got[0].Metadata)
	}
	for _, r := range got {
		if r.ID == other.ID {
			t.Error("non-matching doc returned for 'budget'")
		}
	}
}

func TestHybrid_DefaultRRFIs60(t *testing.T) {
	h := NewHybridSearcher(NewBM25Searcher(), NewVectorSearcher(&fakeVectorReader{}), 0)
	if h.KRRF() != 60 {
		t.Errorf("KRRF = %d, want 60", h.KRRF())
	}
	if DefaultRRF != 60 {
		t.Errorf("DefaultRRF = %d, want 60", DefaultRRF)
	}
}

// TestHybrid_FusionAndTieBreak: BM25 returns {budget, other-budget} (both match
// "budget"); vector returns them in the opposite order plus a third single-list
// doc. Docs in both lists must outrank the single-list doc, and equal fused
// scores break by better (lower) vector rank.
func TestHybrid_FusionAndTieBreak(t *testing.T) {
	idA := uuid.New()
	idB := uuid.New()
	idC := uuid.New()

	bm := NewBM25Searcher()
	chunks := []chunk.Chunk{
		{ID: idA, Content: "budget budget budget", SourcePath: "a"},
		{ID: idB, Content: "budget plan", SourcePath: "b"},
		{ID: idC, Content: "weather climate", SourcePath: "c"},
	}
	if err := bm.Index(context.Background(), chunks); err != nil {
		t.Fatalf("Index error: %v", err)
	}

	// Vector returns [B, A, C] (B closest, then A, then C).
	vector := NewVectorSearcher(&fakeVectorReader{results: []store.SearchResult{
		{ID: idB, Content: "budget plan", SourcePath: "b", Distance: 0.1},
		{ID: idA, Content: "budget budget budget", SourcePath: "a", Distance: 0.2},
		{ID: idC, Content: "weather climate", SourcePath: "c", Distance: 0.9},
	}})

	h := NewHybridSearcher(bm, vector, 0)

	got, err := h.Search(context.Background(), "budget", []float32{1, 0, 0}, 3)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	// C (single-list) must be last.
	if got[2].ID != idC {
		t.Errorf("last result = %v, want single-list doc C", got[2].ID)
	}
	// A and B both appear in two lists; C only in vector → A,B outrank C.
	if got[0].Score < got[2].Score || got[1].Score < got[2].Score {
		t.Errorf("single-list doc did not rank last: scores=%v", []float64{got[0].Score, got[1].Score, got[2].Score})
	}
}

func TestHybrid_IndexForwardsToBM25(t *testing.T) {
	bm := NewBM25Searcher()
	h := NewHybridSearcher(bm, NewVectorSearcher(&fakeVectorReader{}), 0)
	id := uuid.New()
	if err := h.Index(context.Background(), []chunk.Chunk{
		{ID: id, Content: "taxes taxes", SourcePath: "t"},
	}); err != nil {
		t.Fatalf("Index error: %v", err)
	}
	got, err := h.bm25.Search(context.Background(), "taxes", nil, 1)
	if err != nil {
		t.Fatalf("bm25 Search error: %v", err)
	}
	if len(got) != 1 || got[0].ID != id {
		t.Errorf("bm25 did not find indexed doc: %+v", got)
	}
}

func TestHybrid_TopKTruncation(t *testing.T) {
	bm := NewBM25Searcher()
	chunks := []chunk.Chunk{
		{ID: uuid.New(), Content: "alpha alpha alpha"},
		{ID: uuid.New(), Content: "alpha beta"},
		{ID: uuid.New(), Content: "alpha gamma"},
	}
	if err := bm.Index(context.Background(), chunks); err != nil {
		t.Fatalf("Index error: %v", err)
	}
	vector := NewVectorSearcher(&fakeVectorReader{results: []store.SearchResult{
		{ID: chunks[0].ID, Content: "alpha", Distance: 0.1},
		{ID: chunks[1].ID, Content: "alpha", Distance: 0.2},
		{ID: chunks[2].ID, Content: "alpha", Distance: 0.3},
	}})
	h := NewHybridSearcher(bm, vector, 0)

	got, err := h.Search(context.Background(), "alpha", []float32{1}, 2)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}
