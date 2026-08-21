package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/lin-br/go-linai-tools/internal/configs"
	"github.com/lin-br/go-linai-tools/internal/core/domain"
	"github.com/lin-br/go-linai-tools/internal/rag/chunk"
	"github.com/lin-br/go-linai-tools/internal/rag/store"
)

// runIngest implements the `rag ingest <file>` subcommand.
func runIngest(ctx context.Context, cfg *configs.Config, args []string) error {
	fs := flag.NewFlagSet("ingest", flag.ContinueOnError)
	db := fs.String("db", defaultDSN(cfg), "Postgres DSN (env POSTGRES_DSN)")
	chunkSize := fs.Int("chunk-size", chunk.DefaultChunkSize, "chunk size in runes")
	chunkOverlap := fs.Int("chunk-overlap", chunk.DefaultChunkOverlap, "chunk overlap in runes")
	contextual := fs.Bool("contextual", false, "use contextual chunking (LLM summary per document)")
	model := fs.String("model", "", "chat model for contextual summary")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return errors.New("ingest: missing file argument")
	}
	file := fs.Arg(0)

	if err := requireKey("OPENROUTER_API_KEY", cfg.OpenRouterApiKey); err != nil {
		return err
	}
	dsn := *db
	if dsn == "" {
		return errors.New("POSTGRES_DSN is required")
	}

	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("ingest: read %s: %w", file, err)
	}

	emb, err := buildEmbedder(cfg)
	if err != nil {
		return err
	}
	st, err := buildStore(ctx, dsn)
	if err != nil {
		return err
	}

	var chunker chunk.Chunker = chunk.NewRecursiveChunker(*chunkSize, *chunkOverlap)
	if *contextual {
		provider := buildProvider(cfg)
		chunker = chunk.NewContextualChunker(
			chunk.NewRecursiveChunker(*chunkSize, *chunkOverlap),
			provider,
			resolveModel(cfg, *model),
		)
	}

	doc := chunk.Document{SourcePath: file, Content: string(content)}
	chunks, err := chunker.Split(ctx, doc)
	if err != nil {
		return fmt.Errorf("ingest: chunk: %w", err)
	}
	if len(chunks) == 0 {
		return fmt.Errorf("ingest: %s produced no chunks", file)
	}

	inputs := make([]string, len(chunks))
	for i, c := range chunks {
		inputs[i] = c.Content
	}
	embResp, err := emb.Embed(ctx, &domain.EmbeddingRequest{
		Model: *cfg.DefaultEmbeddingModel,
		Input: inputs,
		//InputType: "document",
		// Nvidia embeddings only support "query" and "passage"
		InputType: "passage",
	})
	if err != nil {
		return fmt.Errorf("ingest: embed: %w", err)
	}
	vectors := extractEmbeddingVectors(embResp)
	for i := range vectors {
		vectors[i] = normalize(vectors[i])
	}

	storeChunks := make([]store.Chunk, len(chunks))
	for i, c := range chunks {
		var vec []float32
		if i < len(vectors) {
			vec = vectors[i]
		}
		storeChunks[i] = store.Chunk{
			Content:    c.Content,
			Embedding:  vec,
			Metadata:   c.Metadata,
			SourcePath: c.SourcePath,
		}
	}

	ids, err := st.InsertChunks(ctx, storeChunks)
	if err != nil {
		return fmt.Errorf("ingest: store: %w", err)
	}
	fmt.Printf("stored %d chunks from %s\n", len(ids), file)
	return nil
}

// defaultDSN returns the config Postgres DSN or empty string for the -db flag.
func defaultDSN(cfg *configs.Config) string {
	if cfg.PostgresDSN != nil {
		return *cfg.PostgresDSN
	}
	return ""
}
