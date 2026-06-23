package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/thousandflowers/qr-multi-imgs/exporter"
	"github.com/thousandflowers/qr-multi-imgs/scanner"
)

// ─── TUI state ──────────────────────────────────────────────────────────────

type page int

const (
	pageFolderInput page = iota
	pageScanning
	pageResults
	pageList
	pageExportFormat
	pageQRFormat
	pageWorking
	pageDone
	pageError
)

type model struct {
	page            page
	summary         *scanner.Summary
	input           textinput.Model
	spinner         spinner.Model
	progress        progress.Model
	scanCh          <-chan scanner.Progress
	scanDone        int
	scanTotal       int
	scanFile        string
	exportFmt       string
	qrRecreateFmt   string
	errMsg          string
	actionMsg       string
	width           int
	height          int
	cursor          int
	initialScanPath string
}

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED")).MarginBottom(1)
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E"))
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#EAB308"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED"))
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#52525B")).MarginTop(1)
	mutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#A1A1AA"))
	boxStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).MarginTop(1).MarginBottom(1)
)

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{textinput.Blink, m.spinner.Tick}
	if m.initialScanPath != "" {
		cmds = append(cmds, safeCmd(startScan(m.initialScanPath)))
	}
	return tea.Batch(cmds...)
}

// ─── Messages ───────────────────────────────────────────────────────────────

type scanCompleteMsg struct{ summary *scanner.Summary }
type scanStartedMsg struct{ ch <-chan scanner.Progress }
type scanProgressMsg scanner.Progress
type actionDoneMsg struct{ message string }
type actionErrorMsg struct{ err error }
type crashMsg struct{ recover interface{} }
type clipboardMsg struct{ path string }

// safeCmd wraps a command closure with panic recovery.
func safeCmd(fn func() tea.Msg) tea.Cmd {
	return func() (msg tea.Msg) {
		defer func() {
			if r := recover(); r != nil {
				msg = crashMsg{recover: r}
			}
		}()
		msg = fn()
		return
	}
}

// validateScanPath checks that path exists, is a directory, and is readable.
func validateScanPath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("directory does not exist: %s", path)
		}
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("permission denied: %s", path)
		}
		return fmt.Errorf("cannot access %s: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot read directory %s: %w", path, err)
	}
	f.Close()
	return nil
}

// isWritablePath returns nil if we can create files in the directory.
func isWritablePath(path string) error {
	testFile := filepath.Join(path, ".qr_write_test")
	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("directory is not writable: %w", err)
	}
	f.Close()
	os.Remove(testFile)
	return nil
}

// resolvePath normalizes user input: expands ~, strips quotes, cleans separators.
func resolvePath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return os.Getwd()
	}
	raw = strings.Trim(raw, `"'`)
	if strings.HasPrefix(raw, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot expand ~: %w", err)
		}
		if raw == "~" {
			raw = home
		} else {
			raw = filepath.Join(home, raw[1:])
		}
	}
	raw = filepath.Clean(raw)
	if !filepath.IsAbs(raw) {
		abs, err := filepath.Abs(raw)
		if err != nil {
			return raw, nil
		}
		return abs, nil
	}
	return raw, nil
}

// ─── Update ─────────────────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if w := msg.Width - 4; w > 0 {
			m.progress.Width = w
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case scanStartedMsg:
		m.scanCh = msg.ch
		return m, waitForProgress(m.scanCh)

	case scanProgressMsg:
		m.scanDone = msg.Done
		m.scanTotal = msg.Total
		m.scanFile = msg.File
		return m, waitForProgress(m.scanCh)

	case scanCompleteMsg:
		m.summary = msg.summary
		m.cursor = 0
		// No supported images → skip the results page (which assumes Results[0]).
		if msg.summary == nil || len(msg.summary.Results) == 0 {
			m.page = pageDone
			m.actionMsg = "No supported images found in that folder."
			return m, nil
		}
		m.page = pageResults
		return m, nil

	case actionDoneMsg:
		m.page = pageDone
		m.actionMsg = msg.message
		return m, nil

	case actionErrorMsg:
		m.page = pageError
		m.errMsg = msg.err.Error()
		return m, nil

	case crashMsg:
		m.page = pageError
		m.errMsg = fmt.Sprintf("INTERNAL CRASH\n\n%v\n\nStack trace:\n%s",
			msg.recover, debug.Stack())
		return m, nil

	case clipboardMsg:
		if m.page == pageFolderInput {
			m.input.SetValue(msg.path)
			return m.updateFolderInput(tea.KeyMsg{Type: tea.KeyEnter})
		}
		return m, nil
	}

	if m.page == pageScanning {
		return m.updateScanning(msg)
	}

	return m, nil
}

