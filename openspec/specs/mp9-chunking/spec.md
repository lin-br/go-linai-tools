# Purpose

TBD

# Requirements

## Requirement: Chunker interface and recursive character splitter package exists under internal/rag/chunk

The system SHALL provide a package `internal/rag/chunk` that exports a `Chunker` interface and at least one implementation, `RecursiveChunker`. A `Chunk` type SHALL contain `Content string`, `Index int`, and `Metadata map[string]any`. The `Chunker` interface SHALL have the single method `Split(ctx context.Context, doc Document) ([]Chunk, error)`.

### Scenario: RecursiveChunker implements Chunker

- **WHEN** code declares `var _ chunk.Chunker = (*chunk.RecursiveChunker)(nil)`
- **THEN** the program SHALL compile

### Scenario: Document struct carries source path and raw text

- **WHEN** a `chunk.Document{SourcePath: "notes.txt", Content: "..."}` is created
- **THEN** `SourcePath` and `Content` SHALL be accessible fields

## Requirement: RecursiveChunker splits text by separators with configurable size and overlap

The system SHALL implement `chunk.NewRecursiveChunker(chunkSize, chunkOverlap int) *RecursiveChunker`. The splitter SHALL attempt separators in order: paragraph (`\n\n`), line (`\n`), sentence (`. `), word (` `), character (`""`). It SHALL keep chunks under `chunkSize` runes (measured by `utf8.RuneCountInString`) and overlap adjacent chunks by up to `chunkOverlap` runes when splitting further is required.

### Scenario: Short document stays as one chunk

- **WHEN** a 50-rune document is split with `chunkSize=200` and `chunkOverlap=20`
- **THEN** the result SHALL be exactly one chunk whose content equals the input

### Scenario: Long document splits by paragraphs first

- **WHEN** a document with three paragraphs separated by `\n\n` is split with `chunkSize` smaller than the full document but larger than each paragraph
- **THEN** the result SHALL contain three chunks, one per paragraph

### Scenario: Overlap is applied when splitting within a paragraph

- **WHEN** a single paragraph longer than `chunkSize` is split with `chunkOverlap=20`
- **THEN** adjacent chunks SHALL share up to 20 runes of trailing/leading text

## Requirement: Contextual chunking prepends a one-sentence document summary

The system SHALL implement `chunk.NewContextualChunker(base Chunker, provider outbound.Provider, model string) *ContextualChunker`. This chunker SHALL first call `base.Split(ctx, doc)` to obtain raw chunks. Before returning, it SHALL generate a one-sentence summary of the full document by calling `provider.Chat(ctx, req)` with a system prompt instructing the model to return only a one-sentence summary. It SHALL prepend `"Context: " + summary + "\n\n"` to each chunk's `Content`.

### Scenario: Contextual summary is prepended to every chunk

- **WHEN** a 3-chunk document is processed by `ContextualChunker`
- **THEN** every returned chunk's `Content` SHALL start with the generated context summary

### Scenario: Summary prompt forces a short answer

- **WHEN** `ContextualChunker` builds the chat request for the summary
- **THEN** the system prompt SHALL instruct the model to produce exactly one sentence and no other text

### Scenario: Contextual chunker propagates provider errors

- **WHEN** the provider returns an error during summary generation
- **THEN** `ContextualChunker.Split` SHALL return that error and no chunks

## Requirement: Metadata carries split strategy and chunk index

The system SHALL populate `Chunk.Metadata` with at least the keys `"index"` (chunk position) and `"strategy"` (e.g., `"recursive"` or `"contextual"`). Additional keys are allowed but SHALL NOT overwrite these two.

### Scenario: Metadata contains index and strategy

- **WHEN** `RecursiveChunker.Split` returns chunks
- **THEN** each chunk's `Metadata["index"]` SHALL equal its zero-based position and `Metadata["strategy"]` SHALL equal `"recursive"`

## Requirement: Chunker respects context cancellation

The `RecursiveChunker.Split` method SHALL accept `context.Context` as its first argument and return immediately when the context is cancelled. `ContextualChunker.Split` SHALL additionally propagate cancellation to `provider.Chat`.

### Scenario: Recursive splitter returns context error

- **WHEN** `RecursiveChunker.Split` is called with a cancelled context
- **THEN** it SHALL return `context.Canceled` (or the wrapped context error) and no chunks

## Requirement: Default chunk size is reasonable

The system SHALL export `const DefaultChunkSize = 512` and `const DefaultChunkOverlap = 50` from `internal/rag/chunk`.

### Scenario: Default constants exist

- **WHEN** code references `chunk.DefaultChunkSize` and `chunk.DefaultChunkOverlap`
- **THEN** their values SHALL be `512` and `50` respectively
