package tools

import "errors"

// ErrNoToolCall is returned by Extract when the provider response contains no
// tool calls (the model ignored the forced tool choice and returned text).
var ErrNoToolCall = errors.New("no tool call in response")

// ErrToolNameMismatch is returned by Extract when the response contains one or
// more tool calls but none matches the schema's tool name.
var ErrToolNameMismatch = errors.New("tool call name does not match schema")

// ErrUnmarshalFailed is returned by Extract when the matching tool call's
// arguments cannot be JSON-decoded into the target type. It is wrapped so that
// errors.Is(err, ErrUnmarshalFailed) matches while the underlying decode error
// is preserved in the message.
var ErrUnmarshalFailed = errors.New("failed to unmarshal tool call arguments")
