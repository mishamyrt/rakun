package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"git_sync/internal/config"
	"git_sync/internal/git"
	"git_sync/internal/github"
	"log"
	"net/http"
	"os"
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

func printGroups(groups []git.RepoGroup) {
	for _, group := range groups {
		fmt.Println("Directory: " + group.Dir)
		fmt.Println("Repositories: ")
		for _, repo := range group.Repositories {
			fmt.Println("  Name: " + repo.Name)
			fmt.Println("  URL:  " + repo.URL)
		}
	}
}

func main() {
	// TODO: Make insecure build target
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	flag.Parse()
	appConfig, err := config.Load(configPath)
	if err != nil {
		log.Fatal("Cannot read config file", err)
	}

	groups := []git.RepoGroup{}
	if appConfig.Github.Token != "" {
		log.Println("Collecting GitHub repositories...")
		githubGroups, err := github.CollectRepositories(appConfig.Github)
		if err != nil {
			log.Fatal(err)
		}
		groups = append(groups, githubGroups...)
	}
	if print {
		printGroups(groups)
		os.Exit(0)
	}
	git.SyncRepos(appConfig.Path, groups)
}
