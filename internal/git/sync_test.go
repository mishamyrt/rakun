package git

import (
	"context"
	"os"
	"path/filepath"
	"rakun/internal/taskrun"
	"strings"
	"testing"
	"time"
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

func TestTaskRunnerRejectsInvalidJobs(t *testing.T) {
	if _, err := taskrun.Execute(context.Background(), nil, 0, nil); err == nil {
		t.Fatal("expected jobs validation error")
	}
}
