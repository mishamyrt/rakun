package git

import (
	"context"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGitCommand(t *testing.T, dir string, args ...string) string {
	t.Helper()

	command := args[0]
	output, err := ExecGit(context.Background(), command, dir, args[1:])
	if err != nil {
		t.Fatalf("git %s failed: %v (%s)", strings.Join(args, " "), err, output)
	}
	return output
}

func createAuthenticatedHTTPRemoteRepository(t *testing.T, name string, token string) (string, string) {
	t.Helper()

	rootDir := t.TempDir()
	remoteDir := filepath.Join(rootDir, name+".git")
	workDir := filepath.Join(rootDir, name)

	runGitCommand(t, rootDir, "init", "--bare", remoteDir)
	runGitCommand(t, rootDir, "clone", remoteDir, workDir)
	runGitCommand(t, workDir, "config", "user.email", "rakun@example.com")
	runGitCommand(t, workDir, "config", "user.name", "Rakun Test")
	runGitCommand(t, workDir, "config", "commit.gpgsign", "false")

	filePath := filepath.Join(workDir, "README.md")
	if err := os.WriteFile(filePath, []byte("first version\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGitCommand(t, workDir, "add", "README.md")
	runGitCommand(t, workDir, "commit", "-m", "Initial commit")
	runGitCommand(t, workDir, "push", "origin", "HEAD")

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git executable: %v", err)
	}

	backend := &cgi.Handler{
		Path: gitPath,
		Args: []string{"http-backend"},
		Env: []string{
			"GIT_PROJECT_ROOT=" + rootDir,
			"GIT_HTTP_EXPORT_ALL=1",
		},
		Stderr: os.Stderr,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "git" || password != token {
			w.Header().Set("WWW-Authenticate", `Basic realm="rakun-test"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		backend.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)

	return server.URL + "/" + name + ".git", workDir
}

func TestResolveRemoteHeadRequiresCredentialsForHTTP(t *testing.T) {
	remoteURL, _ := createAuthenticatedHTTPRemoteRepository(t, "protected", "secret-token")

	if _, err := ResolveRemoteHead(context.Background(), remoteURL, nil); err == nil {
		t.Fatal("expected ls-remote to fail without credentials")
	}

	head, err := ResolveRemoteHead(context.Background(), remoteURL, NewTokenCredentials("secret-token"))
	if err != nil {
		t.Fatalf("resolve remote head: %v", err)
	}
	if head.Branch == "" || head.Commit == "" {
		t.Fatalf("unexpected remote head: %#v", head)
	}
}
