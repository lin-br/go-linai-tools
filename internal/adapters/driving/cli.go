package driving

import (
	"bufio"
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

func (c *CLI) StartAgent(in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)
	writer := bufio.NewWriter(out)
	log.SetOutput(writer)

	_, _ = writer.WriteString("Hey there! How are you doing?\n")

	for scanner.Scan() {
		text := scanner.Text()
		sended, err := c.useCase.Send(text)
		if err != nil {
			log.Fatal(err)
		}

		_, err = writer.WriteString(sended.Message)
		if err != nil {
			log.Fatal(err)
		}
	}
}
