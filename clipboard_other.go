//go:build !darwin

package main

import tea "github.com/charmbracelet/bubbletea"

// readClipboard is not supported on this platform — returns nil (no-op).
func readClipboard() tea.Cmd { return nil }
