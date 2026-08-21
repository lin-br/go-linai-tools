package http_clients

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/lin-br/go-linai-tools/internal/core/domain"
	"github.com/lin-br/go-linai-tools/internal/core/ports/outbound"
)

// fakeRerankServer returns an httptest server that decodes a rerankRequest and
// replies with results mapping candidate index -> relevance score.
func fakeRerankServer(t *testing.T, status int, scores map[int]float64, badIndex int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("Authorization = %q, want Bearer sk-test", got)
		}
		if got := r.Header.Get("HTTP-Referer"); got != "lin.com.br" {
			t.Errorf("HTTP-Referer = %q, want lin.com.br", got)
		}
		if got := r.Header.Get("X-OpenRouter-Title"); got != "lin.com.br" {
			t.Errorf("X-OpenRouter-Title = %q, want lin.com.br", got)
		}
		var req rerankRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if status != 0 {
			http.Error(w, "boom", status)
			return
		}
		results := make([]rerankResult, 0, len(scores))
		for idx, sc := range scores {
			results = append(results, rerankResult{
				Document:       rerankDocument{Text: req.Documents[idx].Text},
				Index:          idx,
				RelevanceScore: sc,
			})
		}
		if badIndex >= 0 {
			results = append(results, rerankResult{
				Document:       rerankDocument{Text: "bad"},
				Index:          badIndex,
				RelevanceScore: 1.0,
			})
		}
		resp := rerankResponse{
			ID:      "gen-rerank-test",
			Model:   req.Model,
			Results: results,
			Usage:   rerankUsage{TotalTokens: 42},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestRerank_ReturnsAllResults(t *testing.T) {
	srv := fakeRerankServer(t, 0, map[int]float64{0: 0.9, 1: 0.1, 2: 0.5}, -1)
	defer srv.Close()

	p := NewOpenRouterRerankProvider("sk-test")
	p.client = srv.Client()
	p.baseURL = srv.URL

	resp, err := p.Rerank(context.Background(), &domain.RerankRequest{
		Model:     "cohere/rerank-v3.5",
		Query:     "capital of France",
		Documents: []domain.RerankDocument{{Text: "a"}, {Text: "b"}, {Text: "c"}},
		TopN:      3,
	})
	if err != nil {
		t.Fatalf("Rerank error: %v", err)
	}
	if len(resp.Results) != 3 {
		t.Fatalf("len(Results) = %d, want 3", len(resp.Results))
	}
	if resp.Model != "cohere/rerank-v3.5" {
		t.Errorf("Model = %q, want cohere/rerank-v3.5", resp.Model)
	}
}

func TestRerank_RelevanceScorePreserved(t *testing.T) {
	srv := fakeRerankServer(t, 0, map[int]float64{0: 0.98, 1: 0.42}, -1)
	defer srv.Close()

	p := NewOpenRouterRerankProvider("sk-test")
	p.client = srv.Client()
	p.baseURL = srv.URL

	resp, err := p.Rerank(context.Background(), &domain.RerankRequest{
		Model:     "cohere/rerank-v3.5",
		Query:     "q",
		Documents: []domain.RerankDocument{{Text: "a"}, {Text: "b"}},
		TopN:      2,
	})
	if err != nil {
		t.Fatalf("Rerank error: %v", err)
	}
	scores := map[int]float64{}
	for _, r := range resp.Results {
		scores[r.Index] = r.RelevanceScore
	}
	if scores[0] != 0.98 {
		t.Errorf("Results[0].RelevanceScore = %v, want 0.98", scores[0])
	}
	if scores[1] != 0.42 {
		t.Errorf("Results[1].RelevanceScore = %v, want 0.42", scores[1])
	}
}

func TestRerank_PassesModelQueryTopN(t *testing.T) {
	var gotReq rerankRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotReq)
		resp := rerankResponse{
			ID:    "x",
			Model: gotReq.Model,
			Results: []rerankResult{{
				Document:       rerankDocument{Text: gotReq.Documents[0].Text},
				Index:          0,
				RelevanceScore: 0.9,
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewOpenRouterRerankProvider("sk-test")
	p.client = srv.Client()
	p.baseURL = srv.URL

	_, err := p.Rerank(context.Background(), &domain.RerankRequest{
		Model:     "cohere/rerank-v3.5",
		Query:     "What is the capital of France?",
		Documents: []domain.RerankDocument{{Text: "Paris is the capital of France."}},
		TopN:      3,
	})
	if err != nil {
		t.Fatalf("Rerank error: %v", err)
	}
	if gotReq.Model != "cohere/rerank-v3.5" {
		t.Errorf("Model = %q, want cohere/rerank-v3.5", gotReq.Model)
	}
	if gotReq.Query != "What is the capital of France?" {
		t.Errorf("Query = %q, want the question", gotReq.Query)
	}
	if gotReq.TopN != 3 {
		t.Errorf("TopN = %d, want 3", gotReq.TopN)
	}
	if len(gotReq.Documents) != 1 {
		t.Fatalf("Documents len = %d, want 1", len(gotReq.Documents))
	}
	if gotReq.Documents[0].Text != "Paris is the capital of France." {
		t.Errorf("Documents[0].Text = %q, want the doc", gotReq.Documents[0].Text)
	}
}

func TestRerank_DocumentImagePreserved(t *testing.T) {
	var gotReq rerankRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotReq)
		resp := rerankResponse{Results: []rerankResult{{Index: 0, RelevanceScore: 0.5, Document: gotReq.Documents[0]}}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewOpenRouterRerankProvider("sk-test")
	p.client = srv.Client()
	p.baseURL = srv.URL

	_, err := p.Rerank(context.Background(), &domain.RerankRequest{
		Model:     "cohere/rerank-v3.5",
		Query:     "q",
		Documents: []domain.RerankDocument{{Text: "doc text", Image: "https://example.com/img.png"}},
		TopN:      1,
	})
	if err != nil {
		t.Fatalf("Rerank error: %v", err)
	}
	if gotReq.Documents[0].Text != "doc text" {
		t.Errorf("Text = %q, want 'doc text'", gotReq.Documents[0].Text)
	}
	if gotReq.Documents[0].Image != "https://example.com/img.png" {
		t.Errorf("Image = %q, want the URL", gotReq.Documents[0].Image)
	}
}

func TestRerank_PreservesUsage(t *testing.T) {
	srv := fakeRerankServer(t, 0, map[int]float64{0: 0.9}, -1)
	defer srv.Close()

	p := NewOpenRouterRerankProvider("sk-test")
	p.client = srv.Client()
	p.baseURL = srv.URL

	resp, err := p.Rerank(context.Background(), &domain.RerankRequest{
		Model:     "cohere/rerank-v3.5",
		Query:     "q",
		Documents: []domain.RerankDocument{{Text: "a"}},
		TopN:      1,
	})
	if err != nil {
		t.Fatalf("Rerank error: %v", err)
	}
	if resp.Usage.TotalTokens != 42 {
		t.Errorf("TotalTokens = %d, want 42", resp.Usage.TotalTokens)
	}
}

func TestRerank_ContextCancellation(t *testing.T) {
	srv := fakeRerankServer(t, 0, map[int]float64{0: 0.5}, -1)
	defer srv.Close()

	p := NewOpenRouterRerankProvider("sk-test")
	p.client = srv.Client()
	p.baseURL = srv.URL

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := p.Rerank(ctx, &domain.RerankRequest{
		Model:     "cohere/rerank-v3.5",
		Query:     "q",
		Documents: []domain.RerankDocument{{Text: "a"}},
		TopN:      1,
	})
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestRerank_HTTPErrorPropagated(t *testing.T) {
	srv := fakeRerankServer(t, http.StatusInternalServerError, nil, -1)
	defer srv.Close()

	p := NewOpenRouterRerankProvider("sk-test")
	p.client = srv.Client()
	p.baseURL = srv.URL

	_, err := p.Rerank(context.Background(), &domain.RerankRequest{
		Model:     "cohere/rerank-v3.5",
		Query:     "q",
		Documents: []domain.RerankDocument{{Text: "a"}},
		TopN:      1,
	})
	if err == nil {
		t.Fatal("expected error from 500 response, got nil")
	}
	var pe *outbound.ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *outbound.ProviderError, got %T (%v)", err, err)
	}
	if pe.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want %d", pe.StatusCode, http.StatusInternalServerError)
	}
}

func TestRerank_RateLimitPropagated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	p := NewOpenRouterRerankProvider("sk-test")
	p.client = srv.Client()
	p.baseURL = srv.URL

	_, err := p.Rerank(context.Background(), &domain.RerankRequest{
		Model:     "cohere/rerank-v3.5",
		Query:     "q",
		Documents: []domain.RerankDocument{{Text: "a"}},
		TopN:      1,
	})
	if err == nil {
		t.Fatal("expected error from 429, got nil")
	}
	var pe *outbound.ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *outbound.ProviderError, got %T", err)
	}
	if pe.StatusCode != http.StatusTooManyRequests {
		t.Errorf("StatusCode = %d, want %d", pe.StatusCode, http.StatusTooManyRequests)
	}
}

