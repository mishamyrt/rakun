package main

import (
	"flag"
	"log"
	"path/filepath"
	"rakun/internal/config"
	"rakun/providers/git"
	"rakun/providers/github"
)

var (
	print      bool
	configPath string
)

func init() {
	flag.BoolVar(&print, "p", false,
		"Print collected repositories list without cloning")
	flag.StringVar(&configPath, "c", "config.yaml",
		"Set config path")
}

func main() {
	flag.Parse()
	appConfig, err := config.Load(configPath)
	if err != nil {
		log.Fatal("Cannot read config file", err)
	}

	if len(appConfig.Git) > 0 {
		git.Sync(filepath.Join(appConfig.Path, "git"), appConfig.Git)
	}
	github.Sync(appConfig.Path, appConfig.Github)
}
