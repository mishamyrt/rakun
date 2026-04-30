package git

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

type RemoteSpec struct {
	Remote              string
	ArchiveRelativePath string
	DisplayName         string
	RepositoryName      string
}

type Credentials struct {
	Username string
	Password string
}

type RemoteTarget struct {
	URL         string
	Credentials *Credentials
}

func NewTokenCredentials(token string) *Credentials {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	return &Credentials{
		Username: "git",
		Password: token,
	}
}

func ParseRemote(remote string) (RemoteSpec, error) {
	host, remotePath, err := splitRemote(remote)
	if err != nil {
		return RemoteSpec{}, err
	}

	cleanPath := strings.Trim(strings.TrimPrefix(path.Clean(remotePath), "/"), "/")
	cleanPath = strings.TrimSuffix(cleanPath, ".git")
	if cleanPath == "" || cleanPath == "." || cleanPath == ".." || strings.HasPrefix(cleanPath, "../") {
		return RemoteSpec{}, fmt.Errorf("invalid repository path %q", remotePath)
	}

	repositoryName := path.Base(cleanPath)
	return RemoteSpec{
		Remote:              remote,
		ArchiveRelativePath: filepath.Join(host, filepath.FromSlash(cleanPath)) + ".tar.gz",
		DisplayName:         filepath.ToSlash(filepath.Join(host, filepath.FromSlash(cleanPath))),
		RepositoryName:      repositoryName,
	}, nil
}

func splitRemote(remote string) (string, string, error) {
	if strings.Contains(remote, "://") {
		parsedURL, err := url.Parse(remote)
		if err != nil {
			return "", "", err
		}
		host := parsedURL.Host
		if parsedURL.Scheme == "file" && host == "" {
			host = "file"
		}
		if host == "" {
			return "", "", fmt.Errorf("remote URL host is empty")
		}
		return sanitizeHost(host), parsedURL.Path, nil
	}

	atIndex := strings.Index(remote, "@")
	colonIndex := strings.LastIndex(remote, ":")
	if atIndex >= 0 && colonIndex > atIndex {
		host := sanitizeHost(remote[atIndex+1 : colonIndex])
		remotePath := remote[colonIndex+1:]
		if host == "" || remotePath == "" {
			return "", "", fmt.Errorf("invalid remote %q", remote)
		}
		return host, remotePath, nil
	}

	return "", "", fmt.Errorf("unsupported remote %q", remote)
}

func sanitizeHost(host string) string {
	return strings.ReplaceAll(strings.ToLower(host), ":", "_")
}
