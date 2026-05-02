package taskrun

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type blockingTask struct {
	id      string
	started chan struct{}
}

func (s blockingTask) ID() string {
	return s.id
}

func (s blockingTask) Title() string {
	return s.id
}

func (s blockingTask) Run(ctx context.Context, reporter Reporter) Result {
	reporter.Stage(0.1, "Started")
	close(s.started)
	<-ctx.Done()
	return Result{
		Error:   ctx.Err(),
		Summary: "Canceled",
	}
}

type stageCall struct {
	progress float64
	status   string
}

type stubTask struct {
	id     string
	title  string
	result Result
	stages []stageCall
}

func (s stubTask) ID() string {
	return s.id
}

func (s stubTask) Title() string {
	return s.title
}

func (s stubTask) Run(_ context.Context, reporter Reporter) Result {
	for _, stage := range s.stages {
		reporter.Stage(stage.progress, stage.status)
	}
	return s.result
}

type recordingReporter struct {
	stages []stageCall
}

func (s *recordingReporter) Stage(progress float64, status string) {
	s.stages = append(s.stages, stageCall{progress: progress, status: status})
}

type recordingObserver struct {
	mu      sync.Mutex
	events  []Event
	summary Summary
}

func (s *recordingObserver) HandleEvent(event Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
}

func (s *recordingObserver) Finish(summary Summary) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.summary = summary
}

func (s *recordingObserver) Close() error {
	return nil
}

func TestNewErrorTask(t *testing.T) {
	reporter := &recordingReporter{}
	taskErr := errors.New("invalid config")
	task := NewErrorTask("config", "Config validation", taskErr)

	if task.ID() != "config" {
		t.Fatalf("unexpected ID: %q", task.ID())
	}
	if task.Title() != "Config validation" {
		t.Fatalf("unexpected title: %q", task.Title())
	}

	result := task.Run(context.Background(), reporter)
	if !errors.Is(result.Error, taskErr) {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Summary != "Configuration error" {
		t.Fatalf("unexpected summary: %q", result.Summary)
	}

	if !reflect.DeepEqual(reporter.stages, []stageCall{{progress: 1, status: "Configuration error"}}) {
		t.Fatalf("unexpected reporter stages: %#v", reporter.stages)
	}
}

func TestNoopObserver(t *testing.T) {
	var observer noopObserver

	observer.HandleEvent(Event{Kind: EventStarted})
	observer.Finish(Summary{Total: 1})
	if err := observer.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
}

func TestExecuteCancelsRunningAndQueuedTasks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	started := make(chan struct{})
	tasks := []Task{
		blockingTask{id: "first", started: started},
		blockingTask{id: "second", started: make(chan struct{})},
	}

	done := make(chan struct{})
	var (
		err     error
		summary Summary
	)
	go func() {
		summary, err = Execute(ctx, tasks, 1, nil)
		close(done)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("task did not start")
	}
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("execute did not stop after cancel")
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if summary.Canceled != 2 {
		t.Fatalf("expected 2 canceled tasks, got %d", summary.Canceled)
	}
	if summary.Failed != 0 {
		t.Fatalf("expected no failed tasks, got %d", summary.Failed)
	}
}

func TestExecuteSummarizesSuccessAndFailure(t *testing.T) {
	tasks := []Task{
		stubTask{
			id:    "changed",
			title: "Changed repo",
			stages: []stageCall{
				{progress: 0.25, status: "Cloning"},
			},
			result: Result{
				Changed: true,
				Summary: "Updated",
			},
		},
		stubTask{
			id:    "unchanged",
			title: "Unchanged repo",
			result: Result{
				Summary: "Already up to date",
			},
		},
		stubTask{
			id:    "failed",
			title: "Failed repo",
			result: Result{
				Error:   errors.New("network down"),
				Summary: "Failed",
			},
		},
	}

	observer := &recordingObserver{}
	summary, err := Execute(context.Background(), tasks, 1, observer)
	if err == nil {
		t.Fatal("expected aggregate error")
	}
	if !strings.Contains(err.Error(), "failed to synchronize 1 repositories: Failed repo") {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary.Total != 3 {
		t.Fatalf("unexpected total: %d", summary.Total)
	}
	if summary.Succeeded != 2 {
		t.Fatalf("unexpected succeeded count: %d", summary.Succeeded)
	}
	if summary.Failed != 1 {
		t.Fatalf("unexpected failed count: %d", summary.Failed)
	}
	if summary.Changed != 1 {
		t.Fatalf("unexpected changed count: %d", summary.Changed)
	}
	if summary.Unchanged != 1 {
		t.Fatalf("unexpected unchanged count: %d", summary.Unchanged)
	}
	if summary.Canceled != 0 {
		t.Fatalf("unexpected canceled count: %d", summary.Canceled)
	}
	if len(summary.Outcomes) != 3 {
		t.Fatalf("unexpected outcomes count: %d", len(summary.Outcomes))
	}

	outcomes := map[string]TaskSummary{}
	for _, outcome := range summary.Outcomes {
		outcomes[outcome.ID] = outcome
	}
	if outcomes["changed"].Summary != "Updated" || !outcomes["changed"].Changed {
		t.Fatalf("unexpected changed outcome: %#v", outcomes["changed"])
	}
	if outcomes["unchanged"].Summary != "Already up to date" || outcomes["unchanged"].Changed {
		t.Fatalf("unexpected unchanged outcome: %#v", outcomes["unchanged"])
	}
	if outcomes["failed"].Error != "network down" {
		t.Fatalf("unexpected failed outcome: %#v", outcomes["failed"])
	}

	if observer.summary.Total != summary.Total || observer.summary.Failed != summary.Failed {
		t.Fatalf("unexpected observer summary: %#v", observer.summary)
	}

	gotKinds := make([]EventKind, 0, len(observer.events))
	for _, event := range observer.events {
		gotKinds = append(gotKinds, event.Kind)
	}
	wantKinds := []EventKind{
		EventStarted, EventProgress, EventCompleted,
		EventStarted, EventCompleted,
		EventStarted, EventFailed,
	}
	if !reflect.DeepEqual(gotKinds, wantKinds) {
		t.Fatalf("unexpected event kinds: %#v", gotKinds)
	}
}

func TestExecuteRejectsInvalidJobs(t *testing.T) {
	_, err := Execute(context.Background(), nil, 0, nil)
	if err == nil {
		t.Fatal("expected invalid jobs error")
	}
	if !strings.Contains(err.Error(), "jobs must be at least 1") {
		t.Fatalf("unexpected error: %v", err)
	}
}
