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

// fakeEmbeddingsServer returns an httptest.Server that decodes an
// embeddingsRequest and echoes back an embeddingsResponse whose vectors mirror
// the input count. Each embedding is built by builder(i, n).
func fakeEmbeddingsServer(t *testing.T, status int, builder func(i, n int) []float32) *httptest.Server {
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
		var req embeddingsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if status != 0 {
			http.Error(w, "boom", status)
			return
		}
		data := make([]embeddingsData, len(req.Input))
		for i := range req.Input {
			data[i] = embeddingsData{Embedding: builder(i, len(req.Input)), Index: i, Object: "embedding"}
		}
		resp := embeddingsResponse{
			Data:   data,
			Model:  req.Model,
			Object: "list",
			Usage:  embeddingsUsage{PromptTokens: len(req.Input), TotalTokens: len(req.Input)},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func embVec(i, n int) []float32 {
	return []float32{float32(i + 1), float32(i + 2), float32(i + 3), float32(i + 4)}
}

func TestEmbed_BatchingAndOrder(t *testing.T) {
	srv := fakeEmbeddingsServer(t, 0, embVec)
	defer srv.Close()

	p := NewOpenRouterEmbeddingsProvider("sk-test")
	p.client = srv.Client()
	p.baseURL = srv.URL

	resp, err := p.Embed(context.Background(), &domain.EmbeddingRequest{
		Model: "voyage/voyage-3-large",
		Input: []string{"a", "b", "c"},
	})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	if len(resp.Data) != 3 {
		t.Fatalf("len(Data) = %d, want 3", len(resp.Data))
	}
	for i, d := range resp.Data {
		want := embVec(i, 3)
		if !reflect.DeepEqual(d.Embedding, want) {
			t.Errorf("Data[%d].Embedding = %v, want %v", i, d.Embedding, want)
		}
		if d.Index != i {
			t.Errorf("Data[%d].Index = %d, want %d", i, d.Index, i)
		}
	}
}

func TestEmbed_SingleInput(t *testing.T) {
	srv := fakeEmbeddingsServer(t, 0, embVec)
	defer srv.Close()

	p := NewOpenRouterEmbeddingsProvider("sk-test")
	p.client = srv.Client()
	p.baseURL = srv.URL

	resp, err := p.Embed(context.Background(), &domain.EmbeddingRequest{
		Model: "openai/text-embedding-3-small",
		Input: []string{"single sentence"},
	})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("len(Data) = %d, want 1", len(resp.Data))
	}
	if len(resp.Data[0].Embedding) != 4 {
		t.Fatalf("embedding len = %d, want 4", len(resp.Data[0].Embedding))
	}
}

func TestEmbed_PassesModelAndOptionalFields(t *testing.T) {
	var gotReq embeddingsRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotReq)
		resp := embeddingsResponse{
			Data:   []embeddingsData{{Embedding: []float32{0.1}, Index: 0, Object: "embedding"}},
			Model:  gotReq.Model,
			Object: "list",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewOpenRouterEmbeddingsProvider("sk-test")
	p.client = srv.Client()
	p.baseURL = srv.URL

	_, err := p.Embed(context.Background(), &domain.EmbeddingRequest{
		Model:          "voyage/voyage-3-large",
		Input:          []string{"x"},
		Dimensions:     1024,
		EncodingFormat: "float",
		InputType:      "search_query",
	})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	if gotReq.Model != "voyage/voyage-3-large" {
		t.Errorf("Model = %q, want voyage/voyage-3-large", gotReq.Model)
	}
	if gotReq.Dimensions != 1024 {
		t.Errorf("Dimensions = %d, want 1024", gotReq.Dimensions)
	}
	if gotReq.EncodingFormat != "float" {
		t.Errorf("EncodingFormat = %q, want float", gotReq.EncodingFormat)
	}
	if gotReq.InputType != "search_query" {
		t.Errorf("InputType = %q, want search_query", gotReq.InputType)
	}
}

func TestEmbed_PreservesUsage(t *testing.T) {
	srv := fakeEmbeddingsServer(t, 0, embVec)
	defer srv.Close()

	p := NewOpenRouterEmbeddingsProvider("sk-test")
	p.client = srv.Client()
	p.baseURL = srv.URL

	resp, err := p.Embed(context.Background(), &domain.EmbeddingRequest{
		Model: "voyage/voyage-3-large",
		Input: []string{"a", "b"},
	})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	if resp.Usage.PromptTokens != 2 {
		t.Errorf("PromptTokens = %d, want 2", resp.Usage.PromptTokens)
	}
	if resp.Usage.TotalTokens != 2 {
		t.Errorf("TotalTokens = %d, want 2", resp.Usage.TotalTokens)
	}
}

func TestEmbed_ContextCancellation(t *testing.T) {
	srv := fakeEmbeddingsServer(t, 0, embVec)
	defer srv.Close()

	p := NewOpenRouterEmbeddingsProvider("sk-test")
	p.client = srv.Client()
	p.baseURL = srv.URL

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := p.Embed(ctx, &domain.EmbeddingRequest{
		Model: "voyage/voyage-3-large",
		Input: []string{"x"},
	})
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestEmbed_HTTPErrorPropagated(t *testing.T) {
	srv := fakeEmbeddingsServer(t, http.StatusInternalServerError, nil)
	defer srv.Close()

	p := NewOpenRouterEmbeddingsProvider("sk-test")
	p.client = srv.Client()
	p.baseURL = srv.URL

	_, err := p.Embed(context.Background(), &domain.EmbeddingRequest{
		Model: "voyage/voyage-3-large",
		Input: []string{"x"},
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

func TestEmbed_RateLimitPropagated(t *testing.T) {
	srv := fakeEmbeddingsServer(t, http.StatusTooManyRequests, nil)
	defer srv.Close()

	p := NewOpenRouterEmbeddingsProvider("sk-test")
	p.client = srv.Client()
	p.baseURL = srv.URL

	_, err := p.Embed(context.Background(), &domain.EmbeddingRequest{
		Model: "voyage/voyage-3-large",
		Input: []string{"x"},
	})
	if err == nil {
		t.Fatal("expected error from 429 response, got nil")
	}
	var pe *outbound.ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *outbound.ProviderError, got %T", err)
	}
	if pe.StatusCode != http.StatusTooManyRequests {
		t.Errorf("StatusCode = %d, want %d", pe.StatusCode, http.StatusTooManyRequests)
	}
}

func TestEmbeddingsRequestSerializes(t *testing.T) {
	req := embeddingsRequest{
		Model:     "voyage/voyage-3-large",
		Input:     []string{"x", "y"},
		InputType: "document",
	}
	got, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"model":"voyage/voyage-3-large","input":["x","y"],"input_type":"document"}`
	if string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestEmbeddingsResponseDeserializes(t *testing.T) {
	body := `{"data":[{"embedding":[0.1,0.2,0.3],"index":0,"object":"embedding"}],"model":"voyage/voyage-3-large","object":"list","usage":{"prompt_tokens":2,"total_tokens":2}}`
	var resp embeddingsResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Model != "voyage/voyage-3-large" {
		t.Errorf("Model = %q, want voyage/voyage-3-large", resp.Model)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("Data len = %d, want 1", len(resp.Data))
	}
	wantEmbed := []float32{0.1, 0.2, 0.3}
	if !reflect.DeepEqual(resp.Data[0].Embedding, wantEmbed) {
		t.Errorf("Embedding = %v, want %v", resp.Data[0].Embedding, wantEmbed)
	}
	if resp.Usage.TotalTokens != 2 {
		t.Errorf("TotalTokens = %d, want 2", resp.Usage.TotalTokens)
	}
}

func TestNewOpenRouterEmbeddingsProvider(t *testing.T) {
	p := NewOpenRouterEmbeddingsProvider("sk-test")
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	if p.apiKey != "sk-test" {
		t.Errorf("apiKey = %q, want sk-test", p.apiKey)
	}
}

func TestOpenRouterEmbeddingsProvider_ImplementsEmbedder(t *testing.T) {
	var _ outbound.Embedder = (*OpenRouterEmbeddingsProvider)(nil)
}
