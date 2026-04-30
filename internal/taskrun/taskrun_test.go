package taskrun

import (
	"context"
	"errors"
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