func (m model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.page {
	case pageFolderInput:
		return m.updateFolderInput(msg)
	case pageResults:
		return m.updateResults(msg)
	case pageList:
		return m.updateList(msg)
	case pageExportFormat:
		return m.updateExportFormat(msg)
	case pageQRFormat:
		return m.updateQRFormat(msg)
	case pageDone, pageError:
		return m.updateDone(msg)
	case pageWorking:
		return m, nil
	}
	return m, nil
}

func (m model) updateScanning(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// ─── page: folder input ─────────────────────────────────────────────────────

func (m model) updateFolderInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		rawPath := m.input.Value()
		path, err := resolvePath(rawPath)
		if err != nil {
			m.page = pageError
			m.errMsg = err.Error() + "\n\nPress Enter to go back and try a different path."
			return m, nil
		}
		m.input.SetValue(path)
		if err := validateScanPath(path); err != nil {
			m.page = pageError
			m.errMsg = err.Error() + "\n\nPress Enter to go back and try a different path."
			return m, nil
		}
		m.page = pageScanning
		return m, safeCmd(startScan(path))

	case tea.KeyCtrlC, tea.KeyEscape:
		return m, tea.Quit

	case tea.KeyCtrlD:
		return m, readClipboard()

	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

// ─── page: results menu ─────────────────────────────────────────────────────

func (m model) updateResults(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp, tea.KeyShiftTab:
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case tea.KeyDown, tea.KeyTab:
		if m.cursor < 6 {
			m.cursor++
		}
		return m, nil

	case tea.KeyEnter:
		return m.executeAction(m.cursor)

	case tea.KeyCtrlC:
		return m, tea.Quit
	}
	return m, nil
}

func (m model) executeAction(idx int) (tea.Model, tea.Cmd) {
	switch idx {
	case 0:
		m.page = pageList
		m.cursor = 0
		return m, nil

	case 1:
		m.page = pageExportFormat
		m.cursor = 0
		return m, nil

	case 2:
		if m.summary.WithoutQR == 0 {
			m.page = pageDone
			m.actionMsg = "No images without QR code to delete."
			return m, nil
		}
		base := filepath.Dir(m.summary.Results[0].FilePath)
		if err := isWritablePath(base); err != nil {
			m.page = pageError
			m.errMsg = err.Error()
			return m, nil
		}
		m.page = pageWorking
		return m, safeCmd(deleteWithoutQR(m.summary))

	case 3:
		if len(m.summary.Results) == 0 {
			return m, nil
		}
		base := filepath.Dir(m.summary.Results[0].FilePath)
		if err := isWritablePath(base); err != nil {
			m.page = pageError
			m.errMsg = err.Error()
			return m, nil
		}
		m.page = pageWorking
		return m, safeCmd(organizeByQR(m.summary))

	case 4:
		if m.summary.WithQR == 0 {
			m.page = pageDone
			m.actionMsg = "No images with QR code to recreate."
			return m, nil
		}
		m.page = pageQRFormat
		m.cursor = 0
		return m, nil

	case 5:
		m.page = pageFolderInput
		m.input.SetValue("")
		m.input.Focus()
		return m, nil

	case 6:
		return m, tea.Quit
	}
	return m, nil
}

