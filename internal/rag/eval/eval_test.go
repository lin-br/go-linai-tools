package eval

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/lin-br/go-linai-tools/internal/core/domain"
	"github.com/lin-br/go-linai-tools/internal/core/ports/outbound"
	"github.com/lin-br/go-linai-tools/internal/rag/search"
)

func TestPrecisionAtK_OneRelevantInTop3(t *testing.T) {
	a := uuid.New()
	b := uuid.New()
	c := uuid.New()
	got := PrecisionAtK([]uuid.UUID{a}, []uuid.UUID{a, b, c}, 3)
	if got < 1.0/3.0-1e-9 || got > 1.0/3.0+1e-9 {
		t.Errorf("PrecisionAtK = %v, want %v", got, 1.0/3.0)
	}
}

func TestPrecisionAtK_KLargerThanRetrieved(t *testing.T) {
	a := uuid.New()
	got := PrecisionAtK([]uuid.UUID{a}, []uuid.UUID{a}, 5)
	if got < 1.0/5.0-1e-9 || got > 1.0/5.0+1e-9 {
		t.Errorf("PrecisionAtK = %v, want %v", got, 1.0/5.0)
	}
}

func TestPrecisionAtK_ZeroK(t *testing.T) {
	if got := PrecisionAtK([]uuid.UUID{uuid.New()}, []uuid.UUID{uuid.New()}, 0); got != 0 {
		t.Errorf("PrecisionAtK = %v, want 0", got)
	}
}

func TestRecallAtK_TwoRelevantOneRetrieved(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	c := uuid.New()
	got := RecallAtK([]uuid.UUID{a, b}, []uuid.UUID{a, c, uuid.New()}, 3)
	if got < 0.5-1e-9 || got > 0.5+1e-9 {
		t.Errorf("RecallAtK = %v, want 0.5", got)
	}
}

func TestRecallAtK_EmptyRelevant(t *testing.T) {
	if got := RecallAtK(nil, []uuid.UUID{uuid.New()}, 3); got != 0 {
		t.Errorf("RecallAtK = %v, want 0", got)
	}
}

func TestMRR_FirstPositionRelevant(t *testing.T) {
	a := uuid.New()
	got := MRR([]uuid.UUID{a}, []uuid.UUID{a, uuid.New()})
	if got != 1.0 {
		t.Errorf("MRR = %v, want 1.0", got)
	}
}

func TestMRR_SecondPositionRelevant(t *testing.T) {
	a := uuid.New()
	got := MRR([]uuid.UUID{a}, []uuid.UUID{uuid.New(), a})
	if got < 0.5-1e-9 || got > 0.5+1e-9 {
		t.Errorf("MRR = %v, want 0.5", got)
	}
}

func TestMRR_NoRelevant(t *testing.T) {
	got := MRR([]uuid.UUID{uuid.New()}, []uuid.UUID{uuid.New(), uuid.New()})
	if got != 0 {
		t.Errorf("MRR = %v, want 0", got)
	}
}

// fakeJudgeProvider doubles outbound.Provider for judge tests.
type fakeJudgeProvider struct {
	resp *domain.ChatResponse
	err  error
}

