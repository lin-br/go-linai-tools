package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"

	clients "github.com/lin-br/go-linai-tools/internal/adapters/driven/http_clients"
	"github.com/lin-br/go-linai-tools/internal/adapters/driven/retry"
	"github.com/lin-br/go-linai-tools/internal/configs"
	"github.com/lin-br/go-linai-tools/internal/core/tools"
	"github.com/lin-br/go-linai-tools/internal/core/usecases"
)

func main() {
	modelFlag := flag.String("model", "", "model id to use (overrides config default)")
	formatFlag := flag.String("format", "json", "output format (only \"json\" is supported)")
	prettyFlag := flag.Bool("pretty", true, "indent JSON output with 2-space indentation")
	flag.Parse()

	if *formatFlag != "json" {
		fmt.Fprintf(os.Stderr, "unsupported format: %q (only \"json\" is supported)\n", *formatFlag)
		os.Exit(1)
	}

	cfg, err := configs.LoadConfigs()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	provider := buildProvider(cfg)
	useCase := usecases.NewExtractUseCase(provider)

	model := resolveModel(cfg, *modelFlag)
	if model == "" {
		fmt.Fprintln(os.Stderr, "no model resolved: set -model flag or configure a default model")
		os.Exit(1)
	}

	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read input: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	result, err := useCase.Extract(ctx, model, string(input))
	if err != nil {
		handleError(err)
	}

	var out []byte
	if *prettyFlag {
		out, err = json.MarshalIndent(result, "", "  ")
	} else {
		out, err = json.Marshal(result)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal result: %v\n", err)
		os.Exit(1)
	}

	os.Stdout.Write(out)
	os.Stdout.Write([]byte("\n"))
	os.Exit(0)
}

// handleError maps extraction errors to user-facing messages on stderr and
// exits with code 1. It checks tools.ErrNoToolCall and tools.ErrUnmarshalFailed
// (MP3 typed errors) before falling back to a generic message. No partial JSON
// is written to stdout on any error path.
func handleError(err error) {
	switch {
	case errors.Is(err, tools.ErrNoToolCall):
		fmt.Fprintln(os.Stderr, "Model did not return structured data.")
	case errors.Is(err, tools.ErrUnmarshalFailed):
		fmt.Fprintf(os.Stderr, "Failed to parse structured data: %v\n", err)
	default:
		fmt.Fprintf(os.Stderr, "Extraction failed: %v\n", err)
	}
	os.Exit(1)
}

// buildProvider constructs the provider chain from config: OpenRouterProvider
// wrapped in a RetryProvider (MP2).
func buildProvider(cfg *configs.Config) *retry.RetryProvider {
	switch cfg.Provider {
	case configs.ProviderOpenRouter:
		inner := clients.NewOpenRouterProvider(*cfg.OpenRouterApiKey)
		return retry.NewRetryProvider(inner)
	default:
		log.Fatalf("unsupported provider: %s", cfg.Provider)
		return nil
	}
}

// resolveModel returns the -model flag value when set, otherwise the config
// default. Returns empty string when neither is available.
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
