package http_clients

import "net/http"

// setOpenRouterHeaders sets the common HTTP headers required by every
// OpenRouter API endpoint (chat, embeddings, rerank). The apiKey is sent as a
// Bearer token; HTTP-Referer and X-OpenRouter-Title identify the app.
func setOpenRouterHeaders(req *http.Request, apiKey string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("HTTP-Referer", "lin.com.br")
	req.Header.Set("X-OpenRouter-Title", "lin.com.br")
}
