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

// func printGroups(groups []git.RepoGroup) {
// 	for _, group := range groups {
// 		fmt.Println("Directory: " + group.Dir)
// 		fmt.Println("Repositories: ")
// 		for _, repo := range group.Repositories {
// 			fmt.Println("  Name: " + repo.Name)
// 			fmt.Println("  URL:  " + repo.URL)
// 		}
// 	}
// }

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

	// groups := []git.RepoGroup{}
	// if appConfig.Github.Token != "" {
	// 	log.Println("Collecting GitHub repositories...")
	// 	githubGroups, err := github.CollectRepositories(appConfig.Github)
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	groups = append(groups, githubGroups...)
	// }
	// if print {
	// 	printGroups(groups)
	// 	os.Exit(0)
	// }
	// git.SyncRepos(appConfig.Path, groups)
}
