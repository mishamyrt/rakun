package git

import (
	"bytes"
	"log"
	"os/exec"
)

func Clone(url string, dir string) error {
	return Exec(dir, "clone", url)
}

func Pull(dir string) error {
	return Exec(dir, "pull")
}

func Exec(dir string, args ...string) error {
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
