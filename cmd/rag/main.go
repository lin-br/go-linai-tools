package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	clients "github.com/lin-br/go-linai-tools/internal/adapters/driven/http_clients"
	"github.com/lin-br/go-linai-tools/internal/adapters/driven/retry"
	"github.com/lin-br/go-linai-tools/internal/configs"
	"github.com/lin-br/go-linai-tools/internal/rag/store"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(1)
	}

	sub := os.Args[1]
	if sub == "-h" || sub == "--help" || sub == "help" {
		usage(os.Stdout)
		os.Exit(0)
	}

	// Reject unknown subcommands before loading config so the diagnostic does
	// not depend on environment variables.
	switch sub {
	case "ingest", "query", "eval":
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", sub)
		os.Exit(1)
	}

	cfg, err := configs.LoadConfigs()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	args := os.Args[2:]
	var runErr error
	switch sub {
	case "ingest":
		runErr = runIngest(ctx, cfg, args)
	case "query":
		runErr = runQuery(ctx, cfg, args)
	case "eval":
		runErr = runEval(ctx, cfg, args)
	}
	if runErr != nil {
		fmt.Fprintln(os.Stderr, runErr)
		os.Exit(1)
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "Usage: rag <ingest|query|eval> [flags] <args>")
	fmt.Fprintln(w, "  ingest <file>      Chunk, embed, and store a text file")
	fmt.Fprintln(w, "  query \"<question>\" Answer a question using hybrid search + rerank")
	fmt.Fprintln(w, "  eval -dataset <p>  Run the golden dataset and print metrics JSON")
}

// buildProvider constructs the chat provider chain (OpenRouter + retry).
func buildProvider(cfg *configs.Config) *retry.RetryProvider {
	inner := clients.NewOpenRouterProvider(*cfg.OpenRouterApiKey)
	return retry.NewRetryProvider(inner)
}

// buildEmbedder returns an OpenRouter embeddings provider from config. The
// model is supplied per-request via domain.EmbeddingRequest, so the provider
// only needs the API key.
func buildEmbedder(cfg *configs.Config) (*clients.OpenRouterEmbeddingsProvider, error) {
	if err := requireKey("OPENROUTER_API_KEY", cfg.OpenRouterApiKey); err != nil {
		return nil, err
	}
	return clients.NewOpenRouterEmbeddingsProvider(*cfg.OpenRouterApiKey), nil
}

// buildReranker returns an OpenRouter rerank provider. Errors when the key is
// missing so callers that treat rerank as required surface a clear message.
func buildReranker(cfg *configs.Config) (*clients.OpenRouterRerankProvider, error) {
	if err := requireKey("OPENROUTER_API_KEY", cfg.OpenRouterApiKey); err != nil {
		return nil, err
	}
	return clients.NewOpenRouterRerankProvider(*cfg.OpenRouterApiKey), nil
}

// buildRerankerOptional returns an OpenRouter rerank provider or nil when the
// key is absent.
func buildRerankerOptional(cfg *configs.Config) *clients.OpenRouterRerankProvider {
	if cfg.OpenRouterApiKey == nil || *cfg.OpenRouterApiKey == "" {
		return nil
	}
	c, err := buildReranker(cfg)
	if err != nil {
		return nil
	}
	return c
}

// buildStore opens a pgxpool (pgvector types registered), wraps it in a Store,
// and initializes the schema.
func buildStore(ctx context.Context, dsn string) (*store.Store, error) {
	pool, err := store.NewPool(ctx, dsn)
	if err != nil {
		return nil, err
	}
	s, err := store.NewStore(pool)
	if err != nil {
		return nil, err
	}
	if err := s.InitSchema(ctx); err != nil {
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return s, nil
}

// requireKey validates a config pointer key, returning "<name> is required".
func requireKey(name string, v *string) error {
	if v == nil || *v == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

// resolveModel returns the flag value when set, else the config default.
func resolveModel(cfg *configs.Config, modelFlag string) string {
	if modelFlag != "" {
		return modelFlag
	}
	if cfg.Models != nil {
		if m := cfg.Models.Get(); m != nil {
			return *m
		}
	}
	return ""
}
