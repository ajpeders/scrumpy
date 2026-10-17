package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// A nested Lip Gloss style resets *both* colours. Merely styling the frame
// does not keep its background active for the unstyled cells after that reset.
func TestThemeFrameRepaintsAfterNestedStyles(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() {
		_ = applyTheme("Terminal")
		lipgloss.SetColorProfile(profile)
	})

	m := New(&fakeClient{})
	for _, name := range themeNames {
		if err := applyTheme(name); err != nil {
			t.Fatal(err)
		}
		for _, on := range []bool{true, false} {
			m.backgroundOn = on
			frame := m.themedFrame(titleStyle.Render("title")+" plain "+dimStyle.Render("dim")+" tail", 32)
			wantBG := on && name != "Terminal"
			for _, marker := range []string{"plain", "tail"} {
				at := strings.Index(frame, marker)
				if at < 0 {
					t.Fatalf("%s background=%v: missing %s in %q", name, on, marker, frame)
				}
				// The nearest reset must be followed by the frame palette before
				// any bare characters, including the gaps between styled spans.
				before := frame[:at]
				reset := strings.LastIndex(before, "\x1b[0m")
				if wantBG && (reset < 0 || !strings.Contains(before[reset:], "48;2;")) {
					t.Fatalf("%s background=%v: background lost before %s: %q", name, on, marker, frame)
				}
				if !wantBG && strings.Contains(frame, "48;") {
					t.Fatalf("%s background=%v: painted a theme background: %q", name, on, frame)
				}
			}
		}
	}
}

func TestPaperViewRepaintsAfterThemeChange(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() {
		_ = applyTheme("Terminal")
		lipgloss.SetColorProfile(profile)
	})
	m := New(&fakeClient{})
	for _, previous := range []string{"Terminal", "Dark", "Light", "High contrast"} {
		if err := applyTheme(previous); err != nil {
			t.Fatal(err)
		}
		if err := applyTheme("Paper"); err != nil {
			t.Fatal(err)
		}
		frame := m.View()
		if !strings.Contains(frame, "48;2;243;238;226") ||
			!strings.Contains(frame, "\x1b[0m\x1b[38;2;0;0;0;48;2;243;238;226m") {
			t.Fatalf("%s to Paper did not repaint after styled content: %q", previous, frame)
		}
	}
}
