package tui

import (
	"context"
	"fmt"
	"os"
	"path"
	"rakun/internal/taskrun"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type taskState struct {
	canceled  bool
	completed bool
	duration  time.Duration
	failed    bool
	progress  float64
	startedAt time.Time
	status    string
	title     string
}

type summaryMsg struct {
	summary taskrun.Summary
}

type tickMsg struct{}

type quitMsg struct{}

type Observer struct {
	done        chan error
	interactive bool
	program     *tea.Program
}

type model struct {
	cancel          context.CancelFunc
	cancelRequested bool
	completed       int
	failed          int
	frame           int
	height          int
	jobs            int
	outcomes        []taskrun.TaskSummary
	order           []string
	summary         *taskrun.Summary
	taskStates      map[string]*taskState
	total           int
}

type plainObserver struct {
	summary taskrun.Summary
}

var (
	titleStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	dimStyle           = lipgloss.NewStyle().Faint(true)
	okStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	failStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	barFull            = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).SetString("-")
	barEmpty           = lipgloss.NewStyle().Faint(true).SetString("-")
	taskTitleColumn    = lipgloss.NewStyle().Width(42)
	taskTitleDimColumn = dimStyle.Width(42)
	taskPercentColumn  = lipgloss.NewStyle().Width(4).Align(lipgloss.Right)
)

func New(total int, jobs int, cancel context.CancelFunc) taskrun.Observer {
	interactive := isInteractiveTerminal(os.Stdout)
	if !interactive {
		return &plainObserver{}
	}

	programModel := model{
		cancel:     cancel,
		height:     24,
		jobs:       jobs,
		order:      []string{},
		taskStates: map[string]*taskState{},
		total:      total,
	}
	program := tea.NewProgram(programModel, tea.WithOutput(os.Stdout), tea.WithoutSignalHandler())

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

func (s *Observer) HandleEvent(event taskrun.Event) {
	if s == nil || !s.interactive {
		return
	}
	s.program.Send(event)
}

func (s *Observer) Finish(summary taskrun.Summary) {
	if s == nil || !s.interactive {
		return
	}
	s.program.Send(summaryMsg{summary: summary})
}

func (s *Observer) Close() error {
	if s == nil || !s.interactive {
		return nil
	}
	return <-s.done
}

func (s *plainObserver) HandleEvent(event taskrun.Event) {}

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
		fmt.Fprintln(os.Stdout, line)
	}
}

func (s *plainObserver) Close() error {
	return nil
}

func (s model) Init() tea.Cmd {
	return tickCmd()
}

func (s model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case taskrun.Event:
		s.handleTaskEvent(message)
		return s, nil
	case summaryMsg:
		s.summary = &message.summary
		s.outcomes = append([]taskrun.TaskSummary(nil), message.summary.Outcomes...)
		return s, tea.Tick(180*time.Millisecond, func(time.Time) tea.Msg {
			return quitMsg{}
		})
	case tickMsg:
		if s.summary != nil {
			return s, nil
		}
		s.frame = (s.frame + 1) % len(spinnerFrames)
		return s, tickCmd()
	case tea.WindowSizeMsg:
		if message.Height > 0 {
			s.height = message.Height
		}
		return s, nil
	case quitMsg:
		return s, tea.Quit
	case tea.KeyMsg:
		if message.Type == tea.KeyCtrlC {
			if !s.cancelRequested {
				s.cancelRequested = true
				if s.cancel != nil {
					s.cancel()
				}
			}
			return s, nil
		}
		return s, nil
	default:
		return s, nil
	}
}

func (s model) View() string {
	if s.summary != nil {
		return s.summaryView()
	}
	return s.progressView()
}

func (s *model) handleTaskEvent(event taskrun.Event) {
	task := s.ensureTask(event.TaskID, event.Title)
	switch event.Kind {
	case taskrun.EventStarted:
		task.canceled = false
		task.startedAt = time.Now()
		task.status = "Starting"
		task.progress = 0.02
	case taskrun.EventProgress:
		task.progress = clamp(event.Progress)
		task.status = event.Status
	case taskrun.EventCompleted:
		task.canceled = false
		task.completed = true
		task.failed = false
		task.progress = 1
		task.status = event.Status
		if !task.startedAt.IsZero() {
			task.duration = time.Since(task.startedAt)
		}
		s.completed++
	case taskrun.EventFailed:
		task.canceled = false
		task.completed = true
		task.failed = true
		task.progress = 1
		task.status = event.Status
		if !task.startedAt.IsZero() {
			task.duration = time.Since(task.startedAt)
		}
		s.completed++
		s.failed++
	case taskrun.EventCanceled:
		task.canceled = true
		task.completed = true
		task.failed = false
		task.progress = 1
		task.status = event.Status
		if !task.startedAt.IsZero() {
			task.duration = time.Since(task.startedAt)
		}
		s.completed++
	}
}

func (s *model) ensureTask(id string, title string) *taskState {
	task := s.taskStates[id]
	if task != nil {
		if title != "" {
			task.title = title
		}
		return task
	}

	s.order = append(s.order, id)
	task = &taskState{
		progress: 0,
		title:    title,
	}
	s.taskStates[id] = task
	return task
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
	summary := s.summary
	if summary == nil {
		return ""
	}

	header := []string{
		dimStyle.Render(fmt.Sprintf("completed in %s", summary.Duration.Round(100*time.Millisecond))),
	}
	stats := []string{}
	appendStat := func(value int, style lipgloss.Style, label string) {
		if value == 0 {
			return
		}
		stats = append(stats, fmt.Sprintf("%s %d %s", style.Render("●"), value, label))
	}
	appendStat(summary.Succeeded, okStyle, "succeeded")
	appendStat(summary.Changed, okStyle, "changed")
	appendStat(summary.Unchanged, dimStyle, "unchanged")
	appendStat(summary.Canceled, dimStyle, "canceled")
	appendStat(summary.Failed, failStyle, "failed")
	if len(stats) > 0 {
		header = append(header, "", strings.Join(stats, "\n"))
	}

	body := []string{strings.Join(header, "\n")}
	return strings.Join(body, "\n\n") + "\n"
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
	return fmt.Sprintf("%s %s  %s", statusStyle.Render(prefix), truncate(shortRepoName(task.title), 42), dimStyle.Render(shorten(task.status, 48)))
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

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func isInteractiveTerminal(file *os.File) bool {
	if file == nil {
		return false
	}

	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
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
