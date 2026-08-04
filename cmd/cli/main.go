package main

import (
	"log"
	"os"

	clients "github.com/lin-br/go-linai-tools/internal/adapters/driven/http_clients"
	"github.com/lin-br/go-linai-tools/internal/adapters/driving"
	"github.com/lin-br/go-linai-tools/internal/configs"
	"github.com/lin-br/go-linai-tools/internal/core/usecases"
)

func main() {
	properties := getProperties()
	openRouter := clients.NewOpenRouterClient(*properties)
	sendMessageUseCase := usecases.NewSendMessageUseCase(*properties, openRouter)
	cli := driving.NewCLI(sendMessageUseCase)
	cli.StartAgent(os.Stdin, os.Stdout)
}

func getProperties() *configs.Config {
	properties, err := configs.LoadConfigs()
	if err != nil {
		log.Fatal(err)
	}

	if properties.OpenRouterApiKey == nil {
		log.Fatal("OpenAI API key is missing in configuration")
	}
	return properties
}
