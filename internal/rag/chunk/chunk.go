package chunk

import (
	"context"

	"github.com/google/uuid"
)

// DefaultChunkSize is the default target chunk length in runes.
const DefaultChunkSize = 512

// DefaultChunkOverlap is the default overlap between adjacent chunks in runes.
const DefaultChunkOverlap = 50

// Chunk is a slice of a document produced by a Chunker. ID and SourcePath are
// zero-valued when produced by chunking and populated when loaded from the
// store; search and BM25 indexing rely on ID to merge results across retrievers.
type Chunk struct {
	ID         uuid.UUID      `json:"id"`
	Content    string         `json:"content"`
	SourcePath string         `json:"source_path"`
	Index      int            `json:"index"`
	Metadata   map[string]any `json:"metadata"`
}

// Document is the input to a Chunker: raw text plus its origin path.
type Document struct {
	SourcePath string
	Content    string
}

// Chunker splits a Document into Chunks.
type Chunker interface {
	Split(ctx context.Context, doc Document) ([]Chunk, error)
}

// Metadata keys populated by the chunkers.
const (
	MetaKeyIndex    = "index"
	MetaKeyStrategy = "strategy"
)

// setMeta fills the required metadata keys without overwriting caller-provided
// index, and forces the strategy to the given value.
func setMeta(meta map[string]any, index int, strategy string) map[string]any {
	if meta == nil {
		meta = map[string]any{}
	}
	meta[MetaKeyIndex] = index
	meta[MetaKeyStrategy] = strategy
	return meta
}
