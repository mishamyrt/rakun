package git

import (
	"context"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"rakun/internal/taskrun"
	"strings"
	"testing"
)

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

func runSyncTargets(t *testing.T, output string, jobs int, targets ...RemoteTarget) {
	t.Helper()

	builder, err := NewTaskBuilder(output)
	if err != nil {
		t.Fatalf("new task builder: %v", err)
	}
	tasks := builder.EmitRemoteTargets(targets)
	if _, err := taskrun.Execute(context.Background(), tasks, jobs, nil); err != nil {
		t.Fatalf("execute sync tasks: %v", err)
	}
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

func TestSyncUsesHTTPTokenWithoutPersistingIt(t *testing.T) {
	const token = "secret-token"

	output := t.TempDir()
	remoteURL, workDir := createAuthenticatedHTTPRemoteRepository(t, "project", token)
	target := RemoteTarget{
		URL:         remoteURL,
		Credentials: NewTokenCredentials(token),
	}

	runSyncTargets(t, output, 1, target)
	pushCommit(t, workDir, "second version\n")
	runSyncTargets(t, output, 1, target)

	archivePath := archiveFilePath(t, output, remoteURL)
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("archive missing: %v", err)
	}

	index, err := LoadIndex(output)
	if err != nil {
		t.Fatalf("load index: %v", err)
	}
	spec, err := ParseRemote(remoteURL)
	if err != nil {
		t.Fatalf("parse remote: %v", err)
	}
	state := index.Repositories[spec.ArchiveRelativePath]
	if state.Remote != remoteURL {
		t.Fatalf("unexpected stored remote: %#v", state)
	}

	indexBytes, err := os.ReadFile(filepath.Join(output, IndexFileName))
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	if strings.Contains(string(indexBytes), token) {
		t.Fatal("token leaked into index file")
	}

	extractDir := t.TempDir()
	repoPath, err := ExtractArchive(archivePath, extractDir)
	if err != nil {
		t.Fatalf("extract archive: %v", err)
	}
	originURL, err := ExecGit(context.Background(), "remote", repoPath, []string{"get-url", "origin"})
	if err != nil {
		t.Fatalf("read origin url: %v", err)
	}
	if originURL != remoteURL {
		t.Fatalf("unexpected origin url: %q", originURL)
	}
	if strings.Contains(originURL, token) {
		t.Fatal("token leaked into git origin url")
	}
}
