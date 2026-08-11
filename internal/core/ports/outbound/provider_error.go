package outbound

import "fmt"

// Compile-time interface check: ProviderError satisfies the error interface.
var _ error = (*ProviderError)(nil)

// ProviderError carries the HTTP status code and response body from a provider
// HTTP failure. The optional Err field wraps a transport-level error so that
// errors.Is / errors.As can reach the underlying cause.
type ProviderError struct {
	StatusCode int
	Body       string
	Err        error
}

// Error implements the error interface, surfacing the status code and body.
func (e *ProviderError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("provider error: HTTP %d: %s: %v", e.StatusCode, e.Body, e.Err)
	}
	return fmt.Sprintf("provider error: HTTP %d: %s", e.StatusCode, e.Body)
}

// Unwrap returns the wrapped inner error, enabling errors.Is and errors.As
// to inspect the underlying cause. Returns nil when no inner error is set.
func (e *ProviderError) Unwrap() error {
	return e.Err
}
