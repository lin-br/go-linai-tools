package eval

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"github.com/lin-br/go-linai-tools/internal/core/domain"
	"github.com/lin-br/go-linai-tools/internal/core/ports/outbound"
)

// judgeRubric instructs the model to rate answer relevance on a 1-5 integer
// scale; 5 fully answers the query and matches expected facts, 1 is irrelevant.
const judgeRubric = `You are a strict relevance grader. Rate how well the "generated" text answers the "query" and matches the facts in "expected". Use this scale:
5: fully answers the query and matches expected facts
4: mostly answers, minor gaps
3: partially answers
2: barely relevant
1: irrelevant or wrong
Reply with a single integer from 1 to 5 and nothing else.`

var firstInt = regexp.MustCompile(`-?\d+`)

// Judge scores answer relevance via an outbound.Provider chat call.
type Judge struct {
	provider outbound.Provider
	model    string
}

// NewJudge returns a Judge using the given provider and model.
func NewJudge(provider outbound.Provider, model string) *Judge {
	return &Judge{provider: provider, model: model}
}

// Score sends query/generated/expected to the model and returns the parsed
// integer clamped to [1, 5]. Returns an error if no integer is found.
func (j *Judge) Score(ctx context.Context, query, generated, expected string) (int, error) {
	userMsg := fmt.Sprintf("query: %s\n\nexpected: %s\n\ngenerated: %s", query, expected, generated)
	req := &domain.ChatRequest{
		Model:    j.model,
		System:   judgeRubric,
		Messages: []domain.Message{{Role: domain.MessageRoleUser, Content: userMsg}},
	}
	resp, err := j.provider.Chat(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("eval: judge chat: %w", err)
	}
	score, err := parseScore(resp.Content)
	if err != nil {
		return 0, err
	}
	return clampScore(score), nil
}

// parseScore extracts the first integer from the model response.
func parseScore(s string) (int, error) {
	m := firstInt.FindString(s)
	if m == "" {
		return 0, fmt.Errorf("eval: judge: no score in response %q", s)
	}
	v, err := strconv.Atoi(m)
	if err != nil {
		return 0, fmt.Errorf("eval: judge: parse %q: %w", m, err)
	}
	return v, nil
}

// clampScore constrains a raw score to the [1, 5] range.
func clampScore(v int) int {
	if v < 1 {
		return 1
	}
	if v > 5 {
		return 5
	}
	return v
}
