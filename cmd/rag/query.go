package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/lin-br/go-linai-tools/internal/configs"
	"github.com/lin-br/go-linai-tools/internal/core/domain"
	"github.com/lin-br/go-linai-tools/internal/rag/search"
)

const querySystemPrompt = "Answer the question using only the provided context. " +
	"cite source paths in your answer. If the context does not contain the answer, say so."

// runQuery implements the `rag query "<question>"` subcommand.
func runQuery(ctx context.Context, cfg *configs.Config, args []string) error {
	fs := flag.NewFlagSet("query", flag.ContinueOnError)
	db := fs.String("db", defaultDSN(cfg), "Postgres DSN (env POSTGRES_DSN)")
	topK := fs.Int("top-k", 5, "top k results to answer from")
	model := fs.String("model", "", "chat model")
	rerankFlag := fs.Bool("rerank", true, "enable OpenRouter reranking")
	_ = fs.Bool("contextual", false, "expect context-prefixed chunks from contextual chunking")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return errors.New("query: missing question argument")
	}
	question := fs.Arg(0)

	if err := requireKey("OPENROUTER_API_KEY", cfg.OpenRouterApiKey); err != nil {
		return err
	}
	dsn := *db
	if dsn == "" {
		return errors.New("POSTGRES_DSN is required")
	}

	emb, err := buildEmbedder(cfg)
	if err != nil {
		return err
	}
	st, err := buildStore(ctx, dsn)
	if err != nil {
		return err
	}
	all, err := st.ListChunks(ctx)
	if err != nil {
		return fmt.Errorf("query: list chunks: %w", err)
	}
	if len(all) == 0 {
		return errors.New("no chunks found; run 'rag ingest <file>' first")
	}

	hybrid := search.NewHybridSearcher(
		search.NewBM25Searcher(),
		search.NewVectorSearcher(st),
		0,
	)
	if err := hybrid.Index(ctx, toChunks(all)); err != nil {
		return fmt.Errorf("query: index: %w", err)
	}

	var rerankClient = buildRerankerOptional(cfg)
	if *rerankFlag && rerankClient == nil {
		return errors.New("OPENROUTER_API_KEY is required for reranking")
	}
	if !*rerankFlag {
		rerankClient = nil
	}

	rs := &ragSearcher{
		emb:            emb,
		embeddingModel: *cfg.DefaultEmbeddingModel,
		hybrid:         hybrid,
		rerank:         rerankClient,
		rerankModel:    *cfg.DefaultRerankModel,
	}
	results, err := rs.Search(ctx, question, nil, *topK)
	if err != nil {
		return fmt.Errorf("query: search: %w", err)
	}
	if len(results) == 0 {
		return errors.New("no chunks found; run 'rag ingest <file>' first")
	}

	var sb strings.Builder
	for _, r := range results {
		fmt.Fprintf(&sb, "[source: %s]\n%s\n\n", r.SourcePath, r.Content)
	}

	provider := buildProvider(cfg)
	req := &domain.ChatRequest{
		Model:  resolveModel(cfg, *model),
		System: querySystemPrompt,
		Messages: []domain.Message{{
			Role:    domain.MessageRoleUser,
			Content: "Context:\n" + sb.String() + "Question: " + question,
		}},
	}
	resp, err := provider.Chat(ctx, req)
	if err != nil {
		return fmt.Errorf("query: chat: %w", err)
	}
	fmt.Println(resp.Content)
	return nil
}