// ─── page: list results ─────────────────────────────────────────────────────

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case tea.KeyDown:
		if m.cursor < len(m.summary.Results)-1 {
			m.cursor++
		}
		return m, nil
	case tea.KeyEscape, tea.KeyBackspace:
		m.page = pageResults
		return m, nil
	case tea.KeyCtrlC:
		return m, tea.Quit
	}
	return m, nil
}

// ─── page: QR recreate format ──────────────────────────────────────────────

func (m model) updateQRFormat(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case tea.KeyDown:
		if m.cursor < 2 {
			m.cursor++
		}
		return m, nil
	case tea.KeyEnter:
		formats := []string{"png", "jpg", "svg"}
		m.qrRecreateFmt = formats[m.cursor]
		base := filepath.Dir(m.summary.Results[0].FilePath)
		if err := isWritablePath(base); err != nil {
			m.page = pageError
			m.errMsg = err.Error()
			return m, nil
		}
		m.page = pageWorking
		return m, safeCmd(recreateQRs(m.summary, m.qrRecreateFmt))
	case tea.KeyEscape, tea.KeyBackspace:
		m.page = pageResults
		return m, nil
	case tea.KeyCtrlC:
		return m, tea.Quit
	}
	return m, nil
}

// ─── page: export format ────────────────────────────────────────────────────

func (m model) updateExportFormat(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case tea.KeyDown:
		if m.cursor < 2 {
			m.cursor++
		}
		return m, nil
	case tea.KeyEnter:
		formats := []string{"json", "csv", "txt"}
		m.exportFmt = formats[m.cursor]
		if m.summary == nil || len(m.summary.Results) == 0 {
			m.page = pageError
			m.errMsg = "No results to export.\n\nPress Enter to go back."
			return m, nil
		}
		base := filepath.Dir(m.summary.Results[0].FilePath)
		if err := isWritablePath(base); err != nil {
			m.page = pageError
			m.errMsg = err.Error()
			return m, nil
		}
		m.page = pageWorking
		return m, safeCmd(exportResults(m.summary, m.exportFmt))
	case tea.KeyEscape, tea.KeyBackspace:
		m.page = pageResults
		return m, nil
	case tea.KeyCtrlC:
		return m, tea.Quit
	}
	return m, nil
}

// ─── page: done / error ─────────────────────────────────────────────────────

func (m model) updateDone(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter, tea.KeyEscape:
		m.page = pageResults
		return m, nil
	case tea.KeyCtrlC:
		return m, tea.Quit
	}
	return m, nil
}

// ─── View ───────────────────────────────────────────────────────────────────

func (m model) View() string {
	switch m.page {
	case pageFolderInput:
		return m.viewFolderInput()
	case pageScanning:
		return m.viewScanning()
	case pageResults:
		return m.viewResults()
	case pageList:
		return m.viewList()
	case pageExportFormat:
		return m.viewExportFormat()
	case pageQRFormat:
		return m.viewQRFormat()
	case pageWorking:
		return m.viewWorking()
	case pageDone:
		return m.viewDone()
	case pageError:
		return m.viewError()
	}
	return ""
}

func (m model) viewFolderInput() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("QR Multi IMG Scanner"))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("Drag folder here • Paste (Cmd+V) • Type path • Ctrl+D clipboard"))
	b.WriteString("\n\n")
	b.WriteString(m.input.View())
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("Enter → scan  •  Ctrl+D → paste from clipboard  •  Esc → quit"))
	return b.String()
}

func (m model) viewScanning() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("QR Multi IMG Scanner"))
	b.WriteString("\n\n")

	// Progress bar stays pinned at the top.
	frac := 0.0
	if m.scanTotal > 0 {
		frac = float64(m.scanDone) / float64(m.scanTotal)
	}
	b.WriteString(m.progress.ViewAs(frac))
	b.WriteString(fmt.Sprintf("  %d/%d\n\n", m.scanDone, m.scanTotal))

	if m.scanFile != "" {
		b.WriteString(fmt.Sprintf("%s Scanning: %s\n", m.spinner.View(), truncate(filepath.Base(m.scanFile), 60)))
	} else {
		b.WriteString(fmt.Sprintf("%s Scanning folder for QR codes...\n", m.spinner.View()))
	}
	return b.String()
}

