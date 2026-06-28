package main

import (
	"bufio"
	"fmt"
	"log"
	"os"

	clients "github.com/lin-br/go-linai-tools/internal/adapters/driven/http_clients"
	"github.com/lin-br/go-linai-tools/internal/configs"
)

func main() {
	properties := getProperties()
	openRouter := clients.NewOpenRouterClient(*properties)

	fmt.Println("What you need?")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	response, err := openRouter.DoMessagesRequest(scanner.Text())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s", response)
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
