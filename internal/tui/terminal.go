package tui

import "os"

func isInteractiveTerminal(file *os.File) bool {
	if file == nil {
		return false
	}

	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
