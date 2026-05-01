package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func StartSpinner(status string) func() {
	if !isInteractiveTerminal(os.Stdout) {
		return func() {}
	}

	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		frame := 0
		for {
			fmt.Fprintf(os.Stdout, "\r%s %s", spinnerFrames[frame], dimStyle.Render(status))
			frame = (frame + 1) % len(spinnerFrames)

			select {
			case <-done:
				fmt.Fprintf(os.Stdout, "\r%s\r", strings.Repeat(" ", len(status)+4))
				close(stopped)
				return
			case <-ticker.C:
			}
		}
	}()

	return func() {
		close(done)
		<-stopped
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}
