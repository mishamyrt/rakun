package git

import (
	"context"
	"fmt"
	"strings"
)

// RemoteHead contains the branch name and commit advertised as remote HEAD.
type RemoteHead struct {
	Branch string
	Commit string
}

// ResolveRemoteHead resolves the default branch and commit advertised by remote HEAD.
func ResolveRemoteHead(ctx context.Context, remote string, credentials *Credentials) (RemoteHead, error) {
	output, err := ExecGitWithCredentials(ctx, "ls-remote", "", []string{"--symref", remote, "HEAD"}, remote, credentials)
	if err != nil {
		return RemoteHead{}, err
	}
	return ParseRemoteHead(output)
}

// ParseRemoteHead parses the output of git ls-remote --symref for HEAD.
func ParseRemoteHead(output string) (RemoteHead, error) {
	var remoteHead RemoteHead
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[0] == "ref:" && len(fields) >= 3 && fields[2] == "HEAD" {
			remoteHead.Branch = strings.TrimPrefix(fields[1], "refs/heads/")
			continue
		}
		if fields[1] == "HEAD" {
			remoteHead.Commit = fields[0]
		}
	}

	if remoteHead.Branch == "" || remoteHead.Commit == "" {
		return RemoteHead{}, fmt.Errorf("cannot parse remote HEAD from %q", output)
	}
	return remoteHead, nil
}
