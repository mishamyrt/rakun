package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"rakun/internal/config"
	"rakun/internal/git"
	"rakun/internal/providers/github"
	"rakun/internal/providers/gitlab"
	"rakun/internal/taskrun"
	"rakun/internal/tui"
	"runtime"
	"syscall"

	"github.com/spf13/pflag"
)

var (
	dryRun     bool
	configPath string
	jobs       int
	outputPath string
)

func init() {
	pflag.BoolVarP(&dryRun, "dry-run", "d", false,
		"Print collected repositories list without cloning")
	pflag.StringVarP(&configPath, "config", "c", "rakun.config.yaml",
		"Set config path")
	pflag.IntVarP(
		&jobs,
		"jobs", "j",
		runtime.NumCPU(),
		"Number of repositories to synchronize in parallel",
	)
	pflag.StringVarP(
		&outputPath,
		"output", "o",
		".",
		"Directory for output archives; defaults to the current working directory")
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pflag.Parse()
	appConfig, err := config.Load(configPath)
	if err != nil {
		log.Fatal("Cannot read config file", err)
	}

	if outputPath == "" {
		outputPath = "."
	}
	if outputPath == "." {
		currentDir, err := os.Getwd()
		if err != nil {
			log.Fatal("Cannot determine current working directory", err)
		}
		outputPath = currentDir
	}

	builder, err := git.NewTaskBuilder(outputPath)
	if err != nil {
		log.Fatal("Cannot initialize task builder", err)
	}

	stopResolvingSpinner := tui.StartSpinner("Resolving repositories")
	tasks, err := collectTasks(ctx, appConfig.Groups, builder)
	stopResolvingSpinner()
	if err != nil {
		log.Fatal("Cannot collect tasks", err)
	}
	if dryRun {
		for _, task := range tasks {
			fmt.Println(task.Title())
		}
		return
	}

	observer := tui.New(len(tasks), jobs, cancel)
	_, executeErr := taskrun.Execute(ctx, tasks, jobs, observer)
	flushErr := builder.Flush()
	closeErr := observer.Close()
	if closeErr != nil {
		log.Fatal("Cannot close terminal UI", closeErr)
	}
	if errors.Is(executeErr, context.Canceled) {
		if flushErr != nil {
			log.Fatal("Cannot persist synchronized state", flushErr)
		}
		os.Exit(130)
	}
	if err := errors.Join(executeErr, flushErr); err != nil {
		log.Fatal("Cannot synchronize repositories", err)
	}
}

func collectTasks(ctx context.Context, groups []config.Group, builder *git.TaskBuilder) ([]taskrun.Task, error) {
	tasks := []taskrun.Task{}

	for _, group := range groups {
		switch group.Type {
		case "github":
			var api *github.API
			if len(group.Namespaces) > 0 {
				createdAPI, err := github.NewAPI(github.APIBaseURL(group.Domain), group.Token.Value)
				if err != nil {
					return nil, err
				}
				api = createdAPI
			}

			groupTasks, err := github.EmitTasks(ctx, api, group, builder)
			if err != nil {
				return nil, err
			}
			tasks = append(tasks, groupTasks...)
		case "gitlab":
			var api *gitlab.API
			if len(group.Namespaces) > 0 {
				createdAPI, err := gitlab.NewAPI(gitlab.APIBaseURL(group.Domain), group.Token.Value)
				if err != nil {
					return nil, err
				}
				api = createdAPI
			}

			groupTasks, err := gitlab.EmitTasks(ctx, api, group, builder)
			if err != nil {
				return nil, err
			}
			tasks = append(tasks, groupTasks...)
		default:
			return nil, fmt.Errorf("unsupported source type %q", group.Type)
		}
	}

	return tasks, nil
}
