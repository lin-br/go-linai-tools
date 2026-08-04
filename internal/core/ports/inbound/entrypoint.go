package inbound

import "io"

type Entrypoint interface {
	StartAgent(in io.Reader, out io.Writer)
}