func (m model) viewResults() string {
	if m.summary == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("QR Multi IMG Scanner"))
	b.WriteString("\n")

	s := m.summary
	folder := filepath.Dir(s.Results[0].FilePath)
	if folder == "." {
		folder, _ = os.Getwd()
	}

	summaryBody := fmt.Sprintf(
		"\U0001f4c1  %s\n\nTotal: %d  │  %s %d with QR  │  %s %d empty  │  %s %d errors  │  \u23f1 %s",
		filepath.Base(folder),
		s.Total,
		okStyle.Render("●"), s.WithQR,
		warnStyle.Render("●"), s.WithoutQR,
		errStyle.Render("●"), s.Errors,
		roundDur(s.Duration),
	)
	b.WriteString(boxStyle.Render(summaryBody))
	b.WriteString("\n")

	b.WriteString(mutedStyle.Render("Select action:"))
	b.WriteString("\n\n")

	actions := []string{
		"List all results",
		"Export (JSON / CSV / TXT)",
		"Delete images without QR code",
		"Organize into folders (with_qr / without_qr)",
		"Recreate QR code images",
		"New scan",
		"Quit",
	}
	for i, act := range actions {
		prefix := "  "
		if i == m.cursor {
			prefix = cursorStyle.Render("▸ ")
		}
		b.WriteString(fmt.Sprintf("%s%s\n", prefix, act))
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑ ↓ navigate  •  Enter select  •  Ctrl+C quit"))
	return b.String()
}

func (m model) viewList() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Results — All Scanned Images"))
	b.WriteString("\n\n")

	results := m.summary.Results
	start := 0
	if m.cursor > 12 {
		start = m.cursor - 12
	}
	end := start + 25
	if end > len(results) {
		end = len(results)
	}

	for i, r := range results[start:end] {
		idx := start + i
		prefix := "  "
		if idx == m.cursor {
			prefix = cursorStyle.Render("▸ ")
		}

		icon := okStyle.Render("QR")
		content := ""
		if !r.HasQR {
			icon = warnStyle.Render("--")
		} else if len(r.Contents) > 0 {
			content = "  " + truncate(r.Contents[0], 55)
		}
		if r.Error != "" {
			icon = errStyle.Render("ERR")
			content = "  " + truncate(r.Error, 55)
		}

		b.WriteString(fmt.Sprintf("%s[%s] %s%s\n",
			prefix, icon, filepath.Base(r.FilePath), content))
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render(
		fmt.Sprintf("↑ ↓ scroll (%d items)  •  Esc back  •  Ctrl+C quit",
			len(results))))
	return b.String()
}

func (m model) viewQRFormat() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Recreate QR Code Images"))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("Choose output image format:"))
	b.WriteString("\n\n")

	formats := []string{"PNG (.png)", "JPG (.jpg)", "SVG (.svg)"}
	for i, f := range formats {
		prefix := "  "
		if i == m.cursor {
			prefix = cursorStyle.Render("▸ ")
		}
		b.WriteString(fmt.Sprintf("%s%s\n", prefix, f))
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑ ↓ navigate  •  Enter select  •  Esc back"))
	return b.String()
}

func (m model) viewExportFormat() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Export Results"))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("Choose export format:"))
	b.WriteString("\n\n")

	formats := []string{"JSON (.json)", "CSV (.csv)", "TXT (.txt)"}
	for i, f := range formats {
		prefix := "  "
		if i == m.cursor {
			prefix = cursorStyle.Render("▸ ")
		}
		b.WriteString(fmt.Sprintf("%s%s\n", prefix, f))
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑ ↓ navigate  •  Enter select  •  Esc back"))
	return b.String()
}

func (m model) viewWorking() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("QR Multi IMG Scanner"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("%s Working...\n", m.spinner.View()))
	return b.String()
}

