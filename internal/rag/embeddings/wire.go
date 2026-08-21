package embeddings

// voyageRequest is the request body POSTed to the Voyage AI embeddings
// endpoint. Field order matches the wire scenario in the MP7 spec.
type voyageRequest struct {
	Input     []string `json:"input"`
	Model     string   `json:"model"`
	InputType string   `json:"input_type,omitempty"`
}

// voyageEmbedding wraps a single embedding vector returned by Voyage.
type voyageEmbedding struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

// voyageUsage reports token usage for an embeddings call.
type voyageUsage struct {
	TotalTokens int `json:"total_tokens"`
}

// voyageResponse is the response body from the Voyage AI embeddings endpoint.
type voyageResponse struct {
	Data  []voyageEmbedding `json:"data"`
	Model string            `json:"model"`
	Usage voyageUsage       `json:"usage"`
}
