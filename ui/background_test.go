package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestBackgroundTogglePersistsAcrossThemesAndRestart(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Cleanup(func() { _ = applyTheme("Terminal") })
	m := New(&fakeClient{})
	m.theme = "Paper"
	m.settings = true
	m.themeCursor = len(themeNames)
	m, _ = sendKey(t, m, specialKey(tea.KeyEnter))
	if m.backgroundOn || !strings.Contains(m.View(), "Background: Off") {
		t.Fatal("background toggle did not turn off")
	}
	m.themeCursor = 1 // Dark
	m, _ = sendKey(t, m, specialKey(tea.KeyEnter))
	if m.backgroundOn || m.theme != "Dark" {
		t.Fatal("changing theme reset the background option")
	}
	reopened := New(&fakeClient{}).WithSettings()
	if reopened.backgroundOn || reopened.theme != "Dark" {
		t.Fatal("saved background option was not restored")
	}
}

func TestOldSettingsDefaultBackgroundOn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"theme":"Paper"}`), 0600); err != nil {
		t.Fatal(err)
	}
	config, err := loadAppearance(path)
	if err != nil || !config.Background || config.Theme != "Paper" {
		t.Fatalf("legacy theme settings changed behavior: %+v %v", config, err)
	}
}

func TestBackgroundOffDoesNotPaintThemeBackground(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() {
		_ = applyTheme("Terminal")
		lipgloss.SetColorProfile(profile)
	})
	if err := applyTheme("Paper"); err != nil {
		t.Fatal(err)
	}
	m := New(&fakeClient{})
	if frame := m.themedFrame("text", 10); !strings.Contains(frame, "48;2;250;250;245") {
		t.Fatalf("enabled background was not painted: %q", frame)
	}
	m.backgroundOn = false
	frame := m.themedFrame("text", 10)
	if strings.Contains(frame, "48;") || !strings.Contains(frame, "38;2;0;0;0") {
		t.Fatalf("background off did not preserve only black foreground: %q", frame)
	}
}