func (f *fakeJudgeProvider) Chat(_ context.Context, _ *domain.ChatRequest) (*domain.ChatResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func (f *fakeJudgeProvider) ChatStream(context.Context, *domain.ChatRequest) (<-chan domain.StreamEvent, error) {
	return nil, nil
}

var _ outbound.Provider = (*fakeJudgeProvider)(nil)

func TestJudge_ParsesAndClamps(t *testing.T) {
	tests := []struct {
		name    string
		resp    string
		want    int
		wantErr bool
	}{
		{"plain integer", "4", 4, false},
		{"integer in explanation", "Score: 3 because the answer is partially correct", 3, false},
		{"too high clamps to 5", "10", 5, false},
		{"too low clamps to 1", "0", 1, false},
		{"negative clamps to 1", "-2", 1, false},
		{"no integer errors", "no score here", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := NewJudge(&fakeJudgeProvider{resp: &domain.ChatResponse{Content: tt.resp}}, "m")
			got, err := j.Score(context.Background(), "q", "gen", "exp")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Score error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Score = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestJudge_PropagatesProviderError(t *testing.T) {
	j := NewJudge(&fakeJudgeProvider{err: errors.New("boom")}, "m")
	_, err := j.Score(context.Background(), "q", "g", "e")
	if err == nil {
		t.Fatal("expected provider error to propagate")
	}
}

// fakeSearcher maps queries to preset results.
type fakeSearcher struct {
	byQuery map[string][]search.Result
	err     error
}

func (f *fakeSearcher) Search(_ context.Context, query string, _ []float32, _ int) ([]search.Result, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byQuery[query], nil
}

var _ search.Searcher = (*fakeSearcher)(nil)

func TestEvaluator_AveragesAndNilJudge(t *testing.T) {
	a := uuid.New()
	b := uuid.New()
	c := uuid.New()
	d := uuid.New() // distractor

	s := &fakeSearcher{byQuery: map[string][]search.Result{
		"q1": {{ID: a}, {ID: d}},                       // expected a at pos 1
		"q2": {{ID: d}, {ID: b}},                       // expected b at pos 2
		"q3": {{ID: d}, {ID: d}},                       // expected c not retrieved
	}}
	dataset := &Dataset{Examples: []Example{
		{Query: "q1", ExpectedChunkID: a},
		{Query: "q2", ExpectedChunkID: b},
		{Query: "q3", ExpectedChunkID: c},
	}}
	ev := NewEvaluator(s, nil, 3)

	report, err := ev.Run(context.Background(), dataset)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if len(report.Examples) != 3 {
		t.Fatalf("examples = %d, want 3", len(report.Examples))
	}
	// q1: P=1/3, R=1, MRR=1 ; q2: P=1/3, R=1, MRR=0.5 ; q3: 0,0,0
	wantP := (1.0/3.0 + 1.0/3.0 + 0) / 3
	wantR := (1.0 + 1.0 + 0) / 3
	wantMRR := (1.0 + 0.5 + 0) / 3
	if !closeEnough(report.AvgPrecisionAtK, wantP) {
		t.Errorf("AvgPrecisionAtK = %v, want %v", report.AvgPrecisionAtK, wantP)
	}
	if !closeEnough(report.AvgRecallAtK, wantR) {
		t.Errorf("AvgRecallAtK = %v, want %v", report.AvgRecallAtK, wantR)
	}
	if !closeEnough(report.AvgMRR, wantMRR) {
		t.Errorf("AvgMRR = %v, want %v", report.AvgMRR, wantMRR)
	}
	if report.AvgJudgeScore != 0 {
		t.Errorf("AvgJudgeScore = %v, want 0 (nil judge)", report.AvgJudgeScore)
	}
	for _, ex := range report.Examples {
		if ex.JudgeScore != nil {
			t.Error("JudgeScore should be nil with nil judge")
		}
	}
}

func TestEvaluator_WithJudge(t *testing.T) {
	a := uuid.New()
	s := &fakeSearcher{byQuery: map[string][]search.Result{
		"q": {{ID: a, Content: "answer text"}},
	}}
	judge := NewJudge(&fakeJudgeProvider{resp: &domain.ChatResponse{Content: "4"}}, "m")
	ev := NewEvaluator(s, judge, 3)

	report, err := ev.Run(context.Background(), &Dataset{Examples: []Example{
		{Query: "q", ExpectedChunkID: a, ExpectedAnswer: "expected"},
	}})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if len(report.Examples) != 1 || report.Examples[0].JudgeScore == nil {
		t.Fatal("judge score not recorded")
	}
	if *report.Examples[0].JudgeScore != 4 {
		t.Errorf("JudgeScore = %d, want 4", *report.Examples[0].JudgeScore)
	}
	if report.AvgJudgeScore != 4 {
		t.Errorf("AvgJudgeScore = %v, want 4", report.AvgJudgeScore)
	}
}

func TestEvaluator_SearchErrorAborts(t *testing.T) {
	ev := NewEvaluator(&fakeSearcher{err: errors.New("search down")}, nil, 3)
	_, err := ev.Run(context.Background(), &Dataset{Examples: []Example{{Query: "q"}}})
	if err == nil {
		t.Fatal("expected search error to abort Run")
	}
}

func TestLoadDataset(t *testing.T) {
	path := "testdata/golden.jsonl"
	ds, err := LoadDataset(path)
	if err != nil {
		t.Fatalf("LoadDataset error: %v", err)
	}
	if len(ds.Examples) < 20 {
		t.Errorf("examples = %d, want >= 20", len(ds.Examples))
	}
}

func TestLoadDataset_MissingFile(t *testing.T) {
	if _, err := LoadDataset("testdata/does-not-exist.jsonl"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func closeEnough(a, b float64) bool {
	return a > b-1e-9 && a < b+1e-9
}
