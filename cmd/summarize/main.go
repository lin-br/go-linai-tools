package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"

	clients "github.com/lin-br/go-linai-tools/internal/adapters/driven/http_clients"
	"github.com/lin-br/go-linai-tools/internal/adapters/driven/retry"
	"github.com/lin-br/go-linai-tools/internal/configs"
	"github.com/lin-br/go-linai-tools/internal/core/usecases"
)

func main() {
	modelFlag := flag.String("model", "", "model id to use (overrides config default)")
	systemFlag := flag.String("system", "", "system prompt to use (overrides default summarize prompt)")
	flag.Parse()

	cfg, err := configs.LoadConfigs()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	provider := buildProvider(cfg)
	useCase := usecases.NewSummarizeUseCase(provider)

	model := resolveModel(cfg, *modelFlag)
	if model == "" {
		fmt.Fprintln(os.Stderr, "no model resolved: set -model flag or configure a default model")
		os.Exit(1)
	}

	systemPrompt := *systemFlag
	if systemPrompt == "" {
		systemPrompt = usecases.DefaultSummarizeSystemPrompt
	}

	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := useCase.Stream(ctx, model, systemPrompt, string(input), os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
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
