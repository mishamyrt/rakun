package git

import (
	"bytes"
	"log"
	"os/exec"
)

func ExecGit(command string, dir string, args ...string) error {
	args = append([]string{command}, args...)
	cmd := exec.Command("git", args...)
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	cmd.Dir = dir
	err := cmd.Run()
	if err != nil {
		log.Fatal(err, errBuf.String())
	}
	return err
}
