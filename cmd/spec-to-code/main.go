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
	formatFlag := flag.String("format", "json", "output format: \"json\" or \"text\"")
	langFlag := flag.String("lang", "go", "target language hint passed to the system prompt")
	flag.Parse()

	if *formatFlag != "json" && *formatFlag != "text" {
		fmt.Fprintf(os.Stderr, "unsupported format: %q (only \"json\" and \"text\" are supported)\n", *formatFlag)
		os.Exit(1)
	}

	cfg, err := configs.LoadConfigs()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	provider := buildProvider(cfg)

	model := resolveModel(cfg, *modelFlag)
	if model == "" {
		fmt.Fprintln(os.Stderr, "no model resolved: set -model flag or configure a default model")
		os.Exit(1)
	}

	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read input: %v\n", err)
		os.Exit(1)
	}
	if len(input) == 0 {
		fmt.Fprintln(os.Stderr, "no input: provide a feature description via stdin")
		os.Exit(1)
	}

	useCase := usecases.NewSpecToCodeUseCase(provider, model, *langFlag)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	plan, err := useCase.Plan(ctx, string(input))
	if err != nil {
		handleError(err)
	}

	switch *formatFlag {
	case "json":
		out, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to marshal plan: %v\n", err)
			os.Exit(1)
		}
		os.Stdout.Write(out)
		os.Stdout.Write([]byte("\n"))
	case "text":
		os.Stdout.WriteString(renderTree(plan))
	}
	os.Exit(0)
}

// handleError maps use case errors to user-facing messages on stderr and the
// appropriate exit code. context.Canceled (Ctrl+C) exits 130 with no message.
// Sentinel errors from tools.Extract get specific messages; all other errors
// get a generic "request failed" message.
func handleError(err error) {
	switch {
	case errors.Is(err, context.Canceled):
		os.Exit(130)
	case errors.Is(err, tools.ErrNoToolCall):
		fmt.Fprintln(os.Stderr, "model did not return a structured plan")
		os.Exit(1)
	case errors.Is(err, tools.ErrToolNameMismatch):
		fmt.Fprintln(os.Stderr, "model returned an unexpected tool call")
		os.Exit(1)
	case errors.Is(err, tools.ErrUnmarshalFailed):
		fmt.Fprintf(os.Stderr, "failed to parse model output: %v\n", err)
		os.Exit(1)
	default:
		fmt.Fprintf(os.Stderr, "request failed: %v\n", err)
		os.Exit(1)
	}
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
