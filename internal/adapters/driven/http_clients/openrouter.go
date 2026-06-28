package http_clients

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/lin-br/go-linai-tools/internal/configs"
)

const (
	OpenRouterModel = "openrouter/owl-alpha"
	OpenRouterUrl   = "https://openrouter.ai/api/v1/messages"
)

type OpenRouterClient struct {
	configs configs.Config
}

func NewOpenRouterClient(config configs.Config) *OpenRouterClient {
	return &OpenRouterClient{configs: config}
}

func (o OpenRouterClient) DoMessagesRequest(prompt string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	payload := o.makePayload(prompt)
	request := o.makeRequest(payload)

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	respBody, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	log.Println(string(respBody))
	var messageResponse AnthropicMessageResponse
	err = json.Unmarshal(respBody, &messageResponse)
	if err != nil {
		return "", err
	}

	contents := messageResponse.Content
	for _, content := range contents {
		if content.Type == "text" {
			return content.Text, nil
		}
	}
	return "", errors.New("response contents is empty")
}

func (o OpenRouterClient) makeRequest(payload *bytes.Reader) *http.Request {
	request, err := http.NewRequest(http.MethodPost, OpenRouterUrl, payload)
	if err != nil {
		log.Fatal(err)
	}
	request.Header.Add("Authorization", "Bearer "+*o.configs.OpenRouterApiKey)
	request.Header.Add("HTTP-Referer", "lin.com.br")
	request.Header.Add("X-OpenRouter-Title", "lin.com.br")
	return request
}

func (o OpenRouterClient) makePayload(prompt string) *bytes.Reader {
	body := map[string]interface{}{
		"model": OpenRouterModel,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
	}
	jsonString, _ := json.Marshal(body)
	payload := bytes.NewReader(jsonString)
	return payload
}
