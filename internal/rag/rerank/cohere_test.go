package rerank

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/lin-br/go-linai-tools/internal/core/ports/outbound"
)

// fakeCohereServer returns an httptest server that decodes a rerankRequest and
// replies with a fixed mapping of candidate index -> relevance score, so the
// test can assert ordering and truncation.
func fakeCohereServer(t *testing.T, hitFlag *bool, scores map[int]float64, badIndex int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*hitFlag = true
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-co" {
			t.Errorf("Authorization = %q, want Bearer sk-co", got)
		}
		var req rerankRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		results := make([]rerankResult, 0, len(scores))
		for idx, sc := range scores {
			results = append(results, rerankResult{Index: idx, RelevanceScore: sc})
		}
		if badIndex >= 0 {
			results = append(results, rerankResult{Index: badIndex, RelevanceScore: 1.0})
		}
		resp := rerankResponse{Results: results}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestRerank_OrdersByScoreDescending(t *testing.T) {
	var hit bool
	srv := fakeCohereServer(t, &hit, map[int]float64{0: 0.9, 1: 0.1, 2: 0.5}, -1)
	defer srv.Close()

	c, _ := NewClient("sk-co", srv.Client())
	c.baseURL = srv.URL

	cands := []Candidate{
		{ID: uuid.New(), Text: "a"},
		{ID: uuid.New(), Text: "b"},
		{ID: uuid.New(), Text: "c"},
	}
	got, err := c.Rerank(context.Background(), "annual budget", cands, 3)
	if err != nil {
		t.Fatalf("Rerank error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	want := []int{0, 2, 1} // 0.9, 0.5, 0.1
	for i, wantIdx := range want {
		if got[i].Index != wantIdx {
			t.Errorf("got[%d].Index = %d, want %d", i, got[i].Index, wantIdx)
		}
		if got[i].ID != cands[got[i].Index].ID {
			t.Errorf("got[%d].ID mismatch", i)
		}
	}
	if !hit {
		t.Error("API was not called")
	}
}

func TestRerank_TopNTruncates(t *testing.T) {
	var hit bool
	srv := fakeCohereServer(t, &hit, map[int]float64{0: 0.9, 1: 0.1, 2: 0.5, 3: 0.4, 4: 0.3}, -1)
	defer srv.Close()

	c, _ := NewClient("sk-co", srv.Client())
	c.baseURL = srv.URL

	cands := make([]Candidate, 5)
	for i := range cands {
		cands[i] = Candidate{ID: uuid.New(), Text: "x"}
	}
	got, err := c.Rerank(context.Background(), "q", cands, 2)
	if err != nil {
		t.Fatalf("Rerank error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Index != 0 || got[1].Index != 2 {
		t.Errorf("order = %d,%d, want 0,2", got[0].Index, got[1].Index)
	}
}

func TestRerank_EmptyCandidatesSkipsAPI(t *testing.T) {
	var hit bool
	srv := fakeCohereServer(t, &hit, map[int]float64{}, -1)
	defer srv.Close()

	c, _ := NewClient("sk-co", srv.Client())
	c.baseURL = srv.URL

	got, err := c.Rerank(context.Background(), "q", nil, 5)
	if err != nil {
		t.Fatalf("Rerank error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
	if hit {
		t.Error("API was called for empty candidates")
	}
}

func TestRerank_OutOfRangeIndexReturnsError(t *testing.T) {
	var hit bool
	srv := fakeCohereServer(t, &hit, map[int]float64{0: 0.5}, 99)
	defer srv.Close()

	c, _ := NewClient("sk-co", srv.Client())
	c.baseURL = srv.URL

	cands := []Candidate{{ID: uuid.New(), Text: "a"}, {ID: uuid.New(), Text: "b"}, {ID: uuid.New(), Text: "c"}}
	_, err := c.Rerank(context.Background(), "q", cands, 3)
	if err == nil {
		t.Fatal("expected error for out-of-range index, got nil")
	}
}

func TestRerank_HTTPErrorPropagated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c, _ := NewClient("sk-co", srv.Client())
	c.baseURL = srv.URL

	_, err := c.Rerank(context.Background(), "q", []Candidate{{ID: uuid.New(), Text: "a"}}, 1)
	if err == nil {
		t.Fatal("expected error from 429, got nil")
	}
	var pe *outbound.ProviderError
	if !errors.As(err, &pe) {
		t.Errorf("expected *outbound.ProviderError, got %T", err)
	}
}

func TestRerank_ContextCancellation(t *testing.T) {
	srv := fakeCohereServer(t, new(bool), map[int]float64{0: 0.5}, -1)
	defer srv.Close()

	c, _ := NewClient("sk-co", srv.Client())
	c.baseURL = srv.URL

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.Rerank(ctx, "q", []Candidate{{ID: uuid.New(), Text: "a"}}, 1)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}

func TestNewClient_MissingAPIKey(t *testing.T) {
	c, err := NewClient("", nil)
	if err == nil {
		t.Fatal("expected error for missing api key")
	}
	if c != nil {
		t.Errorf("expected nil client on error, got %v", c)
	}
}

func TestNewClient_NilHTTPFallsBackToDefault(t *testing.T) {
	c, err := NewClient("sk-co", nil)
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	if c.http != http.DefaultClient {
		t.Error("expected http.DefaultClient fallback")
	}
}

func TestDefaultModelConstant(t *testing.T) {
	if DefaultModel != "rerank-v3.5" {
		t.Errorf("DefaultModel = %q, want rerank-v3.5", DefaultModel)
	}
}

func TestWireRequestSerializes(t *testing.T) {
	req := rerankRequest{Query: "q", Model: "rerank-v3.5", Documents: []string{"a", "b"}, TopN: 2}
	got, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"query":"q","model":"rerank-v3.5","documents":["a","b"],"top_n":2}`
	if string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestWireResponseDeserializes(t *testing.T) {
	body := `{"results":[{"index":2,"relevance_score":0.99}]}`
	var resp rerankResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("Results len = %d, want 1", len(resp.Results))
	}
	if resp.Results[0].Index != 2 {
		t.Errorf("Index = %d, want 2", resp.Results[0].Index)
	}
	if resp.Results[0].RelevanceScore != 0.99 {
		t.Errorf("RelevanceScore = %v, want 0.99", resp.Results[0].RelevanceScore)
	}
}
