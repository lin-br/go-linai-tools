package domain

// StreamEventType categorizes a streaming event.
type StreamEventType string

// Stream event types.
const (
	StreamEventTypeText   StreamEventType = "text"
	StreamEventTypeFinish StreamEventType = "finish"
	StreamEventTypeError  StreamEventType = "error"
)

// StreamEvent represents a single chunk or terminal event from a streaming chat.
type StreamEvent struct {
	Type       StreamEventType
	Delta      string
	StopReason string
	Usage      *Usage
	Err        error
}
