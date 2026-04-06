package tui

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"golang.org/x/term"
)

func TestPasswordExecCommandRequiresTerminalFile(t *testing.T) {
	command := &passwordExecCommand{prompt: "API Key: "}
	command.SetStdin(strings.NewReader("not-a-file"))
	if err := command.Run(); err == nil {
		t.Fatal("expected error for non-file stdin")
	}
}

func TestPromptSecureSecretInputReturnsCmd(t *testing.T) {
	if cmd := PromptSecureSecretInput("API Key: "); cmd == nil {
		t.Fatal("expected non-nil tea command")
	}
}

func TestReadSecretBytesNormalEntry(t *testing.T) {
	got, err := readSecretBytes(strings.NewReader("secret-value\n"))
	if err != nil {
		t.Fatalf("readSecretBytes() error = %v", err)
	}
	if got != "secret-value" {
		t.Fatalf("secret = %q", got)
	}
}

func TestReadSecretBytesBackspace(t *testing.T) {
	got, err := readSecretBytes(bytes.NewBuffer([]byte{'a', 'b', 'c', 127, 'd', '\n'}))
	if err != nil {
		t.Fatalf("readSecretBytes() error = %v", err)
	}
	if got != "abd" {
		t.Fatalf("secret = %q", got)
	}
}

func TestReadSecretBytesCtrlCCancel(t *testing.T) {
	_, err := readSecretBytes(bytes.NewBuffer([]byte{'a', 3}))
	if !errors.Is(err, ErrSecretInputCancelled) {
		t.Fatalf("expected cancel error, got %v", err)
	}
}

func TestReadSecretBytesCtrlDCancel(t *testing.T) {
	_, err := readSecretBytes(bytes.NewBuffer([]byte{4}))
	if !errors.Is(err, ErrSecretInputCancelled) {
		t.Fatalf("expected cancel error, got %v", err)
	}
}

func TestReadSecretWithCancelRestoresStateOnCancel(t *testing.T) {
	inReader, inWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	defer inReader.Close()
	defer inWriter.Close()

	outReader, outWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	defer outReader.Close()
	defer outWriter.Close()

	oldGetState := getTerminalState
	oldRestore := restoreTerminalState
	oldEnable := enableSecretMode
	t.Cleanup(func() {
		getTerminalState = oldGetState
		restoreTerminalState = oldRestore
		enableSecretMode = oldEnable
	})

	var restored bool
	getTerminalState = func(fd int) (*term.State, error) { return &term.State{}, nil }
	restoreTerminalState = func(fd int, state *term.State) error {
		restored = true
		return nil
	}
	enableSecretMode = func(fd int) error { return nil }

	go func() {
		_, _ = inWriter.Write([]byte{3})
		_ = inWriter.Close()
	}()

	_, err = ReadSecretWithCancel(inReader, outWriter, "API Key: ")
	if !errors.Is(err, ErrSecretInputCancelled) {
		t.Fatalf("expected cancel error, got %v", err)
	}
	if !restored {
		t.Fatal("expected terminal state to be restored on cancel")
	}
	_ = outWriter.Close()
	output, _ := io.ReadAll(outReader)
	if !strings.Contains(string(output), "API Key: ") {
		t.Fatalf("prompt output = %q", string(output))
	}
}

func TestReadSecretWithCancelRestoresStateOnEnableError(t *testing.T) {
	inReader, _, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	defer inReader.Close()

	outReader, outWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	defer outReader.Close()
	defer outWriter.Close()

	oldGetState := getTerminalState
	oldRestore := restoreTerminalState
	oldEnable := enableSecretMode
	t.Cleanup(func() {
		getTerminalState = oldGetState
		restoreTerminalState = oldRestore
		enableSecretMode = oldEnable
	})

	getTerminalState = func(fd int) (*term.State, error) { return &term.State{}, nil }
	restoreTerminalState = func(fd int, state *term.State) error {
		t.Fatal("restore should not be called when enableSecretMode fails before defer")
		return nil
	}
	enableSecretMode = func(fd int) error { return errors.New("boom") }

	_, err = ReadSecretWithCancel(inReader, outWriter, "API Key: ")
	if err == nil || !strings.Contains(err.Error(), "enabling secure input mode") {
		t.Fatalf("expected enable error, got %v", err)
	}
}

func TestReadSecretWithInterruptCancelsOnSignal(t *testing.T) {
	oldNotify := notifyInterrupt
	oldStop := stopInterruptNotify
	t.Cleanup(func() {
		notifyInterrupt = oldNotify
		stopInterruptNotify = oldStop
	})

	notifyInterrupt = func(ch chan<- os.Signal) {
		ch <- os.Interrupt
	}
	stopInterruptNotify = func(ch chan<- os.Signal) {}

	_, err := readSecretWithInterrupt(strings.NewReader("will-not-be-read"))
	if !errors.Is(err, ErrSecretInputCancelled) {
		t.Fatalf("expected interrupt cancel, got %v", err)
	}
}

func TestIsSecretInputCancel(t *testing.T) {
	cases := []error{
		ErrSecretInputCancelled,
		errors.New("interrupted"),
		errors.New("signal: interrupt"),
		errors.New("operation cancelled"),
	}
	for _, err := range cases {
		if !isSecretInputCancel(err) {
			t.Fatalf("expected cancel detection for %v", err)
		}
	}
}
