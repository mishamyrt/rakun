package git

import (
	"bytes"
	"os/exec"
	"strings"
)

func ExecGit(command string, dir string, args []string) (string, error) {
	args = append([]string{command}, args...)
	cmd := exec.Command("git", args...)
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	cmd.Dir = dir
	err := cmd.Run()
	if err != nil {
		return errBuf.String(), err
	}
	return strings.Trim(outBuf.String(), "\n "), nil
}
