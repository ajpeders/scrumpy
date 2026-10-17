package ui

import (
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// clipboardTool is a host clipboard command and its arguments.
type clipboardTool struct {
	name string
	args []string
}

// clipboardTools lists candidates in preference order.
var clipboardTools = []clipboardTool{
	{"pbcopy", nil},
	{"clip.exe", nil},
	{"wl-copy", nil},
	{"xclip", []string{"-selection", "clipboard"}},
	{"xsel", []string{"--clipboard", "--input"}},
}

// Stubbable in tests.
var (
	lookPath       = exec.LookPath
	clipboardWrite = func(t clipboardTool, text string) error {
		cmd := exec.Command(t.name, t.args...)
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	}
)

// findClipboardTool returns the first available clipboard tool.
func findClipboardTool() (clipboardTool, bool) {
	for _, t := range clipboardTools {
		if _, err := lookPath(t.name); err == nil {
			return t, true
		}
	}
	return clipboardTool{}, false
}

// clipboardText formats an item as "Title — Artist · Album", omitting empty parts.
func clipboardText(it Item) string {
	s := it.Title
	if it.Subtitle != "" {
		s += " — " + it.Subtitle
	}
	if it.Album != "" {
		s += " · " + it.Album
	}
	return s
}

// copiedMsg carries copyItem's status back to Update.
type copiedMsg struct{ status string }

// copyCmd runs the copy off the event loop, since a clipboard tool can stall
// (e.g. xclip with no reachable X server).
func copyCmd(it Item) tea.Cmd {
	return func() tea.Msg { return copiedMsg{copyItem(it)} }
}

// copyItem copies the item's text and returns a status message.
func copyItem(it Item) string {
	t, ok := findClipboardTool()
	if !ok {
		return "no clipboard tool found"
	}
	if err := clipboardWrite(t, clipboardText(it)); err != nil {
		return "Copy failed: " + err.Error()
	}
	return "Copied: " + clipboardText(it)
}
