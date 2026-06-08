//go:build darwin

package main

import (
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// readClipboard returns a command that reads the macOS pasteboard via pbpaste.
func readClipboard() tea.Cmd {
	return func() tea.Msg {
		out, err := exec.Command("pbpaste").Output()
		if err != nil {
			return nil
		}
		path := strings.TrimSpace(string(out))
		if path == "" {
			return nil
		}
		return clipboardMsg{path: path}
	}
}
