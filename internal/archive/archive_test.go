package archive

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveRoundTrip(t *testing.T) {
	sourceRoot := t.TempDir()
	sourceDir := filepath.Join(sourceRoot, "project")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "README.md"), []byte("hello\n"), 0644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	archivePath := filepath.Join(t.TempDir(), "project.tar.gz")
	if err := CreateArchive(archivePath, sourceDir); err != nil {
		t.Fatalf("create archive: %v", err)
	}

	extractedDir, err := ExtractArchive(archivePath, t.TempDir())
	if err != nil {
		t.Fatalf("extract archive: %v", err)
	}
	if filepath.Base(extractedDir) != "project" {
		t.Fatalf("unexpected extracted root: %q", extractedDir)
	}

	readmeBytes, err := os.ReadFile(filepath.Join(extractedDir, "README.md"))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(readmeBytes) != "hello\n" {
		t.Fatalf("unexpected extracted contents: %q", string(readmeBytes))
	}
}

func TestExtractArchiveRejectsInvalidArchives(t *testing.T) {
	t.Run("broken gzip", func(t *testing.T) {
		archivePath := filepath.Join(t.TempDir(), "broken.tar.gz")
		if err := os.WriteFile(archivePath, []byte("broken"), 0644); err != nil {
			t.Fatalf("write broken archive: %v", err)
		}

		_, err := ExtractArchive(archivePath, t.TempDir())
		if !errors.Is(err, ErrInvalid) {
			t.Fatalf("expected ErrInvalid, got %v", err)
		}
	})

	t.Run("multiple roots", func(t *testing.T) {
		archivePath := filepath.Join(t.TempDir(), "multiple-roots.tar.gz")
		writeArchive(t, archivePath, []archiveEntry{
			{name: "one", mode: 0755, typeflag: tar.TypeDir},
			{name: "two", mode: 0755, typeflag: tar.TypeDir},
		})

		_, err := ExtractArchive(archivePath, t.TempDir())
		if !errors.Is(err, ErrInvalid) {
			t.Fatalf("expected ErrInvalid, got %v", err)
		}
	})

	t.Run("invalid path", func(t *testing.T) {
		archivePath := filepath.Join(t.TempDir(), "invalid-path.tar.gz")
		writeArchive(t, archivePath, []archiveEntry{
			{name: "../evil", mode: 0644, typeflag: tar.TypeReg, body: "oops"},
		})

		_, err := ExtractArchive(archivePath, t.TempDir())
		if !errors.Is(err, ErrInvalid) {
			t.Fatalf("expected ErrInvalid, got %v", err)
		}
	})
}

type archiveEntry struct {
	body     string
	linkname string
	mode     int64
	name     string
	typeflag byte
}

func writeArchive(t *testing.T, archivePath string, entries []archiveEntry) {
	t.Helper()

	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive file: %v", err)
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	for _, entry := range entries {
		header := &tar.Header{
			Name:     entry.name,
			Linkname: entry.linkname,
			Mode:     entry.mode,
			Size:     int64(len(entry.body)),
			Typeflag: entry.typeflag,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("write header: %v", err)
		}
		if entry.typeflag == tar.TypeReg {
			if _, err := tarWriter.Write([]byte(entry.body)); err != nil {
				t.Fatalf("write body: %v", err)
			}
		}
	}
}
