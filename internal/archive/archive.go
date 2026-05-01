package archive

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ErrInvalid indicates that an archive is malformed or contains unsupported content.
var ErrInvalid = errors.New("invalid archive")

func wrapInvalid(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %v", ErrInvalid, err)
}

func invalidf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// CreateArchive writes sourceDir to archivePath as a gzip-compressed tar archive.
func CreateArchive(archivePath string, sourceDir string) error {
	if err := os.MkdirAll(filepath.Dir(archivePath), 0755); err != nil {
		return err
	}

	tmpPath := archivePath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)

	walkErr := filepath.Walk(sourceDir, func(currentPath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		linkTarget := ""
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, walkErr = os.Readlink(currentPath)
			if walkErr != nil {
				return walkErr
			}
		}

		header, err := tar.FileInfoHeader(info, linkTarget)
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(filepath.Dir(sourceDir), currentPath)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relativePath)
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		sourceFile, err := os.Open(currentPath)
		if err != nil {
			return err
		}

		_, err = io.Copy(tarWriter, sourceFile)
		closeErr := sourceFile.Close()
		if err == nil {
			err = closeErr
		}
		return err
	})

	closeErr := tarWriter.Close()
	if err == nil {
		err = closeErr
	}
	closeErr = gzipWriter.Close()
	if err == nil {
		err = closeErr
	}
	closeErr = file.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = walkErr
	}
	if err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	return os.Rename(tmpPath, archivePath)
}

// ExtractArchive extracts archivePath into destinationDir and returns the extracted root path.
func ExtractArchive(archivePath string, destinationDir string) (extractedPath string, err error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return "", wrapInvalid(err)
	}
	defer func() {
		if closeErr := gzipReader.Close(); err == nil {
			err = closeErr
		}
	}()

	tarReader := tar.NewReader(gzipReader)
	rootName := ""
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", wrapInvalid(err)
		}

		cleanName := path.Clean(header.Name)
		if cleanName == "." || cleanName == ".." || strings.HasPrefix(cleanName, "../") {
			return "", invalidf("archive contains invalid path %q", header.Name)
		}

		segments := strings.Split(cleanName, "/")
		if rootName == "" {
			rootName = segments[0]
		} else if rootName != segments[0] {
			return "", invalidf("archive contains multiple roots")
		}

		targetPath := filepath.Join(destinationDir, filepath.FromSlash(cleanName))
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return "", err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return "", err
			}
			targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(targetFile, tarReader); err != nil {
				closeErr := targetFile.Close()
				if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
					return "", errors.Join(wrapInvalid(err), closeErr)
				}
				return "", errors.Join(err, closeErr)
			}
			if err := targetFile.Close(); err != nil {
				return "", err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return "", err
			}
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return "", err
			}
		default:
			return "", invalidf("unsupported archive entry %q", header.Name)
		}
	}

	if rootName == "" {
		return "", invalidf("archive is empty")
	}
	return filepath.Join(destinationDir, filepath.FromSlash(rootName)), nil
}
