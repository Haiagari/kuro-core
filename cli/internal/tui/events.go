package tui


import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"kuro/cli/internal/orchestrator"
)

// waitForEvent returns a tea.Cmd that reads PhaseEvent from the channel
// and forwards them as tea.Msg to the Bubbletea program.
// The goroutine exits when the context is cancelled or the channel is closed.
func waitForEvent(ctx context.Context, eventsCh <-chan orchestrator.PhaseEvent) tea.Cmd {
	return func() tea.Msg {
		select {
		case evt, ok := <-eventsCh:
			if !ok {
				return nil
			}
			return evt
		case <-ctx.Done():
			return nil
		}
	}
}
