package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"

	"github.com/lin-br/go-linai-tools/internal/configs"
	"github.com/lin-br/go-linai-tools/internal/rag/eval"
	"github.com/lin-br/go-linai-tools/internal/rag/search"
)

// runEval implements the `rag eval -dataset <path>` subcommand.
func runEval(ctx context.Context, cfg *configs.Config, args []string) error {
	fs := flag.NewFlagSet("eval", flag.ContinueOnError)
	db := fs.String("db", defaultDSN(cfg), "Postgres DSN (env POSTGRES_DSN)")
	dataset := fs.String("dataset", "testdata/golden.jsonl", "golden .jsonl dataset path")
	topK := fs.Int("top-k", 5, "top k for retrieval metrics")
	judgeFlag := fs.Bool("judge", false, "enable LLM-as-judge scoring")
	model := fs.String("model", "", "chat model for the judge")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := requireKey("OPENROUTER_API_KEY", cfg.OpenRouterApiKey); err != nil {
		return err
	}
	dsn := *db
	if dsn == "" {
		return errors.New("POSTGRES_DSN is required")
	}

	ds, err := eval.LoadDataset(*dataset)
	if err != nil {
		return fmt.Errorf("eval: %w", err)
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
		return fmt.Errorf("eval: list chunks: %w", err)
	}
	hybrid := search.NewHybridSearcher(
		search.NewBM25Searcher(),
		search.NewVectorSearcher(st),
		0,
	)
	if len(all) > 0 {
		if err := hybrid.Index(ctx, toChunks(all)); err != nil {
			return fmt.Errorf("eval: index: %w", err)
		}
	}

	rs := &ragSearcher{
		emb:            emb,
		embeddingModel: *cfg.DefaultEmbeddingModel,
		hybrid:         hybrid,
		rerank:         buildRerankerOptional(cfg),
		rerankModel:    *cfg.DefaultRerankModel,
	}

	var judge *eval.Judge
	if *judgeFlag {
		if err := requireKey("OPENROUTER_API_KEY", cfg.OpenRouterApiKey); err != nil {
			return err
		}
		judge = eval.NewJudge(buildProvider(cfg), resolveModel(cfg, *model))
	}

	ev := eval.NewEvaluator(rs, judge, *topK)
	report, err := ev.Run(ctx, ds)
	if err != nil {
		return fmt.Errorf("eval: run: %w", err)
	}

	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("eval: marshal: %w", err)
	}
	fmt.Println(string(out))
	return nil
}
