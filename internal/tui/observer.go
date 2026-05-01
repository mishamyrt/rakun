package tui

import (
	"context"
	"fmt"
	"os"
	"rakun/internal/taskrun"
	"time"

	tea "charm.land/bubbletea/v2"
)

// Observer forwards task events into the interactive TUI.
type Observer struct {
	done        chan error
	interactive bool
	program     *tea.Program
}

type plainObserver struct {
	summary taskrun.Summary
}

// New creates a task observer that uses the TUI when stdout is interactive.
func New(total int, jobs int, cancel context.CancelFunc) taskrun.Observer {
	if !isInteractiveTerminal(os.Stdout) {
		return &plainObserver{}
	}

	program := tea.NewProgram(
		newModel(total, jobs, cancel),
		tea.WithOutput(os.Stdout),
		tea.WithoutSignalHandler(),
	)

	observer := &Observer{
		done:        make(chan error, 1),
		interactive: true,
		program:     program,
	}
	go func() {
		_, runErr := program.Run()
		observer.done <- runErr
		close(observer.done)
	}()
	return observer
}

// HandleEvent forwards an event to the interactive TUI program.
func (s *Observer) HandleEvent(event taskrun.Event) {
	if s == nil || !s.interactive {
		return
	}
	s.program.Send(event)
}

// Finish sends the final summary to the interactive TUI program.
func (s *Observer) Finish(summary taskrun.Summary) {
	if s == nil || !s.interactive {
		return
	}
	s.program.Send(summaryMsg{summary: summary})
}

// Close waits for the interactive TUI program to exit.
func (s *Observer) Close() error {
	if s == nil || !s.interactive {
		return nil
	}
	return <-s.done
}

func (s *plainObserver) HandleEvent(_ taskrun.Event) {}

func (s *plainObserver) Finish(summary taskrun.Summary) {
	s.summary = summary

	lines := []string{
		fmt.Sprintf("  total: %d", summary.Total),
		fmt.Sprintf("  succeeded: %d", summary.Succeeded),
		fmt.Sprintf("  changed: %d", summary.Changed),
		fmt.Sprintf("  unchanged: %d", summary.Unchanged),
		fmt.Sprintf("  canceled: %d", summary.Canceled),
		fmt.Sprintf("  failed: %d", summary.Failed),
		fmt.Sprintf("  duration: %s", summary.Duration.Round(100*time.Millisecond)),
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(os.Stdout, line); err != nil {
			return
		}
	}
}

func (s *plainObserver) Close() error {
	return nil
}
