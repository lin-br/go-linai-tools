package chunk

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/lin-br/go-linai-tools/internal/core/domain"
	"github.com/lin-br/go-linai-tools/internal/core/ports/outbound"
)

// maxSummaryInputRunes bounds the document text sent to the chat provider for
// contextual summary generation, keeping the request within typical context
// budgets. Truncation is a deliberate trade-off (design D6 / risk note).
const maxSummaryInputRunes = 8000

const summarySystemPrompt = "Summarize the following document in exactly one sentence. " +
	"Output only that single sentence and no other text."

// ContextualChunker wraps a base Chunker and prepends a one-sentence document
// summary (Anthropic 2024 contextual chunking) to every chunk's Content.
type ContextualChunker struct {
	base     Chunker
	provider outbound.Provider
	model    string
}

// Compile-time check: ContextualChunker implements Chunker.
var _ Chunker = (*ContextualChunker)(nil)

// NewContextualChunker wraps base with contextual chunking. The provider is
// used to generate the one-sentence summary; model selects the chat model.
func NewContextualChunker(base Chunker, provider outbound.Provider, model string) *ContextualChunker {
	return &ContextualChunker{base: base, provider: provider, model: model}
}

// Split runs the base chunker, then asks the provider for a one-sentence
// document summary and prepends "Context: <summary>\n\n" to each chunk.
// Provider errors propagate; no chunks are returned on failure.
func (c *ContextualChunker) Split(ctx context.Context, doc Document) ([]Chunk, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw, err := c.base.Split(ctx, doc)
	if err != nil {
		return nil, err
	}

	summary, err := c.summarize(ctx, doc.Content)
	if err != nil {
		return nil, fmt.Errorf("contextual chunk: summary: %w", err)
	}

	prefix := "Context: " + summary + "\n\n"
	out := make([]Chunk, len(raw))
	for i, ch := range raw {
		ch.Content = prefix + ch.Content
		ch.Metadata = setMeta(ch.Metadata, ch.Index, "contextual")
		out[i] = ch
	}
	return out, nil
}

// summarize calls the chat provider for a one-sentence summary, truncating the
// document to maxSummaryInputRunes to bound the request.
func (c *ContextualChunker) summarize(ctx context.Context, content string) (string, error) {
	doc := content
	if utf8.RuneCountInString(content) > maxSummaryInputRunes {
		doc = string([]rune(content)[:maxSummaryInputRunes])
	}
	req := &domain.ChatRequest{
		Model:    c.model,
		System:   summarySystemPrompt,
		Messages: []domain.Message{{Role: domain.MessageRoleUser, Content: doc}},
	}
	resp, err := c.provider.Chat(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}
