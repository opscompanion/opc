package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/opscompanion/opc/internal/tui"
)

func promptConfirmAction(label string, message string, defaultYes bool) (bool, error) {
	if !isInteractiveSession() {
		return promptConfirmFallback(message, defaultYes)
	}

	model := tui.NewConfirmModel(label, message, defaultYes)
	finalModel, err := tea.NewProgram(model).Run()
	if err != nil {
		return false, err
	}
	result, ok := finalModel.(*tui.ConfirmModel)
	if !ok {
		return false, fmt.Errorf("unexpected confirm model type %T", finalModel)
	}
	if result.Cancelled {
		return false, nil
	}
	return result.YesSelected(), nil
}

func promptConfirmFallback(message string, defaultYes bool) (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	suffix := "[y/N]"
	if defaultYes {
		suffix = "[Y/n]"
	}
	fmt.Printf("%s %s: ", message, suffix)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("reading confirmation: %w", err)
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer == "" {
		return defaultYes, nil
	}
	return answer == "y" || answer == "yes", nil
}
