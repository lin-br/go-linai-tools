package driving

import (
	"bufio"
	"context"
	"io"
	"log"

	"github.com/lin-br/go-linai-tools/internal/core/domain"
	"github.com/lin-br/go-linai-tools/internal/core/usecases"
)

type CLI struct {
	useCase *usecases.DoSendMessageUseCase
}

func NewCLI(useCase *usecases.DoSendMessageUseCase) *CLI {
	return &CLI{
		useCase: useCase,
	}
}

func (c *CLI) StartAgent(ctx context.Context, in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)
	writer := bufio.NewWriter(out)
	log.SetOutput(writer)

	_, _ = writer.WriteString("Hey there! How are you doing?\n")
	_ = writer.Flush()

	for scanner.Scan() {
		text := scanner.Text()
		events, err := c.useCase.Stream(ctx, text)
		if err != nil {
			_, _ = writer.WriteString(err.Error() + "\n")
			_ = writer.Flush()
			log.Fatal(err)
		}

		for event := range events {
			switch event.Type {
			case domain.StreamEventTypeText:
				_, _ = writer.WriteString(event.Delta)
			case domain.StreamEventTypeFinish:
				_, _ = writer.WriteString("\n")
			case domain.StreamEventTypeError:
				_, _ = writer.WriteString("\n" + event.Err.Error() + "\n")
			}
		}
		_ = writer.Flush()
	}
}
