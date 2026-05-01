package tui

import (
	"rakun/internal/taskrun"
	"testing"
	"time"
)

func TestModelHandleTaskEvent(t *testing.T) {
	startedAt := time.Date(2026, time.May, 2, 10, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(3*time.Second + 400*time.Millisecond)

	testCases := []struct {
		name    string
		prepare func(*model)
		event   taskrun.Event
		assert  func(*testing.T, model)
	}{
		{
			name: "started event initializes task state",
			event: taskrun.Event{
				Kind:   taskrun.EventStarted,
				TaskID: "alpha",
				Title:  "/repos/alpha",
			},
			assert: func(t *testing.T, got model) {
				task := got.taskStates["alpha"]
				if task == nil {
					t.Fatal("expected task state to be created")
				}
				if len(got.order) != 1 || got.order[0] != "alpha" {
					t.Fatalf("unexpected task order: %#v", got.order)
				}
				if task.title != "/repos/alpha" {
					t.Fatalf("unexpected title: %q", task.title)
				}
				if task.status != "Starting" {
					t.Fatalf("unexpected status: %q", task.status)
				}
				if task.progress != 0.02 {
					t.Fatalf("unexpected progress: %v", task.progress)
				}
				if !task.startedAt.Equal(startedAt) {
					t.Fatalf("unexpected startedAt: %v", task.startedAt)
				}
				if task.completed {
					t.Fatal("expected task to remain active")
				}
			},
		},
		{
			name: "progress event clamps and updates task title",
			prepare: func(m *model) {
				m.ensureTask("beta", "/repos/old-beta")
			},
			event: taskrun.Event{
				Kind:     taskrun.EventProgress,
				TaskID:   "beta",
				Title:    "/repos/new-beta",
				Progress: 1.5,
				Status:   "Fetching",
			},
			assert: func(t *testing.T, got model) {
				task := got.taskStates["beta"]
				if task == nil {
					t.Fatal("expected task state to exist")
				}
				if task.title != "/repos/new-beta" {
					t.Fatalf("unexpected title: %q", task.title)
				}
				if task.progress != 1 {
					t.Fatalf("unexpected progress: %v", task.progress)
				}
				if task.status != "Fetching" {
					t.Fatalf("unexpected status: %q", task.status)
				}
			},
		},
		{
			name: "completed event records success and duration",
			prepare: func(m *model) {
				m.taskStates["gamma"] = &taskState{
					progress:  0.6,
					startedAt: startedAt,
					status:    "Running",
					title:     "/repos/gamma",
				}
				m.order = []string{"gamma"}
			},
			event: taskrun.Event{
				Kind:   taskrun.EventCompleted,
				TaskID: "gamma",
				Status: "Done",
			},
			assert: func(t *testing.T, got model) {
				task := got.taskStates["gamma"]
				if task == nil {
					t.Fatal("expected task state to exist")
				}
				if !task.completed || task.failed || task.canceled {
					t.Fatalf("unexpected flags: %+v", task)
				}
				if task.progress != 1 {
					t.Fatalf("unexpected progress: %v", task.progress)
				}
				if task.status != "Done" {
					t.Fatalf("unexpected status: %q", task.status)
				}
				if task.duration != finishedAt.Sub(startedAt) {
					t.Fatalf("unexpected duration: %s", task.duration)
				}
				if got.completed != 1 {
					t.Fatalf("unexpected completed count: %d", got.completed)
				}
				if got.failed != 0 {
					t.Fatalf("unexpected failed count: %d", got.failed)
				}
			},
		},
		{
			name: "failed event records failure counters",
			prepare: func(m *model) {
				m.taskStates["delta"] = &taskState{
					startedAt: startedAt,
					title:     "/repos/delta",
				}
				m.order = []string{"delta"}
			},
			event: taskrun.Event{
				Kind:   taskrun.EventFailed,
				TaskID: "delta",
				Status: "Permission denied",
			},
			assert: func(t *testing.T, got model) {
				task := got.taskStates["delta"]
				if task == nil {
					t.Fatal("expected task state to exist")
				}
				if !task.completed || !task.failed || task.canceled {
					t.Fatalf("unexpected flags: %+v", task)
				}
				if task.status != "Permission denied" {
					t.Fatalf("unexpected status: %q", task.status)
				}
				if got.completed != 1 {
					t.Fatalf("unexpected completed count: %d", got.completed)
				}
				if got.failed != 1 {
					t.Fatalf("unexpected failed count: %d", got.failed)
				}
			},
		},
		{
			name: "canceled event marks task canceled without failure",
			prepare: func(m *model) {
				m.taskStates["epsilon"] = &taskState{
					startedAt: startedAt,
					title:     "/repos/epsilon",
				}
				m.order = []string{"epsilon"}
			},
			event: taskrun.Event{
				Kind:   taskrun.EventCanceled,
				TaskID: "epsilon",
				Status: "Canceled",
			},
			assert: func(t *testing.T, got model) {
				task := got.taskStates["epsilon"]
				if task == nil {
					t.Fatal("expected task state to exist")
				}
				if !task.completed || task.failed || !task.canceled {
					t.Fatalf("unexpected flags: %+v", task)
				}
				if task.status != "Canceled" {
					t.Fatalf("unexpected status: %q", task.status)
				}
				if got.completed != 1 {
					t.Fatalf("unexpected completed count: %d", got.completed)
				}
				if got.failed != 0 {
					t.Fatalf("unexpected failed count: %d", got.failed)
				}
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newModel(5, 2, nil)
			m.now = func() time.Time {
				return startedAt
			}
			if testCase.prepare != nil {
				testCase.prepare(&m)
			}
			if testCase.event.Kind == taskrun.EventCompleted ||
				testCase.event.Kind == taskrun.EventFailed ||
				testCase.event.Kind == taskrun.EventCanceled {
				m.now = func() time.Time {
					return finishedAt
				}
			}

			m.handleTaskEvent(testCase.event)
			testCase.assert(t, m)
		})
	}
}
