package eval

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
)

// Example is one golden-set entry: a query, the chunk ID expected to be
// retrieved, and an optional expected answer for LLM-as-judge scoring.
type Example struct {
	Query            string    `json:"query"`
	ExpectedChunkID  uuid.UUID `json:"expected_chunk_id"`
	ExpectedAnswer   string    `json:"expected_answer,omitempty"`
}

// Dataset is a collection of golden Example values.
type Dataset struct {
	Examples []Example `json:"examples"`
}

// LoadDataset reads a .jsonl file (one JSON object per line) and returns a
// Dataset. Blank lines are skipped; invalid JSON reports the offending line.
func LoadDataset(path string) (*Dataset, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("eval: open dataset %q: %w", path, err)
	}
	defer f.Close()

	var examples []Example
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ex Example
		if err := json.Unmarshal([]byte(line), &ex); err != nil {
			return nil, fmt.Errorf("eval: dataset line %d: %w", lineNo, err)
		}
		examples = append(examples, ex)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("eval: read dataset: %w", err)
	}
	return &Dataset{Examples: examples}, nil
}
