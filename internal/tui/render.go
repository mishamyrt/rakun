package tui

import (
	"fmt"
	"path"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	dimStyle           = lipgloss.NewStyle().Faint(true)
	okStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	failStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	barFull            = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).SetString("-")
	barEmpty           = lipgloss.NewStyle().Faint(true).SetString("-")
	taskTitleDimColumn = dimStyle.Width(42)
	taskPercentColumn  = lipgloss.NewStyle().Width(4).Align(lipgloss.Right)
)

func (s model) View() tea.View {
	if s.summary != nil {
		return tea.NewView(s.summaryView())
	}
	return tea.NewView(s.progressView())
}

func (s model) progressView() string {
	activeIDs := s.activeTaskIDs()
	recent := s.recentOutcomes(s.recentCapacity(len(activeIDs)))
	lines := []string{}

	for _, task := range recent {
		lines = append(lines, s.renderRecent(task))
	}

	for _, id := range activeIDs {
		task := s.taskStates[id]
		lines = append(lines, s.renderTaskLine(task))
	}

	if len(lines) > 0 {
		lines = append(lines, "")
	}
	lines = append(lines, s.renderProgressLine())

	return strings.Join(lines, "\n") + "\n"
}

func (s model) summaryView() string {
	if s.summary == nil {
		return ""
	}

	header := []string{
		dimStyle.Render(fmt.Sprintf("completed in %s", s.summary.Duration.Round(100*time.Millisecond))),
	}
	stats := []string{}
	appendStat := func(value int, style lipgloss.Style, label string) {
		if value == 0 {
			return
		}
		stats = append(stats, fmt.Sprintf("%s %d %s", style.Render("●"), value, label))
	}
	appendStat(s.summary.Succeeded, okStyle, "succeeded")
	appendStat(s.summary.Changed, okStyle, "changed")
	appendStat(s.summary.Unchanged, dimStyle, "unchanged")
	appendStat(s.summary.Canceled, dimStyle, "canceled")
	appendStat(s.summary.Failed, failStyle, "failed")
	if len(stats) > 0 {
		header = append(header, "", strings.Join(stats, "\n"))
	}

	return strings.Join(header, "\n") + "\n"
}

func (s model) renderProgressLine() string {
	line := fmt.Sprintf(
		"%s %s (%d/%d)",
		spinnerFrames[s.frame],
		dimStyle.Render("Processing..."),
		s.completed,
		s.total,
	)
	if s.cancelRequested {
		line += dimStyle.Render("  canceling...")
	}
	return line
}

func (s model) renderTaskLine(task *taskState) string {
	title := taskTitleDimColumn.Render(truncate(shortRepoName(task.title), 42))
	percent := taskPercentColumn.Render(fmt.Sprintf("%d%%", int(task.progress*100)))
	return fmt.Sprintf(
		"%s  %s  %s  %s",
		title,
		renderBar(task.progress, 20),
		percent,
		dimStyle.Render(shorten(task.status, 40)),
	)
}

func (s model) renderRecent(task *taskState) string {
	statusStyle := okStyle
	prefix := "✔"
	if task.canceled {
		statusStyle = dimStyle
		prefix = "◌"
	} else if task.failed {
		statusStyle = failStyle
		prefix = "✖"
	}
	return fmt.Sprintf(
		"%s %s  %s",
		statusStyle.Render(prefix),
		truncate(shortRepoName(task.title), 42),
		dimStyle.Render(shorten(task.status, 48)),
	)
}

func (s model) activeTaskIDs() []string {
	activeIDs := []string{}
	for _, id := range s.order {
		task := s.taskStates[id]
		if task.completed {
			continue
		}
		activeIDs = append(activeIDs, id)
	}
	if len(activeIDs) > s.jobs {
		return activeIDs[:s.jobs]
	}
	return activeIDs
}

func (s model) recentOutcomes(limit int) []*taskState {
	if limit <= 0 {
		return nil
	}
	recent := []*taskState{}
	for index := len(s.order) - 1; index >= 0; index-- {
		task := s.taskStates[s.order[index]]
		if !task.completed {
			continue
		}
		recent = append(recent, task)
		if len(recent) == limit {
			break
		}
	}
	return recent
}

func (s model) recentCapacity(activeCount int) int {
	reservedLines := activeCount + 1
	if activeCount > 0 || s.completed > 0 {
		reservedLines++
	}
	available := s.height - reservedLines
	if available < 0 {
		return 0
	}
	return available
}

func clamp(progress float64) float64 {
	if progress < 0 {
		return 0
	}
	if progress > 1 {
		return 1
	}
	return progress
}

func renderBar(progress float64, width int) string {
	progress = clamp(progress)
	filled := int(progress * float64(width))
	filled = max(filled, 0)
	filled = min(filled, width)

	return strings.Repeat(barFull.String(), filled) + strings.Repeat(barEmpty.String(), width-filled)
}

func truncate(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 1 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
}

func shorten(value string, width int) string {
	return truncate(strings.TrimSpace(value), width)
}

func shortRepoName(value string) string {
	trimmed := strings.Trim(value, "/")
	if trimmed == "" {
		return value
	}
	return path.Base(trimmed)
}
