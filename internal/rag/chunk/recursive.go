package chunk

import (
	"context"
	"strings"
	"unicode/utf8"
)

// separators is the cascade used by RecursiveChunker: paragraph, line,
// sentence, word, character.
var separators = []string{"\n\n", "\n", ". ", " ", ""}

// RecursiveChunker splits text by trying each separator in order, recursing
// into oversized pieces with the next separator until chunks fit chunkSize
// runes. Adjacent chunks overlap by up to chunkOverlap runes.
type RecursiveChunker struct {
	chunkSize    int
	chunkOverlap int
}

// Compile-time check: RecursiveChunker implements Chunker.
var _ Chunker = (*RecursiveChunker)(nil)

// NewRecursiveChunker returns a splitter. Non-positive chunkSize falls back to
// DefaultChunkSize; negative chunkOverlap is treated as zero.
func NewRecursiveChunker(chunkSize, chunkOverlap int) *RecursiveChunker {
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	if chunkOverlap < 0 {
		chunkOverlap = 0
	}
	return &RecursiveChunker{chunkSize: chunkSize, chunkOverlap: chunkOverlap}
}

// Split chunks doc.Content, populating each Chunk's Index and strategy metadata.
// A cancelled context is respected before any work begins.
func (c *RecursiveChunker) Split(ctx context.Context, doc Document) ([]Chunk, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	pieces := c.splitText(doc.Content, separators)
	merged := c.merge(pieces)
	chunks := make([]Chunk, len(merged))
	for i, m := range merged {
		chunks[i] = Chunk{
			Content:    m,
			SourcePath: doc.SourcePath,
			Index:      i,
			Metadata:   setMeta(map[string]any{}, i, "recursive"),
		}
	}
	return chunks, nil
}

// splitText recursively breaks text into pieces no longer than chunkSize runes
// using the separator cascade.
func (c *RecursiveChunker) splitText(text string, seps []string) []string {
	if utf8.RuneCountInString(text) <= c.chunkSize {
		return []string{text}
	}
	sep, rest := c.pickSeparator(text, seps)
	var pieces []string
	for _, p := range splitKeep(text, sep) {
		if strings.TrimSpace(p) == "" {
			continue
		}
		if utf8.RuneCountInString(p) <= c.chunkSize {
			pieces = append(pieces, p)
			continue
		}
		pieces = append(pieces, c.splitText(p, rest)...)
	}
	return pieces
}

// pickSeparator returns the first separator present in text and the remaining
// cascade to recurse with. The empty separator is the guaranteed fallback.
func (c *RecursiveChunker) pickSeparator(text string, seps []string) (string, []string) {
	for i, s := range seps {
		if s == "" {
			return "", seps[i+1:]
		}
		if strings.Contains(text, s) {
			return s, seps[i+1:]
		}
	}
	return "", nil
}

// merge joins pieces into chunks of at most chunkSize runes, prepending up to
// chunkOverlap runes from the previous chunk's tail to the next chunk.
func (c *RecursiveChunker) merge(pieces []string) []string {
	if len(pieces) == 0 {
		return nil
	}
	size := c.chunkSize
	overlap := c.chunkOverlap
	var chunks []string
	cur := pieces[0]
	for i := 1; i < len(pieces); i++ {
		p := pieces[i]
		if utf8.RuneCountInString(cur)+utf8.RuneCountInString(p) <= size {
			cur += p
			continue
		}
		chunks = append(chunks, cur)
		tail := takeLastRunes(cur, overlap)
		// cap overlap so the next chunk still fits within size
		room := size - utf8.RuneCountInString(p)
		if room < 0 {
			room = 0
		}
		if utf8.RuneCountInString(tail) > room {
			tail = takeFirstRunes(tail, room)
		}
		cur = tail + p
	}
	chunks = append(chunks, cur)
	return chunks
}

// splitKeep splits s by sep, appending sep to every non-final piece so the
// separator is preserved. For sep=="" it splits into individual runes.
func splitKeep(s, sep string) []string {
	if sep == "" {
		return splitRunes(s)
	}
	raw := strings.Split(s, sep)
	out := make([]string, 0, len(raw))
	for i, p := range raw {
		piece := p
		if i < len(raw)-1 {
			piece = p + sep
		}
		if piece != "" {
			out = append(out, piece)
		}
	}
	return out
}

func splitRunes(s string) []string {
	if s == "" {
		return nil
	}
	runes := []rune(s)
	out := make([]string, 0, len(runes))
	for _, r := range runes {
		out = append(out, string(r))
	}
	return out
}

func takeLastRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[len(runes)-n:])
}

func takeFirstRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
