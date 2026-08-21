package domain

// EmbeddingRequest is the provider-agnostic request for text embeddings. The
// Model field selects the embedding model on the provider (e.g.
// "voyage/voyage-3-large" or "openai/text-embedding-3-small" on OpenRouter).
// Input is the batch of texts to embed. Dimensions, EncodingFormat, and
// InputType are optional provider-specific knobs.
type EmbeddingRequest struct {
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	Dimensions     int      `json:"dimensions,omitempty"`
	EncodingFormat string   `json:"encoding_format,omitempty"`
	InputType      string   `json:"input_type,omitempty"`
}

// EmbeddingResponse is the provider-agnostic response from an embeddings call.
// Data is ordered to match the input batch; each item carries its Index.
type EmbeddingResponse struct {
	Model string          `json:"model"`
	Data  []EmbeddingData `json:"data"`
	Usage EmbeddingUsage  `json:"usage"`
}

// EmbeddingData wraps a single embedding vector with its position in the batch.
type EmbeddingData struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
	Object    string    `json:"object"`
}

// EmbeddingUsage reports token usage for an embeddings call.
type EmbeddingUsage struct {
	PromptTokens int     `json:"prompt_tokens"`
	TotalTokens  int     `json:"total_tokens"`
	Cost         float64 `json:"cost,omitempty"`
}
