package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	clients "github.com/lin-br/go-linai-tools/internal/adapters/driven/http_clients"
	"github.com/lin-br/go-linai-tools/internal/adapters/driving"
	"github.com/lin-br/go-linai-tools/internal/configs"
	"github.com/lin-br/go-linai-tools/internal/core/usecases"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	properties := getProperties()
	provider := buildProvider(properties)
	sendMessageUseCase := usecases.NewSendMessageUseCase(*properties, provider)
	cli := driving.NewCLI(sendMessageUseCase)
	cli.StartAgent(ctx, os.Stdin, os.Stdout)
}

func getProperties() *configs.Config {
	properties, err := configs.LoadConfigs()
	if err != nil {
		log.Fatal(err)
	}
	return properties
}

func buildProvider(properties *configs.Config) *clients.OpenRouterProvider {
	switch properties.Provider {
	case configs.ProviderOpenRouter:
		return clients.NewOpenRouterProvider(*properties.OpenRouterApiKey)
	default:
		log.Fatalf("unsupported provider: %s", properties.Provider)
		return nil
	}
}
