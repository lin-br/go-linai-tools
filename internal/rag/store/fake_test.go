package store

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
)

// fakeQuerier is an in-memory Querier used by store tests and by downstream
// search tests via the exported FakeStore adapter. It records inserts parsed
// from the INSERT SQL and answers QueryChunks by computing Euclidean distance
// against the stored embeddings.
type fakeQuerier struct {
	chunks []Chunk
}

func newFakeQuerier() *fakeQuerier { return &fakeQuerier{} }

func (q *fakeQuerier) Exec(ctx context.Context, sql string, args ...any) error {
	if strings.Contains(sql, "INSERT INTO chunks") {
		if len(args) < 5 {
			return nil
		}
		id, _ := args[0].(uuid.UUID)
		content, _ := args[1].(string)
		emb := extractEmbedding(args[2])
		metaStr, _ := args[3].(string)
		source, _ := args[4].(string)
		var meta map[string]any
		if metaStr != "" {
			_ = json.Unmarshal([]byte(metaStr), &meta)
		}
		q.chunks = append(q.chunks, Chunk{
			ID: id, Content: content, Embedding: emb, Metadata: meta, SourcePath: source,
		})
	}
	return nil
}

func (q *fakeQuerier) WithTx(ctx context.Context, fn func(Execer) error) error {
	return fn(q)
}

func (q *fakeQuerier) ListChunks(ctx context.Context) ([]Chunk, error) {
	out := make([]Chunk, len(q.chunks))
	for i, c := range q.chunks {
		out[i] = Chunk{
			ID:         c.ID,
			Content:    c.Content,
			Metadata:   copyMap(c.Metadata),
			SourcePath: c.SourcePath,
		}
	}
	return out, nil
}

func (q *fakeQuerier) QueryChunks(ctx context.Context, queryVec []float32, k int) ([]SearchResult, error) {
	type scored struct {
		idx      int
		distance float64
	}
	out := make([]scored, 0, len(q.chunks))
	for i, c := range q.chunks {
		out = append(out, scored{idx: i, distance: euclidean(c.Embedding, queryVec)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].distance != out[j].distance {
			return out[i].distance < out[j].distance
		}
		return out[i].idx < out[j].idx
	})
	if k > len(out) {
		k = len(out)
	}
	results := make([]SearchResult, 0, k)
	for i := 0; i < k; i++ {
		c := q.chunks[out[i].idx]
		results = append(results, SearchResult{
			ID:         c.ID,
			Content:    c.Content,
			SourcePath: c.SourcePath,
			Metadata:   copyMap(c.Metadata),
			Distance:   out[i].distance,
		})
	}
	return results, nil
}

// extractEmbedding handles both pgvector.Vector and []float32 args so the fake
// works regardless of how the caller wrapped the embedding.
func extractEmbedding(v any) []float32 {
	switch t := v.(type) {
	case pgvector.Vector:
		return t.Slice()
	case []float32:
		return t
	}
	return nil
}

func euclidean(a, b []float32) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var sum float64
	for i := 0; i < n; i++ {
		d := float64(a[i]) - float64(b[i])
		sum += d * d
	}
	return math.Sqrt(sum)
}

func copyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
