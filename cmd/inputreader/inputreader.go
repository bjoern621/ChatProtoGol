package inputreader

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"bjoernblessin.de/chatprotogol/sock"
)

type Command string

type CommandHandler func(args []string)

type InputReader struct {
	scanner  *bufio.Scanner
	handlers map[Command][]CommandHandler
	socket   sock.Socket
}

func NewInputReader(socket sock.Socket) *InputReader {
	return &InputReader{
		scanner:  bufio.NewScanner(os.Stdin),
		handlers: make(map[Command][]CommandHandler),
		socket:   socket,
	}
}

func (ir *InputReader) AddHandler(cmd Command, handler CommandHandler) {
	ir.handlers[cmd] = append(ir.handlers[cmd], handler)
}

// InputLoop continuously reads from stdin and notifies registered handlers about commands.
// This method will block until an "exit" command is processed or an error in input scanning occurs.
func (ir *InputReader) InputLoop() {
	fmt.Println("Ready for commands. Type 'exit' to stop, 'help' for a list of commands.")

	for {
		addrPort, err := ir.socket.GetLocalAddress()
		var promptPrefix string
		if err != nil {
			promptPrefix = "Socket closed"
		} else {
			promptPrefix = addrPort.String()
		}
		fmt.Printf("%s > ", promptPrefix)

		if !ir.scanner.Scan() {
			if err := ir.scanner.Err(); err != nil {
				fmt.Fprintln(os.Stderr, "Error reading from stdin:", err)
			}
			break
		}

		parts := strings.Fields(ir.scanner.Text())
		if len(parts) == 0 {
			continue
		}

		command := strings.ToLower(parts[0])
		args := parts[1:]

		if command == "exit" {
			for _, handler := range ir.handlers[Command(command)] {
				handler(args)
			}
			return
		} else if command == "help" {
			fmt.Println("Available commands:")

			for cmd := range ir.handlers {
				fmt.Printf("- %s\n", cmd)
			}
		} else {
			if _, exists := ir.handlers[Command(command)]; !exists {
				fmt.Printf("No handlers registered for command: '%s'\n", command)
			} else {
				for _, handler := range ir.handlers[Command(command)] {
					handler(args)
				}
			}
		}
	}
}
