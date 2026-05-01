package git

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"os/exec"
	"strings"
)

func ExecGit(ctx context.Context, command string, dir string, args []string) (string, error) {
	return ExecGitWithCredentials(ctx, command, dir, args, "", nil)
}

func ExecGitWithCredentials(ctx context.Context, command string, dir string, args []string, remote string, credentials *Credentials) (string, error) {
	args = append([]string{command}, args...)
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	cmd.Dir = dir
	cmd.Env = gitCommandEnv(remote, credentials)
	err := cmd.Run()
	if err != nil {
		return errBuf.String(), err
	}
	return strings.Trim(outBuf.String(), "\n "), nil
}

func gitCommandEnv(remote string, credentials *Credentials) []string {
	env := append([]string{}, os.Environ()...)
	env = append(env, "GIT_TERMINAL_PROMPT=0")

	remote = strings.ToLower(remote)
	if credentials == nil || (!strings.HasPrefix(remote, "https://") && !strings.HasPrefix(remote, "http://")) {
		return env
	}

	username := credentials.Username
	if username == "" {
		username = "git"
	}
	header := "Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+credentials.Password))

	return append(
		env,
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=http.extraHeader",
		"GIT_CONFIG_VALUE_0="+header,
	)
}
