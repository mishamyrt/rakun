package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"rakun/internal/config"
	"rakun/internal/providers/git"
	"rakun/internal/providers/github"
	"rakun/internal/providers/gitlab"
	"rakun/internal/taskrun"
	"rakun/internal/tui"
	"runtime"
	"syscall"
)

var (
	print      bool
	configPath string
	jobs       int
	outputPath string
)

func init() {
	flag.BoolVar(&print, "p", false,
		"Print collected repositories list without cloning")
	flag.StringVar(&configPath, "c", "rakun.config.yaml",
		"Set config path")
	flag.IntVar(&jobs, "j", runtime.NumCPU(), "Number of repositories to synchronize in parallel")
	flag.IntVar(&jobs, "jobs", 4, "Number of repositories to synchronize in parallel")
	flag.StringVar(&outputPath, "o", ".", "Directory for output archives; defaults to the current working directory")
	flag.StringVar(&outputPath, "output", ".", "Directory for output archives; defaults to the current working directory")
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	flag.Parse()
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
	if print {
		for _, task := range tasks {
			fmt.Println(task.Title())
		}
		return
	}

	observer := tui.New(len(tasks), jobs, cancel)
	_, executeErr := taskrun.Execute(ctx, tasks, jobs, observer)
	closeErr := observer.Close()
	if closeErr != nil {
		log.Fatal("Cannot close terminal UI", closeErr)
	}
	if errors.Is(executeErr, context.Canceled) {
		os.Exit(130)
	}
	if executeErr != nil {
		log.Fatal("Cannot synchronize repositories", executeErr)
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
