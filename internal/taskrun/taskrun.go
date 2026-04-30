package taskrun

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type EventKind string

const (
	EventStarted   EventKind = "started"
	EventProgress  EventKind = "progress"
	EventCompleted EventKind = "completed"
	EventFailed    EventKind = "failed"
	EventCanceled  EventKind = "canceled"
)

type Event struct {
	Kind     EventKind
	Progress float64
	Status   string
	TaskID   string
	Title    string
}

type TaskSummary struct {
	Canceled bool
	Changed  bool
	Duration time.Duration
	Error    string
	ID       string
	Summary  string
	Title    string
}

type Summary struct {
	Canceled  int
	Changed   int
	Duration  time.Duration
	Failed    int
	Outcomes  []TaskSummary
	StartedAt time.Time
	Succeeded int
	Total     int
	Unchanged int
}

type Reporter interface {
	Stage(progress float64, status string)
}

type Task interface {
	ID() string
	Title() string
	Run(ctx context.Context, reporter Reporter) Result
}

type Result struct {
	Changed bool
	Error   error
	Summary string
}

type Observer interface {
	HandleEvent(event Event)
	Finish(summary Summary)
	Close() error
}

type errorTask struct {
	err   error
	id    string
	title string
}

func NewErrorTask(id string, title string, err error) Task {
	return errorTask{
		err:   err,
		id:    id,
		title: title,
	}
}

func (s errorTask) ID() string {
	return s.id
}

func (s errorTask) Title() string {
	return s.title
}

func (s errorTask) Run(ctx context.Context, reporter Reporter) Result {
	reporter.Stage(1, "Configuration error")
	return Result{
		Error:   s.err,
		Summary: "Configuration error",
	}
}

type noopObserver struct{}

func (s noopObserver) HandleEvent(event Event) {}

func (s noopObserver) Finish(summary Summary) {}

func (s noopObserver) Close() error {
	return nil
}

type taskReporter struct {
	observer Observer
	task     Task
}

func Execute(ctx context.Context, tasks []Task, jobs int, observer Observer) (Summary, error) {
	if jobs < 1 {
		return Summary{}, fmt.Errorf("jobs must be at least 1")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if observer == nil {
		observer = noopObserver{}
	}

	startedAt := time.Now()
	results := make(chan TaskSummary, len(tasks))
	queue := make(chan Task)
	taskByID := make(map[string]Task, len(tasks))
	finishedTaskIDs := map[string]bool{}
	for _, task := range tasks {
		taskByID[task.ID()] = task
	}

	workerCount := jobs
	if workerCount > len(tasks) {
		workerCount = len(tasks)
	}

	var workers sync.WaitGroup
	for workerIndex := 0; workerIndex < workerCount; workerIndex++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case task, ok := <-queue:
					if !ok {
						return
					}
					observer.HandleEvent(Event{
						Kind:   EventStarted,
						Status: "Queued",
						TaskID: task.ID(),
						Title:  task.Title(),
					})

					reporter := taskReporter{
						observer: observer,
						task:     task,
					}
					taskStartedAt := time.Now()
					result := task.Run(ctx, reporter)

					outcome := TaskSummary{
						Changed:  result.Changed,
						Duration: time.Since(taskStartedAt),
						ID:       task.ID(),
						Summary:  result.Summary,
						Title:    task.Title(),
					}
					if isCanceled(result.Error) {
						outcome.Canceled = true
						outcome.Summary = "Canceled"
						observer.HandleEvent(Event{
							Kind:   EventCanceled,
							Status: "Canceled",
							TaskID: task.ID(),
							Title:  task.Title(),
						})
					} else if result.Error != nil {
						outcome.Error = result.Error.Error()
						observer.HandleEvent(Event{
							Kind:   EventFailed,
							Status: outcome.Error,
							TaskID: task.ID(),
							Title:  task.Title(),
						})
					} else {
						observer.HandleEvent(Event{
							Kind:   EventCompleted,
							Status: result.Summary,
							TaskID: task.ID(),
							Title:  task.Title(),
						})
					}
					results <- outcome
				}
			}
		}()
	}

	go func() {
		for _, task := range tasks {
			select {
			case <-ctx.Done():
				close(queue)
				workers.Wait()
				close(results)
				return
			case queue <- task:
			}
		}
		close(queue)
		workers.Wait()
		close(results)
	}()

	summary := Summary{
		StartedAt: startedAt,
		Total:     len(tasks),
	}
	var failures []string
	for outcome := range results {
		finishedTaskIDs[outcome.ID] = true
		summary.Outcomes = append(summary.Outcomes, outcome)
		if outcome.Canceled {
			summary.Canceled++
			continue
		}
		if outcome.Error != "" {
			summary.Failed++
			failures = append(failures, outcome.Title)
			continue
		}
		summary.Succeeded++
		if outcome.Changed {
			summary.Changed++
		} else {
			summary.Unchanged++
		}
	}
	if ctx.Err() != nil {
		for _, task := range tasks {
			if finishedTaskIDs[task.ID()] {
				continue
			}
			outcome := TaskSummary{
				Canceled: true,
				ID:       task.ID(),
				Summary:  "Canceled before start",
				Title:    task.Title(),
			}
			summary.Canceled++
			summary.Outcomes = append(summary.Outcomes, outcome)
		}
	}
	summary.Duration = time.Since(startedAt)
	observer.Finish(summary)

	if ctx.Err() != nil {
		return summary, ctx.Err()
	}
	if summary.Failed > 0 {
		return summary, fmt.Errorf("failed to synchronize %d repositories: %s", summary.Failed, strings.Join(failures, ", "))
	}
	return summary, nil
}

func (s taskReporter) Stage(progress float64, status string) {
	s.observer.HandleEvent(Event{
		Kind:     EventProgress,
		Progress: progress,
		Status:   status,
		TaskID:   s.task.ID(),
		Title:    s.task.Title(),
	})
}

func isCanceled(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
