package tui

import (
	"context"
	"rakun/internal/taskrun"
	"time"

	tea "charm.land/bubbletea/v2"
)

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

type model struct {
	cancel          context.CancelFunc
	now             func() time.Time
	cancelRequested bool
	completed       int
	failed          int
	frame           int
	height          int
	jobs            int
	order           []string
	summary         *taskrun.Summary
	taskStates      map[string]*taskState
	total           int
}

func newModel(total int, jobs int, cancel context.CancelFunc) model {
	return model{
		cancel:     cancel,
		height:     24,
		jobs:       jobs,
		now:        time.Now,
		order:      []string{},
		taskStates: map[string]*taskState{},
		total:      total,
	}
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
	case tea.KeyPressMsg:
		if message.String() == "ctrl+c" {
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

func (s *model) handleTaskEvent(event taskrun.Event) {
	task := s.ensureTask(event.TaskID, event.Title)
	switch event.Kind {
	case taskrun.EventStarted:
		task.canceled = false
		task.startedAt = s.now()
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
			task.duration = s.now().Sub(task.startedAt)
		}
		s.completed++
	case taskrun.EventFailed:
		task.canceled = false
		task.completed = true
		task.failed = true
		task.progress = 1
		task.status = event.Status
		if !task.startedAt.IsZero() {
			task.duration = s.now().Sub(task.startedAt)
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
			task.duration = s.now().Sub(task.startedAt)
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
