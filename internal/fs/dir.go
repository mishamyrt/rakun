package fs

import "os"

func CreateDirectory(path string) error {
	err := os.MkdirAll(path, 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}
	return nil
}
