package inbound

import (
	"context"
	"io"
)

type Entrypoint interface {
	StartAgent(ctx context.Context, in io.Reader, out io.Writer)
}