func TestRerankRequestSerializes(t *testing.T) {
	req := rerankRequest{
		Model:     "cohere/rerank-v3.5",
		Query:     "What is the capital of France?",
		Documents: []rerankDocument{{Text: "Paris is the capital of France."}},
		TopN:      3,
	}
	got, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"model":"cohere/rerank-v3.5","query":"What is the capital of France?","documents":[{"text":"Paris is the capital of France."}],"top_n":3}`
	if string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestRerankResponseDeserializes(t *testing.T) {
	body := `{"id":"gen-rerank-123","model":"cohere/rerank-v3.5","provider":"Cohere","results":[{"document":{"text":"Paris is the capital of France."},"index":0,"relevance_score":0.98}],"usage":{"search_units":1,"total_tokens":150}}`
	var resp rerankResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.ID != "gen-rerank-123" {
		t.Errorf("ID = %q, want gen-rerank-123", resp.ID)
	}
	if resp.Model != "cohere/rerank-v3.5" {
		t.Errorf("Model = %q, want cohere/rerank-v3.5", resp.Model)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("Results len = %d, want 1", len(resp.Results))
	}
	if resp.Results[0].Index != 0 {
		t.Errorf("Index = %d, want 0", resp.Results[0].Index)
	}
	if resp.Results[0].RelevanceScore != 0.98 {
		t.Errorf("RelevanceScore = %v, want 0.98", resp.Results[0].RelevanceScore)
	}
	if resp.Results[0].Document.Text != "Paris is the capital of France." {
		t.Errorf("Document.Text = %q, want the doc", resp.Results[0].Document.Text)
	}
	if resp.Usage.SearchUnits != 1 {
		t.Errorf("SearchUnits = %d, want 1", resp.Usage.SearchUnits)
	}
	if resp.Usage.TotalTokens != 150 {
		t.Errorf("TotalTokens = %d, want 150", resp.Usage.TotalTokens)
	}
}

func TestNewOpenRouterRerankProvider(t *testing.T) {
	p := NewOpenRouterRerankProvider("sk-test")
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	if p.apiKey != "sk-test" {
		t.Errorf("apiKey = %q, want sk-test", p.apiKey)
	}
}

func TestOpenRouterRerankProvider_ImplementsReranker(t *testing.T) {
	var _ outbound.Reranker = (*OpenRouterRerankProvider)(nil)
}

func TestFromRerankWire_EmptyResults(t *testing.T) {
	resp := fromRerankWire(&rerankResponse{ID: "x", Model: "m"})
	if resp.ID != "x" {
		t.Errorf("ID = %q, want x", resp.ID)
	}
	if len(resp.Results) != 0 {
		t.Errorf("Results len = %d, want 0", len(resp.Results))
	}
}

func TestFromEmbeddingsWire_EmptyData(t *testing.T) {
	resp := fromEmbeddingsWire(&embeddingsResponse{Model: "m"})
	if resp.Model != "m" {
		t.Errorf("Model = %q, want m", resp.Model)
	}
	if len(resp.Data) != 0 {
		t.Errorf("Data len = %d, want 0", len(resp.Data))
	}
}

func TestToEmbeddingsWire_RoundTrips(t *testing.T) {
	req := &domain.EmbeddingRequest{
		Model:          "voyage/voyage-3-large",
		Input:          []string{"a", "b"},
		Dimensions:     512,
		EncodingFormat: "float",
		InputType:      "document",
	}
	wire := toEmbeddingsWire(req)
	if wire.Model != req.Model {
		t.Errorf("Model = %q, want %q", wire.Model, req.Model)
	}
	if !reflect.DeepEqual(wire.Input, req.Input) {
		t.Errorf("Input = %v, want %v", wire.Input, req.Input)
	}
	if wire.Dimensions != req.Dimensions {
		t.Errorf("Dimensions = %d, want %d", wire.Dimensions, req.Dimensions)
	}
}
