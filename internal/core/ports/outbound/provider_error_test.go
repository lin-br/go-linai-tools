package outbound

import (
	"errors"
	"testing"
)

func TestProviderErrorError(t *testing.T) {
	tests := []struct {
		name string
		err  *ProviderError
		want string
	}{
		{
			name: "status and body only",
			err:  &ProviderError{StatusCode: 429, Body: "rate limited"},
			want: "provider error: HTTP 429: rate limited",
		},
		{
			name: "with wrapped err",
			err:  &ProviderError{StatusCode: 500, Body: "boom", Err: errors.New("connection reset")},
			want: "provider error: HTTP 500: boom: connection reset",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProviderErrorErrorsAs(t *testing.T) {
	pe := &ProviderError{StatusCode: 429, Body: "rate limited"}
	var target *ProviderError
	if !errors.As(pe, &target) {
		t.Fatal("errors.As failed to match *ProviderError")
	}
	if target.StatusCode != 429 {
		t.Errorf("StatusCode = %d, want 429", target.StatusCode)
	}
}

func TestProviderErrorErrorsAsWrapped(t *testing.T) {
	wrapped := errors.Join(&ProviderError{StatusCode: 400, Body: "bad request"})
	var target *ProviderError
	if !errors.As(wrapped, &target) {
		t.Fatal("errors.As failed to extract *ProviderError from wrapped error")
	}
	if target.StatusCode != 400 {
		t.Errorf("StatusCode = %d, want 400", target.StatusCode)
	}
}

func TestProviderErrorUnwrap(t *testing.T) {
	inner := errors.New("root cause")
	pe := &ProviderError{StatusCode: 503, Body: "unavailable", Err: inner}
	if !errors.Is(pe, inner) {
		t.Fatal("errors.Is failed to find wrapped inner error")
	}
	if unwrapped := pe.Unwrap(); unwrapped != inner {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, inner)
	}
}

func TestProviderErrorUnwrapNil(t *testing.T) {
	pe := &ProviderError{StatusCode: 500}
	if unwrapped := pe.Unwrap(); unwrapped != nil {
		t.Errorf("Unwrap() = %v, want nil", unwrapped)
	}
}
