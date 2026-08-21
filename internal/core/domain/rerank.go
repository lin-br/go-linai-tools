package domain

// RerankDocument is a document to be reranked. Text is the plain-text content;
// Image is an optional URL or base64 data URI for multimodal models. At least
// one of Text or Image should be populated.
type RerankDocument struct {
	Text  string `json:"text,omitempty"`
	Image string `json:"image,omitempty"`
}

// RerankRequest is the provider-agnostic request for document reranking. Model
// selects the rerank model on the provider (e.g. "cohere/rerank-v3.5" on
// OpenRouter). Documents is the list to rank against Query. TopN limits the
// returned results; when zero the provider returns all documents ranked.
type RerankRequest struct {
	Model     string           `json:"model"`
	Query     string           `json:"query"`
	Documents []RerankDocument `json:"documents"`
	TopN      int              `json:"top_n,omitempty"`
}

// RerankResponse is the provider-agnostic response from a rerank call. Results
// are sorted by descending relevance.
type RerankResponse struct {
	ID      string         `json:"id"`
	Model   string         `json:"model"`
	Results []RerankResult `json:"results"`
	Usage   RerankUsage    `json:"usage,omitempty"`
}

// RerankResult is a single reranked document with its original index in the
// input list and its relevance score (higher is more relevant).
type RerankResult struct {
	Document       RerankDocument `json:"document"`
	Index          int            `json:"index"`
	RelevanceScore float64        `json:"relevance_score"`
}

// RerankUsage reports usage statistics for a rerank call.
type RerankUsage struct {
	SearchUnits int     `json:"search_units,omitempty"`
	TotalTokens int     `json:"total_tokens,omitempty"`
	Cost        float64 `json:"cost,omitempty"`
}
