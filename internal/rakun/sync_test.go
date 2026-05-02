package rakun

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"rakun/internal/archive"
	"rakun/internal/git"
	"strings"
	"sync/atomic"
	"testing"
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
	output, err := git.ExecGit(context.Background(), command, dir, args[1:])
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

func installGitCommandRecorder(t *testing.T) string {
	t.Helper()

	realGitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git executable: %v", err)
	}

	logPath := filepath.Join(t.TempDir(), "git-commands.log")
	wrapperDir := t.TempDir()
	wrapperPath := filepath.Join(wrapperDir, "git")
	wrapperScript := "#!/bin/sh\n" +
		"printf '%s\\n' \"$1\" >> \"$RAKUN_GIT_LOG\"\n" +
		"exec \"$RAKUN_GIT_REAL\" \"$@\"\n"
	if err := os.WriteFile(wrapperPath, []byte(wrapperScript), 0755); err != nil {
		t.Fatalf("write git wrapper: %v", err)
	}

	t.Setenv("RAKUN_GIT_REAL", realGitPath)
	t.Setenv("RAKUN_GIT_LOG", logPath)
	t.Setenv("PATH", wrapperDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	return logPath
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

	spec, err := git.ParseRemote(remote)
	if err != nil {
		t.Fatalf("parse remote: %v", err)
	}
	return filepath.Join(output, spec.ArchiveRelativePath)
}

func archiveDigest(t *testing.T, archivePath string) [32]byte {
	t.Helper()

	archiveBytes, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	return sha256.Sum256(archiveBytes)
}

func loadRepositoryState(t *testing.T, output string, remote string) RepositoryState {
	t.Helper()

	index, err := LoadIndex(output)
	if err != nil {
		t.Fatalf("load index: %v", err)
	}

	spec, err := git.ParseRemote(remote)
	if err != nil {
		t.Fatalf("parse remote: %v", err)
	}

	state, ok := index.Repositories[spec.ArchiveRelativePath]
	if !ok {
		t.Fatalf("missing repository state for %q", spec.ArchiveRelativePath)
	}
	return state
}

func runSyncTasks(t *testing.T, output string, jobs int, remotes ...string) {
	t.Helper()

	app, err := New(output, jobs)
	if err != nil {
		t.Fatalf("new rakun: %v", err)
	}
	tasks := app.emitRemoteTasks(remotes)
	if _, err := app.Run(context.Background(), tasks, nil); err != nil {
		t.Fatalf("execute sync tasks: %v", err)
	}
}

func runSyncTargets(t *testing.T, output string, jobs int, targets ...git.RemoteTarget) {
	t.Helper()

	app, err := New(output, jobs)
	if err != nil {
		t.Fatalf("new rakun: %v", err)
	}
	tasks := app.emitRemoteTargets(targets)
	if _, err := app.Run(context.Background(), tasks, nil); err != nil {
		t.Fatalf("execute sync tasks: %v", err)
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

	firstDigest := archiveDigest(t, archivePath)
	firstState := loadRepositoryState(t, output, remoteURL)

	runSyncTasks(t, output, 2, remoteURL)
	secondDigest := archiveDigest(t, archivePath)
	secondState := loadRepositoryState(t, output, remoteURL)
	if firstDigest != secondDigest {
		t.Fatalf("archive should not be rewritten when remote is unchanged")
	}
	if secondState.Commit != firstState.Commit {
		t.Fatalf("index commit changed unexpectedly: before=%q after=%q", firstState.Commit, secondState.Commit)
	}
	if !secondState.UpdatedAt.Equal(firstState.UpdatedAt) {
		t.Fatalf("index state should not be rewritten when remote is unchanged: before=%s after=%s", firstState.UpdatedAt, secondState.UpdatedAt)
	}

	newCommit := pushCommit(t, workDir, "second version\n")
	runSyncTasks(t, output, 2, remoteURL)
	thirdDigest := archiveDigest(t, archivePath)
	if thirdDigest == secondDigest {
		t.Fatalf("archive should be rewritten after a remote update")
	}

	thirdState := loadRepositoryState(t, output, remoteURL)
	if thirdState.Commit != newCommit {
		t.Fatalf("index commit mismatch: %#v", thirdState)
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

	if _, err := archive.ExtractArchive(archivePath, t.TempDir()); err != nil {
		t.Fatalf("archive should be recreated after failure: %v", err)
	}
}

func TestSyncReturnsFetchErrorWithoutRecloning(t *testing.T) {
	const token = "secret-token"

	output := t.TempDir()
	remoteURL, workDir, failFetch := createAuthenticatedHTTPRemoteRepositoryWithFetchFailure(t, "project", token)
	target := git.RemoteTarget{
		URL:         remoteURL,
		Credentials: git.NewTokenCredentials(token),
	}

	runSyncTargets(t, output, 1, target)

	archivePath := archiveFilePath(t, output, remoteURL)
	beforeDigest := archiveDigest(t, archivePath)
	beforeState := loadRepositoryState(t, output, remoteURL)

	newCommit := pushCommit(t, workDir, "second version\n")
	failFetch.Store(true)

	app, err := New(output, 1)
	if err != nil {
		t.Fatalf("new rakun: %v", err)
	}
	task := app.emitRemoteTarget(target)
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

	afterDigest := archiveDigest(t, archivePath)
	if beforeDigest != afterDigest {
		t.Fatal("archive should not be rewritten when sync fails")
	}
	afterState := loadRepositoryState(t, output, remoteURL)
	if afterState.Commit != beforeState.Commit {
		t.Fatalf("index commit changed unexpectedly after failed sync: before=%q after=%q remote=%q", beforeState.Commit, afterState.Commit, newCommit)
	}
}

func TestSyncInitialCloneSkipsRedundantFetch(t *testing.T) {
	output := t.TempDir()
	remoteURL, _, _ := createRemoteRepository(t, "project")
	commandLogPath := installGitCommandRecorder(t)

	runSyncTasks(t, output, 1, remoteURL)

	commandLog, err := os.ReadFile(commandLogPath)
	if err != nil {
		t.Fatalf("read git command log: %v", err)
	}
	commands := strings.Fields(string(commandLog))

	disallowed := map[string]int{
		"remote":   0,
		"fetch":    0,
		"checkout": 0,
		"reset":    0,
		"clean":    0,
	}
	for _, command := range commands {
		if _, tracked := disallowed[command]; tracked {
			disallowed[command]++
		}
	}

	for command, count := range disallowed {
		if count != 0 {
			t.Fatalf("expected initial sync to skip git %s, saw commands %v", command, commands)
		}
	}
}

func TestSyncUsesHTTPTokenWithoutPersistingIt(t *testing.T) {
	const token = "secret-token"

	output := t.TempDir()
	remoteURL, workDir, _ := createAuthenticatedHTTPRemoteRepositoryWithFetchFailure(t, "project", token)
	target := git.RemoteTarget{
		URL:         remoteURL,
		Credentials: git.NewTokenCredentials(token),
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
	spec, err := git.ParseRemote(remoteURL)
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
	repoPath, err := archive.ExtractArchive(archivePath, extractDir)
	if err != nil {
		t.Fatalf("extract archive: %v", err)
	}
	originURL, err := git.ExecGit(context.Background(), "remote", repoPath, []string{"get-url", "origin"})
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

func TestRunRejectsInvalidJobs(t *testing.T) {
	app, err := New(t.TempDir(), 0)
	if err != nil {
		t.Fatalf("new rakun: %v", err)
	}

	if _, err := app.Run(context.Background(), nil, nil); err == nil {
		t.Fatal("expected jobs validation error")
	}
}