func (m model) viewDone() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("\u2714 Done"))
	b.WriteString("\n\n")
	b.WriteString(boxStyle.Render(m.actionMsg))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("Enter → back  •  Ctrl+C → quit"))
	return b.String()
}

func (m model) viewError() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("\u2716 Error"))
	b.WriteString("\n\n")
	b.WriteString(errStyle.Render(m.errMsg))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("Enter → back  •  Ctrl+C → quit"))
	return b.String()
}

// ─── QR Scanning (TUI bridge) ───────────────────────────────────────────────

func startScan(path string) tea.Cmd {
	return func() tea.Msg {
		ch, err := scanner.ScanFolderStream(path)
		if err != nil {
			return actionErrorMsg{err}
		}
		return scanStartedMsg{ch}
	}
}

// waitForProgress blocks on the next event from the scan channel and turns it
// into a message. One event per call; Update re-issues it until the summary.
func waitForProgress(ch <-chan scanner.Progress) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return nil
		}
		if p.Summary != nil {
			return scanCompleteMsg{p.Summary}
		}
		return scanProgressMsg(p)
	}
}

// ─── Actions ────────────────────────────────────────────────────────────────

func deleteWithoutQR(s *scanner.Summary) tea.Cmd {
	return func() tea.Msg {
		var deleted int
		var errs []string
		for _, r := range s.Results {
			if r.HasQR || r.Error != "" {
				continue
			}
			if err := os.Remove(r.FilePath); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", filepath.Base(r.FilePath), err))
			} else {
				deleted++
			}
		}
		msg := fmt.Sprintf("Deleted %d image(s) without QR code.", deleted)
		if len(errs) > 0 {
			msg += "\n\nErrors:\n" + strings.Join(errs, "\n")
		}
		return actionDoneMsg{msg}
	}
}

func organizeByQR(s *scanner.Summary) tea.Cmd {
	return func() tea.Msg {
		base := filepath.Dir(s.Results[0].FilePath)
		withDir := filepath.Join(base, "with_qr")
		withoutDir := filepath.Join(base, "without_qr")
		os.MkdirAll(withDir, 0755)
		os.MkdirAll(withoutDir, 0755)

		var movedWith, movedWithout int
		var errs []string

		for _, r := range s.Results {
			destDir := withoutDir
			if r.HasQR {
				destDir = withDir
			}

			destPath := filepath.Join(destDir, filepath.Base(r.FilePath))
			if _, err := os.Stat(destPath); err == nil {
				baseName := strings.TrimSuffix(
					filepath.Base(r.FilePath), filepath.Ext(r.FilePath))
				ext := filepath.Ext(r.FilePath)
				destPath = filepath.Join(destDir,
					fmt.Sprintf("%s_%x%s", baseName, time.Now().UnixNano(), ext))
			}

			if err := os.Rename(r.FilePath, destPath); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", filepath.Base(r.FilePath), err))
			} else if r.HasQR {
				movedWith++
			} else {
				movedWithout++
			}
		}

		msg := fmt.Sprintf("Organized %d files:\n  with_qr: %d\n  without_qr: %d",
			movedWith+movedWithout, movedWith, movedWithout)
		if len(errs) > 0 {
			msg += "\n\nErrors:\n" + strings.Join(errs, "\n")
		}
		return actionDoneMsg{msg}
	}
}

