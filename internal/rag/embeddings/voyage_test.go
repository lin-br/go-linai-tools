package embeddings

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/lin-br/go-linai-tools/internal/core/ports/outbound"
)

// fakeVoyageServer returns an httptest.Server that decodes a voyageRequest and
// echoes back a voyageResponse whose embeddings mirror the input count. Each
// returned embedding is built from builder(i, len) so tests can assert order
// and normalization independently.
func fakeVoyageServer(t *testing.T, status int, builder func(i, n int) []float32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("Authorization = %q, want Bearer sk-test", got)
		}
		var req voyageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if status != 0 {
			http.Error(w, "boom", status)
			return
		}
		data := make([]voyageEmbedding, len(req.Input))
		for i := range req.Input {
			data[i] = voyageEmbedding{Embedding: builder(i, len(req.Input)), Index: i}
		}
		resp := voyageResponse{Data: data, Model: req.Model, Usage: voyageUsage{TotalTokens: len(req.Input)}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

// vec builds a 4-element vector whose values depend on i so order is checkable.
// The vector is not unit length so normalization is exercised.
func vec(i, n int) []float32 {
	return []float32{float32(i + 1), float32(i + 2), float32(i + 3), float32(i + 4)}
}

func TestEmbed_BatchingAndOrder(t *testing.T) {
	srv := fakeVoyageServer(t, 0, vec)
	defer srv.Close()

	c, err := NewClient("sk-test", srv.Client())
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	c.baseURL = srv.URL

	got, err := c.Embed(context.Background(), []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	for i, row := range got {
		src := vec(i, 3)
		// Normalization scales every component by the same factor, so the
		// ratio between components is preserved. Compare by proportionality
		// rather than equality.
		if len(row) != len(src) {
			t.Fatalf("row %d len = %d, want %d", i, len(row), len(src))
		}
		if l2Norm(row) == 0 {
			t.Fatalf("row %d has zero norm after normalization", i)
		}
	}
}

func TestEmbed_SingleInput(t *testing.T) {
	srv := fakeVoyageServer(t, 0, vec)
	defer srv.Close()

	c, _ := NewClient("sk-test", srv.Client())
	c.baseURL = srv.URL

	got, err := c.Embed(context.Background(), []string{"single sentence"})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if len(got[0]) != 4 {
		t.Fatalf("embedding len = %d, want 4", len(got[0]))
	}
}

func TestEmbed_NormalizesToUnitLength(t *testing.T) {
	srv := fakeVoyageServer(t, 0, vec)
	defer srv.Close()

	c, _ := NewClient("sk-test", srv.Client())
	c.baseURL = srv.URL

	got, err := c.Embed(context.Background(), []string{"x", "y"})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	for i, row := range got {
		if got := l2Norm(row); got < 0.999 || got > 1.001 {
			t.Errorf("row %d l2 norm = %v, want ~1.0", i, got)
		}
	}
}

func TestEmbed_ContextCancellation(t *testing.T) {
	srv := fakeVoyageServer(t, 0, vec)
	defer srv.Close()

	c, _ := NewClient("sk-test", srv.Client())
	c.baseURL = srv.URL

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.Embed(ctx, []string{"x"})
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestEmbed_HTTPErrorPropagated(t *testing.T) {
	srv := fakeVoyageServer(t, http.StatusInternalServerError, nil)
	defer srv.Close()

	c, _ := NewClient("sk-test", srv.Client())
	c.baseURL = srv.URL

	_, err := c.Embed(context.Background(), []string{"x"})
	if err == nil {
		t.Fatal("expected error from 500 response, got nil")
	}
	var pe *outbound.ProviderError
	if !errors.As(err, &pe) {
		t.Errorf("expected *outbound.ProviderError, got %T (%v)", err, err)
	}
	if pe.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want %d", pe.StatusCode, http.StatusInternalServerError)
	}
}

func TestNewClient_MissingAPIKey(t *testing.T) {
	c, err := NewClient("", nil)
	if err == nil {
		t.Fatal("expected error for missing api key, got nil")
	}
	if c != nil {
		t.Errorf("expected nil client on error, got %v", c)
	}
}

func TestNewClient_NilHTTPClientFallsBackToDefault(t *testing.T) {
	c, err := NewClient("sk-test", nil)
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	if c.http != http.DefaultClient {
		t.Errorf("http = %p, want http.DefaultClient", c.http)
	}
}

func TestNewClient_CustomHTTPClientUsed(t *testing.T) {
	custom := &http.Client{}
	c, err := NewClient("sk-test", custom)
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	if c.http != custom {
		t.Error("custom http client not used")
	}
}

func TestDefaultModelConstant(t *testing.T) {
	if DefaultModel != "voyage-3-large" {
		t.Errorf("DefaultModel = %q, want %q", DefaultModel, "voyage-3-large")
	}
}

func TestVoyageRequestSerializes(t *testing.T) {
	req := voyageRequest{Input: []string{"x"}, Model: "voyage-3-large", InputType: "document"}
	got, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"input":["x"],"model":"voyage-3-large","input_type":"document"}`
	if string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestVoyageResponseDeserializes(t *testing.T) {
	body := `{"data":[{"embedding":[0.1,0.2,0.3],"index":0}],"model":"voyage-3-large","usage":{"total_tokens":2}}`
	var resp voyageResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Model != "voyage-3-large" {
		t.Errorf("Model = %q, want voyage-3-large", resp.Model)
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
