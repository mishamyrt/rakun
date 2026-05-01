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
	"sync/atomic"
	"testing"
	"time"
)

type recordedReporter struct {
	statuses []string
}

func (r *recordedReporter) Stage(_ float64, status string) {
	r.statuses = append(r.statuses, status)
}

func runGitCommand(t *testing.T, dir string, args ...string) string {
	t.Helper()

	command := args[0]
	output, err := ExecGit(context.Background(), command, dir, args[1:])
	if err != nil {
		t.Fatalf("git %s failed: %v (%s)", strings.Join(args, " "), err, output)
	}
	return output
}

func createRemoteRepository(t *testing.T, name string) (string, string, string) {
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

	remoteURL := "file://" + remoteDir
	return remoteURL, remoteDir, workDir
}

func createAuthenticatedHTTPRemoteRepositoryWithFetchFailure(t *testing.T, name string, token string) (string, string, *atomic.Bool) {
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

	failFetch := &atomic.Bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "git" || password != token {
			w.Header().Set("WWW-Authenticate", `Basic realm="rakun-test"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if failFetch.Load() && r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/git-upload-pack") {
			http.Error(w, "upload-pack unavailable", http.StatusServiceUnavailable)
			return
		}
		backend.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)

	return server.URL + "/" + name + ".git", workDir, failFetch
}

func pushCommit(t *testing.T, workDir string, contents string) string {
	t.Helper()

	filePath := filepath.Join(workDir, "README.md")
	if err := os.WriteFile(filePath, []byte(contents), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGitCommand(t, workDir, "add", "README.md")
	runGitCommand(t, workDir, "commit", "-m", "Update")
	runGitCommand(t, workDir, "push", "origin", "HEAD")
	return runGitCommand(t, workDir, "rev-parse", "HEAD")
}

func archiveFilePath(t *testing.T, output string, remote string) string {
	t.Helper()

	spec, err := ParseRemote(remote)
	if err != nil {
		t.Fatalf("parse remote: %v", err)
	}
	return filepath.Join(output, spec.ArchiveRelativePath)
}

func runSyncTasks(t *testing.T, output string, jobs int, remotes ...string) {
	t.Helper()

	builder, err := NewTaskBuilder(output)
	if err != nil {
		t.Fatalf("new task builder: %v", err)
	}
	tasks := builder.EmitRemoteTasks(remotes)
	if _, err := taskrun.Execute(context.Background(), tasks, jobs, nil); err != nil {
		t.Fatalf("execute sync tasks: %v", err)
	}
	if err := builder.Flush(); err != nil {
		t.Fatalf("flush sync state: %v", err)
	}
}

func TestSyncCreatesAndUpdatesArchives(t *testing.T) {
	output := t.TempDir()
	remoteURL, _, workDir := createRemoteRepository(t, "project")

	runSyncTasks(t, output, 2, remoteURL)

	archivePath := archiveFilePath(t, output, remoteURL)
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("archive missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(output, IndexFileName)); err != nil {
		t.Fatalf("index missing: %v", err)
	}

	firstInfo, err := os.Stat(archivePath)
	if err != nil {
		t.Fatalf("stat archive: %v", err)
	}

	time.Sleep(1100 * time.Millisecond)
	runSyncTasks(t, output, 2, remoteURL)
	secondInfo, err := os.Stat(archivePath)
	if err != nil {
		t.Fatalf("stat archive after second sync: %v", err)
	}
	if !firstInfo.ModTime().Equal(secondInfo.ModTime()) {
		t.Fatalf("archive should not be rewritten when remote is unchanged")
	}

	time.Sleep(1100 * time.Millisecond)
	newCommit := pushCommit(t, workDir, "second version\n")
	runSyncTasks(t, output, 2, remoteURL)
	thirdInfo, err := os.Stat(archivePath)
	if err != nil {
		t.Fatalf("stat archive after update: %v", err)
	}
	if !thirdInfo.ModTime().After(secondInfo.ModTime()) {
		t.Fatalf("archive should be rewritten after a remote update")
	}

	index, err := LoadIndex(output)
	if err != nil {
		t.Fatalf("load index: %v", err)
	}
	spec, err := ParseRemote(remoteURL)
	if err != nil {
		t.Fatalf("parse remote: %v", err)
	}
	if index.Repositories[spec.ArchiveRelativePath].Commit != newCommit {
		t.Fatalf("index commit mismatch: %#v", index.Repositories[spec.ArchiveRelativePath])
	}
}

func TestSyncRecoversFromMissingIndexAndBrokenArchive(t *testing.T) {
	output := t.TempDir()
	remoteURL, _, workDir := createRemoteRepository(t, "recoverable")

	runSyncTasks(t, output, 2, remoteURL)

	if err := os.Remove(filepath.Join(output, IndexFileName)); err != nil {
		t.Fatalf("remove index: %v", err)
	}
	runSyncTasks(t, output, 2, remoteURL)

	archivePath := archiveFilePath(t, output, remoteURL)
	if err := os.WriteFile(archivePath, []byte("broken archive"), 0644); err != nil {
		t.Fatalf("break archive: %v", err)
	}
	pushCommit(t, workDir, "third version\n")
	runSyncTasks(t, output, 2, remoteURL)

	if _, err := ExtractArchive(archivePath, t.TempDir()); err != nil {
		t.Fatalf("archive should be recreated after failure: %v", err)
	}
}

func TestSyncReturnsFetchErrorWithoutRecloning(t *testing.T) {
	const token = "secret-token"

	output := t.TempDir()
	remoteURL, workDir, failFetch := createAuthenticatedHTTPRemoteRepositoryWithFetchFailure(t, "project", token)
	target := RemoteTarget{
		URL:         remoteURL,
		Credentials: NewTokenCredentials(token),
	}

	runSyncTargets(t, output, 1, target)

	archivePath := archiveFilePath(t, output, remoteURL)
	beforeInfo, err := os.Stat(archivePath)
	if err != nil {
		t.Fatalf("stat archive before failed sync: %v", err)
	}

	time.Sleep(1100 * time.Millisecond)
	pushCommit(t, workDir, "second version\n")
	failFetch.Store(true)

	builder, err := NewTaskBuilder(output)
	if err != nil {
		t.Fatalf("new task builder: %v", err)
	}
	task := builder.EmitRemoteTarget(target)
	if task == nil {
		t.Fatal("expected sync task")
	}

	reporter := &recordedReporter{}
	result := task.Run(context.Background(), reporter)
	if result.Error == nil {
		t.Fatal("expected sync to fail when fetch is unavailable")
	}
	for _, status := range reporter.statuses {
		if status == "Archive invalid, recloning" || status == "Cloning repository" {
			t.Fatalf("unexpected rebuild fallback after fetch failure: %v", reporter.statuses)
		}
	}

	afterInfo, err := os.Stat(archivePath)
	if err != nil {
		t.Fatalf("stat archive after failed sync: %v", err)
	}
	if !beforeInfo.ModTime().Equal(afterInfo.ModTime()) {
		t.Fatal("archive should not be rewritten when sync fails")
	}
}

func TestTaskRunnerRejectsInvalidJobs(t *testing.T) {
	if _, err := taskrun.Execute(context.Background(), nil, 0, nil); err == nil {
		t.Fatal("expected jobs validation error")
	}
}