func recreateQRs(s *scanner.Summary, format string) tea.Cmd {
	return func() tea.Msg {
		base := filepath.Dir(s.Results[0].FilePath)
		outDir := filepath.Join(base, "recreated_qr")
		os.MkdirAll(outDir, 0755)

		var created int
		var errs []string

		for _, r := range s.Results {
			if !r.HasQR || len(r.Contents) == 0 {
				continue
			}

			for i, content := range r.Contents {
				safeName := sanitizeFilename(content, 40)
				if safeName == "" {
					safeName = fmt.Sprintf("qr_%x", time.Now().UnixNano())
				}
				if i > 0 {
					safeName = fmt.Sprintf("%s_%d", safeName, i+1)
				}
				outPath := filepath.Join(outDir, safeName+"."+format)
				if err := exporter.WriteQRCodeFormat(content, outPath, exporter.QRCodeFormat(format)); err != nil {
					errs = append(errs, fmt.Sprintf("%s: %v", safeName, err))
				} else {
					created++
				}
			}
		}

		msg := fmt.Sprintf("Created %d QR image(s) in %s/ (format: %s)", created, filepath.Base(outDir), format)
		if len(errs) > 0 {
			msg += "\n\nErrors:\n" + strings.Join(errs, "\n")
		}
		return actionDoneMsg{msg}
	}
}

func exportResults(s *scanner.Summary, format string) tea.Cmd {
	return func() tea.Msg {
		base := filepath.Dir(s.Results[0].FilePath)
		path, err := exporter.Export(s.Results, exporter.ExportFormat(format), base)
		if err != nil {
			return actionErrorMsg{err}
		}
		return actionDoneMsg{fmt.Sprintf("Exported to: %s", path)}
	}
}

// ─── Small helpers ──────────────────────────────────────────────────────────

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

func roundDur(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func sanitizeFilename(s string, maxLen int) string {
	replacer := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
		"\n", "", "\r", "",
	)
	s = replacer.Replace(s)
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return strings.TrimSpace(s)
}

// ─── Main ───────────────────────────────────────────────────────────────────

var version = "1.3.0"

// ponytail: overridable in tests; avoids os.Exit killing the test process.
var osExit = os.Exit

func main() {
	showHelp, showVersion := parseArgs(os.Args[1:])
	if showHelp {
		printHelp()
		osExit(0)
	}
	if showVersion {
		printVersion()
		osExit(0)
	}
	m := initModel(os.Args)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		osExit(1)
	}
}

// parseArgs extracts flags from CLI args without side effects.
func parseArgs(args []string) (showHelp, showVersion bool) {
	for _, a := range args {
		switch a {
		case "-h", "--help":
			showHelp = true
		case "-v", "--version":
			showVersion = true
		}
	}
	return
}

func printHelp() {
	fmt.Println(`qr-multi-imgs — scan a folder of images for QR codes

Usage:
  qr-multi-imgs                   interactive TUI (type or drag folder path)
  qr-multi-imgs /path/to/images   scan directly, skip folder prompt
  qr-multi-imgs --help            this help text
  qr-multi-imgs --version         show version

Actions inside the TUI:
  l   List all scan results
  e   Export as JSON / CSV / TXT
  d   Delete images without QR code
  o   Organize into with_qr / without_qr folders
  r   Recreate QR code images from decoded content
  q   Quit

Input methods:
  Type a path, drag & drop a folder, or press Ctrl+D to paste clipboard.
  Press Enter on an empty input to scan the current directory.

Supported formats: PNG, JPG, JPEG, GIF, BMP, WebP
Project: https://github.com/thousandflowers/qr-multi-imgs`)
}

func printVersion() {
	fmt.Printf("qr-multi-imgs %s\n", version)
}

// initModel creates and configures the initial TUI model from CLI args.
func initModel(args []string) model {
	m := model{
		page:     pageFolderInput,
		input:    textinput.New(),
		spinner:  spinner.New(spinner.WithSpinner(spinner.Dot)),
		progress: progress.New(progress.WithDefaultGradient()),
	}

	m.input.Placeholder = "/path/to/images (Enter for current dir)"
	m.input.CharLimit = 512
	m.input.Width = 80

	if len(args) > 1 && args[1] != "" && args[1][0] != '-' {
		path, err := resolvePath(args[1])
		if err == nil {
			if err := validateScanPath(path); err == nil {
				m.page = pageScanning
				m.initialScanPath = path
				m.input.SetValue(path)
			} else {
				m.input.SetValue(path)
			}
		}
	}

	if m.page != pageScanning {
		m.input.Focus()
	}
	return m
}
