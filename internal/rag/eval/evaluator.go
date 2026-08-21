package eval

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/lin-br/go-linai-tools/internal/rag/search"
)

// ExampleReport holds per-example metrics and the retrieved IDs.
type ExampleReport struct {
	Query           string      `json:"query"`
	ExpectedChunkID uuid.UUID   `json:"expected_chunk_id"`
	PrecisionAtK    float64     `json:"precision_at_k"`
	RecallAtK       float64     `json:"recall_at_k"`
	MRR             float64     `json:"mrr"`
	JudgeScore      *int        `json:"judge_score,omitempty"`
	RetrievedIDs    []uuid.UUID `json:"retrieved_ids"`
}

// Report aggregates per-example metrics into averages.
type Report struct {
	AvgPrecisionAtK float64         `json:"avg_precision_at_k"`
	AvgRecallAtK    float64         `json:"avg_recall_at_k"`
	AvgMRR          float64         `json:"avg_mrr"`
	AvgJudgeScore   float64         `json:"avg_judge_score"`
	Examples        []ExampleReport `json:"examples"`
}

// Evaluator runs the retrieval pipeline over a Dataset and produces a Report.
type Evaluator struct {
	searcher search.Searcher
	judge    *Judge
	topK     int
}

// NewEvaluator wires a searcher and optional judge. When judge is nil, no LLM
// scoring is performed.
func NewEvaluator(searcher search.Searcher, judge *Judge, topK int) *Evaluator {
	if topK <= 0 {
		topK = 5
	}
	return &Evaluator{searcher: searcher, judge: judge, topK: topK}
}

// Run evaluates every example and aggregates the results. Search errors abort
// the run; judge errors are non-fatal and recorded as a nil score.
func (e *Evaluator) Run(ctx context.Context, dataset *Dataset) (*Report, error) {
	report := &Report{Examples: make([]ExampleReport, 0, len(dataset.Examples))}
	var sumP, sumR, sumMRR float64
	var judgeSum float64
	var judgeCount int

	for _, ex := range dataset.Examples {
		results, err := e.searcher.Search(ctx, ex.Query, nil, e.topK)
		if err != nil {
			return nil, fmt.Errorf("eval: search for %q: %w", ex.Query, err)
		}
		retrieved := make([]uuid.UUID, len(results))
		for i, r := range results {
			retrieved[i] = r.ID
		}
		relevant := []uuid.UUID{ex.ExpectedChunkID}

		er := ExampleReport{
			Query:           ex.Query,
			ExpectedChunkID: ex.ExpectedChunkID,
			PrecisionAtK:    PrecisionAtK(relevant, retrieved, e.topK),
			RecallAtK:       RecallAtK(relevant, retrieved, e.topK),
			MRR:             MRR(relevant, retrieved),
			RetrievedIDs:    retrieved,
		}

		if e.judge != nil {
			retrievedText := joinContents(results)
			if score, err := e.judge.Score(ctx, ex.Query, retrievedText, ex.ExpectedAnswer); err == nil {
				er.JudgeScore = &score
				judgeSum += float64(score)
				judgeCount++
			}
		}

		sumP += er.PrecisionAtK
		sumR += er.RecallAtK
		sumMRR += er.MRR
		report.Examples = append(report.Examples, er)
	}

	n := len(dataset.Examples)
	if n > 0 {
		report.AvgPrecisionAtK = sumP / float64(n)
		report.AvgRecallAtK = sumR / float64(n)
		report.AvgMRR = sumMRR / float64(n)
	}
	if judgeCount > 0 {
		report.AvgJudgeScore = judgeSum / float64(judgeCount)
	}
	return report, nil
}

// joinContents concatenates retrieved result contents for the judge.
func joinContents(results []search.Result) string {
	parts := make([]string, 0, len(results))
	for _, r := range results {
		parts = append(parts, r.Content)
	}
	return strings.Join(parts, "\n---\n")
}
