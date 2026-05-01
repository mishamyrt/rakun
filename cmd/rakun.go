package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"rakun/internal/config"
	"rakun/internal/rakun"
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

	app, err := rakun.New(outputPath, jobs)
	if err != nil {
		log.Fatal("Cannot initialize rakun", err)
	}

	stopResolvingSpinner := tui.StartSpinner("Resolving repositories")
	tasks, err := app.Collect(ctx, appConfig.Groups)
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
	_, runErr := app.Run(ctx, tasks, observer)
	closeErr := observer.Close()
	if closeErr != nil {
		log.Fatal("Cannot close terminal UI", closeErr)
	}
	if errors.Is(runErr, context.Canceled) {
		os.Exit(130)
	}
	if runErr != nil {
		log.Fatal("Cannot synchronize repositories", runErr)
	}
}
