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
	OpenRouterUrl = "https://openrouter.ai/api/v1/messages"
)

type OpenRouterClient struct {
	configs configs.Config
}

func NewOpenRouterClient(config configs.Config) *OpenRouterClient {
	return &OpenRouterClient{configs: config}
}

func (o *OpenRouterClient) DoMessagesRequest(prompt string, model *string) (string, error) {
	model, err := o.parseModel(model)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	payload := o.makePayload(prompt, *model)
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

func (o *OpenRouterClient) makeRequest(payload *bytes.Reader) *http.Request {
	request, err := http.NewRequest(http.MethodPost, OpenRouterUrl, payload)
	if err != nil {
		log.Fatal(err)
	}
	request.Header.Add("Authorization", "Bearer "+*o.configs.OpenRouterApiKey)
	request.Header.Add("HTTP-Referer", "lin.com.br")
	request.Header.Add("X-OpenRouter-Title", "lin.com.br")
	return request
}

func (o *OpenRouterClient) makePayload(prompt string, model string) *bytes.Reader {
	body := map[string]interface{}{
		"model": model,
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

func (o *OpenRouterClient) parseModel(model *string) (*string, error) {
	if model == nil {
		if o.configs.Models.Get() == nil {
			return nil, errors.New("the AI model is empty")
		}
		return o.configs.Models.Get(), nil
	}
	return model, nil
}
