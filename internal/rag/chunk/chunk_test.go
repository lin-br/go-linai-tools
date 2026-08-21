package chunk

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/lin-br/go-linai-tools/internal/core/domain"
	"github.com/lin-br/go-linai-tools/internal/core/ports/outbound"
)

// Compile-time interface checks.
var (
	_ Chunker = (*RecursiveChunker)(nil)
	_ Chunker = (*ContextualChunker)(nil)
)

// fakeProvider is a minimal outbound.Provider double for contextual chunk tests.
type fakeProvider struct {
	chatResp *domain.ChatResponse
	chatErr  error
	gotReq   *domain.ChatRequest
}

func (f *fakeProvider) Chat(ctx context.Context, req *domain.ChatRequest) (*domain.ChatResponse, error) {
	f.gotReq = req
	if f.chatErr != nil {
		return nil, f.chatErr
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return f.chatResp, nil
}

func (f *fakeProvider) ChatStream(context.Context, *domain.ChatRequest) (<-chan domain.StreamEvent, error) {
	return nil, nil
}

var _ outbound.Provider = (*fakeProvider)(nil)

func TestDefaultConstants(t *testing.T) {
	if DefaultChunkSize != 512 {
		t.Errorf("DefaultChunkSize = %d, want 512", DefaultChunkSize)
	}
	if DefaultChunkOverlap != 50 {
		t.Errorf("DefaultChunkOverlap = %d, want 50", DefaultChunkOverlap)
	}
}

func TestRecursiveChunker_ShortDocSingleChunk(t *testing.T) {
	c := NewRecursiveChunker(200, 20)
	doc := Document{SourcePath: "n.txt", Content: "a short document that fits in one chunk"}
	chunks, err := c.Split(context.Background(), doc)
	if err != nil {
		t.Fatalf("Split error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("chunks = %d, want 1", len(chunks))
	}
	if chunks[0].Content != doc.Content {
		t.Errorf("content = %q, want %q", chunks[0].Content, doc.Content)
	}
	if chunks[0].Index != 0 {
		t.Errorf("Index = %d, want 0", chunks[0].Index)
	}
}

func TestRecursiveChunker_ParagraphSplits(t *testing.T) {
	c := NewRecursiveChunker(50, 0) // each paragraph < size, adjacent pair > size
	p1 := strings.Repeat("a", 40)
	p2 := strings.Repeat("b", 40)
	p3 := strings.Repeat("c", 40)
	doc := Document{Content: p1 + "\n\n" + p2 + "\n\n" + p3}

	chunks, err := c.Split(context.Background(), doc)
	if err != nil {
		t.Fatalf("Split error: %v", err)
	}
	if len(chunks) != 3 {
		t.Fatalf("chunks = %d, want 3", len(chunks))
	}
	for i, ch := range chunks {
		if ch.Metadata[MetaKeyIndex] != i {
			t.Errorf("chunk %d index meta = %v, want %d", i, ch.Metadata[MetaKeyIndex], i)
		}
		if ch.Metadata[MetaKeyStrategy] != "recursive" {
			t.Errorf("chunk %d strategy = %v, want recursive", i, ch.Metadata[MetaKeyStrategy])
		}
	}
}

func TestRecursiveChunker_OverlapWithinParagraph(t *testing.T) {
	size, overlap := 60, 10
	c := NewRecursiveChunker(size, overlap)
	// One long paragraph of repeated words with no sentence/line breaks.
	words := strings.Repeat("alpha ", 200)
	doc := Document{Content: strings.TrimSpace(words)}

	chunks, err := c.Split(context.Background(), doc)
	if err != nil {
		t.Fatalf("Split error: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("chunks = %d, want >=2", len(chunks))
	}
	for i := 0; i < len(chunks)-1; i++ {
		a := chunks[i].Content
		b := chunks[i+1].Content
		if utf8.RuneCountInString(a) > size {
			t.Errorf("chunk %d len = %d, want <= %d", i, utf8.RuneCountInString(a), size)
		}
		if !sharesOverlap(a, b, overlap) {
			t.Errorf("chunks %d/%d do not share up to %d runes", i, i+1, overlap)
		}
	}
}

// sharesOverlap reports whether some suffix of a (length 1..overlap) equals the
// matching-length prefix of b.
func sharesOverlap(a, b string, overlap int) bool {
	max := overlap
	if ra := utf8.RuneCountInString(a); ra < max {
		max = ra
	}
	if rb := utf8.RuneCountInString(b); rb < max {
		max = rb
	}
	ar := []rune(a)
	br := []rune(b)
	for L := max; L >= 1; L-- {
		if string(ar[len(ar)-L:]) == string(br[:L]) {
			return true
		}
	}
	return false
}

func TestRecursiveChunker_CancelledContext(t *testing.T) {
	c := NewRecursiveChunker(100, 10)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.Split(ctx, Document{Content: "anything"})
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestRecursiveChunker_NonPositiveSizeFallsBack(t *testing.T) {
	c := NewRecursiveChunker(0, -5)
	if c.chunkSize != DefaultChunkSize {
		t.Errorf("chunkSize = %d, want %d", c.chunkSize, DefaultChunkSize)
	}
	if c.chunkOverlap != 0 {
		t.Errorf("chunkOverlap = %d, want 0", c.chunkOverlap)
	}
}

func TestContextualChunker_PrependsSummary(t *testing.T) {
	base := NewRecursiveChunker(20, 0)
	// 3 distinct paragraphs so the base yields 3 chunks.
	doc := Document{
		SourcePath: "doc.txt",
		Content:    strings.Repeat("a", 30) + "\n\n" + strings.Repeat("b", 30) + "\n\n" + strings.Repeat("c", 30),
	}
	prov := &fakeProvider{chatResp: &domain.ChatResponse{Content: "Doc summary."}}
	c := NewContextualChunker(base, prov, "test-model")

	chunks, err := c.Split(context.Background(), doc)
	if err != nil {
		t.Fatalf("Split error: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("chunks = %d, want >=2", len(chunks))
	}
	for i, ch := range chunks {
		if !strings.HasPrefix(ch.Content, "Context: Doc summary.\n\n") {
			t.Errorf("chunk %d missing context prefix: %q", i, ch.Content)
		}
		if ch.Metadata[MetaKeyStrategy] != "contextual" {
			t.Errorf("chunk %d strategy = %v, want contextual", i, ch.Metadata[MetaKeyStrategy])
		}
	}
}

func TestContextualChunker_SummaryPromptForcesShortAnswer(t *testing.T) {
	base := NewRecursiveChunker(200, 0)
	prov := &fakeProvider{chatResp: &domain.ChatResponse{Content: "ok"}}
	c := NewContextualChunker(base, prov, "m")
	_, _ = c.Split(context.Background(), Document{Content: "x"})
	if prov.gotReq == nil {
		t.Fatal("provider did not receive a request")
	}
	if !strings.Contains(prov.gotReq.System, "exactly one sentence") {
		t.Errorf("system prompt = %q, want it to force one sentence", prov.gotReq.System)
	}
	if prov.gotReq.Model != "m" {
		t.Errorf("model = %q, want m", prov.gotReq.Model)
	}
}

func TestContextualChunker_PropagatesProviderError(t *testing.T) {
	base := NewRecursiveChunker(200, 0)
	prov := &fakeProvider{chatErr: errors.New("boom")}
	c := NewContextualChunker(base, prov, "m")
	chunks, err := c.Split(context.Background(), Document{Content: "x"})
	if err == nil {
		t.Fatal("expected provider error to propagate, got nil")
	}
	if len(chunks) != 0 {
		t.Errorf("expected no chunks on error, got %d", len(chunks))
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error %v should wrap boom", err)
	}
}

func TestContextualChunker_CancelledContext(t *testing.T) {
	base := NewRecursiveChunker(200, 0)
	prov := &fakeProvider{chatResp: &domain.ChatResponse{Content: "s"}}
	c := NewContextualChunker(base, prov, "m")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.Split(ctx, Document{Content: "x"})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}
