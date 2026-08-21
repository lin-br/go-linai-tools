package rerank

// rerankRequest is the request body POSTed to the Cohere v2 rerank endpoint.
type rerankRequest struct {
	Query     string   `json:"query"`
	Model     string   `json:"model"`
	Documents []string `json:"documents"`
	TopN      int      `json:"top_n"`
}

// rerankResult is a single item in the Cohere rerank response: the original
// candidate index and its relevance score.
type rerankResult struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
}

// rerankResponse is the response body from the Cohere v2 rerank endpoint.
type rerankResponse struct {
	Results []rerankResult `json:"results"`
	ID      string         `json:"id,omitempty"`
}
