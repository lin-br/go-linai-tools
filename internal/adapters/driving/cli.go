package driving

import (
	"bufio"
	"context"
	"io"
	"log"

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
		response, err := c.useCase.Send(ctx, text)
		if err != nil {
			_, _ = writer.WriteString(err.Error() + "\n")
			_ = writer.Flush()
			log.Fatal(err)
		}

		_, err = writer.WriteString(response.Content + "\n")
		if err != nil {
			log.Fatal(err)
		}
		_ = writer.Flush()
	}
}
