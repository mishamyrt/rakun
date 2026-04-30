package git

import "testing"

func TestParseRemoteHTTP(t *testing.T) {
	spec, err := ParseRemote("https://github.com/example/project.git")
	if err != nil {
		t.Fatalf("parse remote: %v", err)
	}
	if spec.ArchiveRelativePath != "github.com/example/project.tar.gz" {
		t.Fatalf("unexpected archive path: %q", spec.ArchiveRelativePath)
	}
	if spec.RepositoryName != "project" {
		t.Fatalf("unexpected repository name: %q", spec.RepositoryName)
	}
}

func TestParseRemoteFileURL(t *testing.T) {
	spec, err := ParseRemote("file:///tmp/example/project.git")
	if err != nil {
		t.Fatalf("parse remote: %v", err)
	}
	if spec.ArchiveRelativePath != "file/tmp/example/project.tar.gz" {
		t.Fatalf("unexpected archive path: %q", spec.ArchiveRelativePath)
	}
}

func TestParseRemoteHead(t *testing.T) {
	head, err := ParseRemoteHead("ref: refs/heads/main HEAD\n0123456789abcdef0123456789abcdef01234567\tHEAD")
	if err != nil {
		t.Fatalf("parse remote head: %v", err)
	}
	if head.Branch != "main" {
		t.Fatalf("unexpected branch: %q", head.Branch)
	}
	if head.Commit != "0123456789abcdef0123456789abcdef01234567" {
		t.Fatalf("unexpected commit: %q", head.Commit)
	}
}
