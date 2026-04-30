package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadGroupedFormatWithEnvToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "secret-token")

	path := writeConfigFile(t, `- domain: github.com
  type: github
  token: !env $GITHUB_TOKEN
  namespaces:
    mishamyrt:
      skip:
        - private-repo
    Paulownia-Group:
  repos:
    - ghostty-org/ghostty
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if len(cfg.Groups) != 1 {
		t.Fatalf("unexpected groups count: %d", len(cfg.Groups))
	}
	group := cfg.Groups[0]
	if group.Token.Value != "secret-token" {
		t.Fatalf("unexpected token value: %q", group.Token.Value)
	}
	if group.Namespaces["mishamyrt"] == nil || len(group.Namespaces["mishamyrt"].Skip) != 1 {
		t.Fatalf("unexpected namespace skip config: %#v", group.Namespaces["mishamyrt"])
	}
	if group.Namespaces["Paulownia-Group"] != nil {
		t.Fatalf("expected nil config for empty namespace entry, got %#v", group.Namespaces["Paulownia-Group"])
	}
	if len(group.Repos) != 1 || group.Repos[0] != "ghostty-org/ghostty" {
		t.Fatalf("unexpected repos: %#v", group.Repos)
	}
}

func TestLoadReposOnlyWithoutToken(t *testing.T) {
	path := writeConfigFile(t, `- domain: github.example.com
  type: github
  repos:
    - example/project
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if len(cfg.Groups) != 1 {
		t.Fatalf("unexpected groups count: %d", len(cfg.Groups))
	}
	if cfg.Groups[0].Token.Set {
		t.Fatalf("expected token to be unset: %#v", cfg.Groups[0].Token)
	}
}

func TestLoadGitLabGroupedFormatWithNestedPaths(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "secret-token")

	path := writeConfigFile(t, `- domain: gitlab.example.com
  type: gitlab
  token: !env $GITLAB_TOKEN
  namespaces:
    platform/core:
      skip:
        - app
        - platform/core/tools/cli
  repos:
    - platform/core/api
    - platform/core/tools/cli
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if len(cfg.Groups) != 1 {
		t.Fatalf("unexpected groups count: %d", len(cfg.Groups))
	}
	group := cfg.Groups[0]
	if group.Token.Value != "secret-token" {
		t.Fatalf("unexpected token value: %q", group.Token.Value)
	}
	if group.Namespaces["platform/core"] == nil || len(group.Namespaces["platform/core"].Skip) != 2 {
		t.Fatalf("unexpected namespace skip config: %#v", group.Namespaces["platform/core"])
	}
	if len(group.Repos) != 2 || group.Repos[0] != "platform/core/api" || group.Repos[1] != "platform/core/tools/cli" {
		t.Fatalf("unexpected repos: %#v", group.Repos)
	}
}

func TestLoadRejectsOutputInLegacyRoot(t *testing.T) {
	path := writeConfigFile(t, `output: backups
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected output validation error")
	}
	if !strings.Contains(err.Error(), "use -o or --output") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsLegacySourcesFormat(t *testing.T) {
	path := writeConfigFile(t, `sources:
  - type: github-user
    url: https://github.com/example
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected legacy format error")
	}
	if !strings.Contains(err.Error(), "legacy config format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsNamespacesWithoutToken(t *testing.T) {
	path := writeConfigFile(t, `- domain: github.com
  type: github
  namespaces:
    example:
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "token is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsGitLabNamespacesWithoutToken(t *testing.T) {
	path := writeConfigFile(t, `- domain: gitlab.com
  type: gitlab
  namespaces:
    platform/core:
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "token is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsMissingEnvToken(t *testing.T) {
	path := writeConfigFile(t, `- domain: github.com
  type: github
  token: !env $MISSING_TOKEN
  namespaces:
    example:
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected env lookup error")
	}
	if !strings.Contains(err.Error(), "MISSING_TOKEN is not set") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsInvalidRepoRef(t *testing.T) {
	path := writeConfigFile(t, `- domain: github.com
  type: github
  repos:
    - https://github.com/example/project.git
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected repo validation error")
	}
	if !strings.Contains(err.Error(), "owner/repo format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsInvalidGitLabRepoRef(t *testing.T) {
	path := writeConfigFile(t, `- domain: gitlab.com
  type: gitlab
  repos:
    - project
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected repo validation error")
	}
	if !strings.Contains(err.Error(), "GitLab project path") {
		t.Fatalf("unexpected error: %v", err)
	}
}
