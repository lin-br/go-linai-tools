package domain

// Usage captures token and billing information for a chat request.
type Usage struct {
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
	Cost         *float64
}
