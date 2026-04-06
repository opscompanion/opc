package tui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

var ErrSecretInputCancelled = errors.New("secure input cancelled")

type SecretResultMsg struct {
	Value string
	Err   error
}

type passwordExecCommand struct {
	prompt string
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	value  string
}

var (
	getTerminalState     = term.GetState
	restoreTerminalState = term.Restore
	enableSecretMode     = setSecretInputMode
	notifyInterrupt      = func(ch chan<- os.Signal) { signal.Notify(ch, os.Interrupt, syscall.SIGINT) }
	stopInterruptNotify  = signal.Stop
)

func (c *passwordExecCommand) SetStdin(r io.Reader)  { c.stdin = r }
func (c *passwordExecCommand) SetStdout(w io.Writer) { c.stdout = w }
func (c *passwordExecCommand) SetStderr(w io.Writer) { c.stderr = w }

func (c *passwordExecCommand) Run() error {
	inFile, ok := c.stdin.(*os.File)
	if !ok {
		return fmt.Errorf("secure input requires a terminal stdin")
	}
	outFile, ok := c.stdout.(*os.File)
	if !ok {
		return fmt.Errorf("secure input requires a terminal stdout")
	}

	secret, err := ReadSecretWithCancel(inFile, outFile, c.prompt)
	if err != nil {
		return err
	}
	c.value = secret
	return nil
}

func ReadSecretWithCancel(inFile *os.File, outFile *os.File, prompt string) (string, error) {
	oldState, err := getTerminalState(int(inFile.Fd()))
	if err != nil {
		return "", fmt.Errorf("saving terminal state: %w", err)
	}
	if err := enableSecretMode(int(inFile.Fd())); err != nil {
		return "", fmt.Errorf("enabling secure input mode: %w", err)
	}
	defer restoreTerminalState(int(inFile.Fd()), oldState)
	defer fmt.Fprintln(outFile)

	if _, err := fmt.Fprint(outFile, prompt); err != nil {
		return "", err
	}

	secret, err := readSecretWithInterrupt(inFile)
	if err != nil {
		if isSecretInputCancel(err) {
			return "", ErrSecretInputCancelled
		}
		return "", fmt.Errorf("reading secret: %w", err)
	}
	return secret, nil
}

type secretReadResult struct {
	value string
	err   error
}

func readSecretWithInterrupt(r io.Reader) (string, error) {
	resultCh := make(chan secretReadResult, 1)
	interruptCh := make(chan os.Signal, 1)
	notifyInterrupt(interruptCh)
	defer stopInterruptNotify(interruptCh)

	go func() {
		value, err := readSecretBytes(r)
		resultCh <- secretReadResult{value: value, err: err}
	}()

	select {
	case result := <-resultCh:
		return result.value, result.err
	case <-interruptCh:
		return "", ErrSecretInputCancelled
	}
}

func readSecretBytes(r io.Reader) (string, error) {
	var buf [1]byte
	secret := make([]byte, 0, 32)

	for {
		n, err := r.Read(buf[:])
		if n > 0 {
			switch buf[0] {
			case '\r', '\n':
				return string(secret), nil
			case '\b', 127:
				if len(secret) > 0 {
					secret = secret[:len(secret)-1]
				}
			case 3, 4:
				return "", ErrSecretInputCancelled
			default:
				secret = append(secret, buf[0])
			}
			continue
		}
		if err != nil {
			if err == io.EOF && len(secret) > 0 {
				return string(secret), nil
			}
			if err == io.EOF {
				return "", ErrSecretInputCancelled
			}
			return "", err
		}
	}
}

func PromptSecureSecretInput(prompt string) tea.Cmd {
	command := &passwordExecCommand{prompt: prompt}
	return tea.Exec(command, func(err error) tea.Msg {
		return SecretResultMsg{
			Value: command.value,
			Err:   err,
		}
	})
}

func isSecretInputCancel(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.EINTR) || errors.Is(err, ErrSecretInputCancelled) {
		return true
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "interrupt") || strings.Contains(text, "canceled") || strings.Contains(text, "cancelled")
}
