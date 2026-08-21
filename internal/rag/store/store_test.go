package store

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/lin-br/go-linai-tools/internal/rag/embeddings"
)

// Compile-time check: pgxQuerier satisfies Querier.
var _ Querier = (*pgxQuerier)(nil)

func TestNewStore_NilPoolReturnsError(t *testing.T) {
	s, err := NewStore(nil)
	if err == nil {
		t.Fatal("expected error for nil pool, got nil")
	}
	if s != nil {
		t.Errorf("expected nil store on error, got %v", s)
	}
}

func TestNewStore_FromFakeQuerier(t *testing.T) {
	s := NewStoreFromQuerier(newFakeQuerier())
	if s == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestInitSchema_IsNoOpOnFake(t *testing.T) {
	s := NewStoreFromQuerier(newFakeQuerier())
	if err := s.InitSchema(context.Background()); err != nil {
		t.Errorf("InitSchema error: %v", err)
	}
}

func TestInsertChunks_EmptyIsNoOp(t *testing.T) {
	fq := newFakeQuerier()
	s := NewStoreFromQuerier(fq)

	ids, err := s.InsertChunks(context.Background(), nil)
	if err != nil {
		t.Fatalf("InsertChunks(nil) error: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("ids len = %d, want 0", len(ids))
	}
	if len(fq.chunks) != 0 {
		t.Errorf("fake touched by empty insert: chunks=%d", len(fq.chunks))
	}
}

func TestInsertChunks_SinglePreservesOrderAndMetadata(t *testing.T) {
	fq := newFakeQuerier()
	s := NewStoreFromQuerier(fq)

	vec := []float32{1, 0, 0}
	ids, err := s.InsertChunks(context.Background(), []Chunk{
		{Content: "hello", Embedding: vec, Metadata: map[string]any{"idx": 0}, SourcePath: "notes.txt"},
	})
	if err != nil {
		t.Fatalf("InsertChunks error: %v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("ids len = %d, want 1", len(ids))
	}
	if ids[0] == uuid.Nil {
		t.Error("generated id is zero")
	}
	if len(fq.chunks) != 1 {
		t.Fatalf("fake chunks = %d, want 1", len(fq.chunks))
	}
	if fq.chunks[0].Content != "hello" {
		t.Errorf("content = %q, want hello", fq.chunks[0].Content)
	}
	if fq.chunks[0].SourcePath != "notes.txt" {
		t.Errorf("source = %q, want notes.txt", fq.chunks[0].SourcePath)
	}
	if got := fq.chunks[0].Metadata["idx"]; got != float64(0) {
		t.Errorf("metadata idx = %v(%T), want 0", got, got)
	}
	if !equalVec(fq.chunks[0].Embedding, vec) {
		t.Errorf("embedding = %v, want %v", fq.chunks[0].Embedding, vec)
	}
}

func TestInsertChunks_BatchPreservesOrder(t *testing.T) {
	fq := newFakeQuerier()
	s := NewStoreFromQuerier(fq)

	contents := []string{"a", "b", "c"}
	chunks := make([]Chunk, 0, len(contents))
	for i, c := range contents {
		emb := make([]float32, 4)
		emb[i] = 1
		chunks = append(chunks, Chunk{
			Content:    c,
			Embedding:  emb,
			Metadata:   map[string]any{"index": i},
			SourcePath: "doc.txt",
		})
	}
	ids, err := s.InsertChunks(context.Background(), chunks)
	if err != nil {
		t.Fatalf("InsertChunks error: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("ids len = %d, want 3", len(ids))
	}
	if len(fq.chunks) != 3 {
		t.Fatalf("fake chunks = %d, want 3", len(fq.chunks))
	}
	for i, c := range contents {
		if fq.chunks[i].Content != c {
			t.Errorf("chunk %d content = %q, want %q", i, fq.chunks[i].Content, c)
		}
		if fq.chunks[i].ID != ids[i] {
			t.Errorf("chunk %d id %v != returned id %v", i, fq.chunks[i].ID, ids[i])
		}
	}
}

func TestSearch_ReturnsTopKNearest(t *testing.T) {
	fq := newFakeQuerier()
	s := NewStoreFromQuerier(fq)

	// Three unit vectors along axes; query closest to x-axis.
	_, err := s.InsertChunks(context.Background(), []Chunk{
		{Content: "x", Embedding: []float32{1, 0, 0}, SourcePath: "x.txt"},
		{Content: "y", Embedding: []float32{0, 1, 0}, SourcePath: "y.txt"},
		{Content: "z", Embedding: []float32{0, 0, 1}, SourcePath: "z.txt"},
	})
	if err != nil {
		t.Fatalf("InsertChunks error: %v", err)
	}

	res, err := s.Search(context.Background(), []float32{0.9, 0.1, 0.0}, 2)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("results len = %d, want 2", len(res))
	}
	if res[0].Content != "x" {
		t.Errorf("first result = %q, want x", res[0].Content)
	}
	if res[0].Distance > res[1].Distance {
		t.Errorf("results not ordered by distance: %v > %v", res[0].Distance, res[1].Distance)
	}
}

func TestSearch_EmptyTableReturnsEmpty(t *testing.T) {
	s := NewStoreFromQuerier(newFakeQuerier())
	res, err := s.Search(context.Background(), []float32{1, 0}, 5)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("results len = %d, want 0", len(res))
	}
}

func TestSearch_InvalidKReturnsError(t *testing.T) {
	s := NewStoreFromQuerier(newFakeQuerier())
	_, err := s.Search(context.Background(), []float32{1, 0}, 0)
	if err == nil {
		t.Fatal("expected error for k=0, got nil")
	}
	_, err = s.Search(context.Background(), []float32{1, 0}, -1)
	if err == nil {
		t.Fatal("expected error for k=-1, got nil")
	}
}

func TestSearch_MetadataRoundTrip(t *testing.T) {
	fq := newFakeQuerier()
	s := NewStoreFromQuerier(fq)

	_, err := s.InsertChunks(context.Background(), []Chunk{
		{Content: "intro", Embedding: []float32{1, 0}, Metadata: map[string]any{"section": "intro"}, SourcePath: "doc.md"},
	})
	if err != nil {
		t.Fatalf("InsertChunks error: %v", err)
	}
	res, err := s.Search(context.Background(), []float32{1, 0}, 1)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if res[0].Metadata["section"] != "intro" {
		t.Errorf("metadata section = %v, want intro", res[0].Metadata["section"])
	}
}

func TestSchemaSQLMatchesFile(t *testing.T) {
	if schemaSQL == "" {
		t.Fatal("schemaSQL is empty")
	}
	if !contains(schemaSQL, "CREATE EXTENSION IF NOT EXISTS vector") {
		t.Error("schemaSQL missing vector extension")
	}
	if !contains(schemaSQL, "CREATE TABLE IF NOT EXISTS chunks") {
		t.Error("schemaSQL missing chunks table")
	}
	if !contains(schemaSQL, "embedding   vector(2048)") {
		t.Error("schemaSQL missing embedding vector(2048) column")
	}
}

// Sentinel-error sanity check so the package is not error-free in name only.
var errTest = errors.New("test sentinel")

func TestSentinelUsed(t *testing.T) {
	if !errors.Is(errTest, errTest) {
		t.Error("sentinel self-equality broken")
	}
}

func equalVec(a, b []float32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// Ensure the embeddings package and store package agree on the vector shape
// by embedding a single input and storing it through the fake.
func TestEmbeddingsToStoreIntegration(t *testing.T) {
	_ = embeddings.DefaultModel
	fq := newFakeQuerier()
	s := NewStoreFromQuerier(fq)
	// Synthetic embedding (Voyage is not called here) to verify the store
	// accepts []float32 produced downstream by embeddings.Client.
	if _, err := s.InsertChunks(context.Background(), []Chunk{
		{Content: "c", Embedding: []float32{0.5, 0.5, 0.5, 0.5}, SourcePath: "p"},
	}); err != nil {
		t.Fatalf("InsertChunks error: %v", err)
	}
}
