package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
)

var themeNames = []string{"Terminal", "Dark", "Light", "Paper", "High contrast"}

// A nil colour leaves the corresponding terminal default intact.
var themeBackground lipgloss.TerminalColor
var themeForeground lipgloss.TerminalColor

type themePalette struct {
	background lipgloss.TerminalColor
	foreground lipgloss.TerminalColor
	accent     lipgloss.TerminalColor
	green      lipgloss.TerminalColor
	yellow     lipgloss.TerminalColor
	red        lipgloss.TerminalColor
	dim        lipgloss.TerminalColor
}

func paletteFor(name string) (themePalette, error) {
	switch name {
	case "Terminal":
		return themePalette{accent: lipgloss.ANSIColor(1), green: lipgloss.ANSIColor(10),
			yellow: lipgloss.ANSIColor(11), red: lipgloss.ANSIColor(9), dim: lipgloss.ANSIColor(8)}, nil
	case "Dark":
		return themePalette{background: lipgloss.Color("#151922"), foreground: lipgloss.Color("#E8EDF5"),
			accent: lipgloss.Color("#8FC7FF"), green: lipgloss.Color("#83D7A5"),
			yellow: lipgloss.Color("#F9CC79"), red: lipgloss.Color("#FF8D9A"), dim: lipgloss.Color("#A1ACBD")}, nil
	case "Light":
		return themePalette{background: lipgloss.Color("#FAFAF5"), foreground: lipgloss.Color("#202B38"),
			accent: lipgloss.Color("#245B9A"), green: lipgloss.Color("#196B42"),
			yellow: lipgloss.Color("#805400"), red: lipgloss.Color("#B32C40"), dim: lipgloss.Color("#566473")}, nil
	case "Paper":
		return themePalette{background: lipgloss.Color("#FAFAF5"), foreground: lipgloss.Color("#000000"),
			accent: lipgloss.Color("#000000"), green: lipgloss.Color("#000000"),
			yellow: lipgloss.Color("#000000"), red: lipgloss.Color("#000000"), dim: lipgloss.Color("#000000")}, nil
	case "High contrast":
		return themePalette{background: lipgloss.Color("#000000"), foreground: lipgloss.Color("#FFFFFF"),
			accent: lipgloss.Color("#FFFF00"), green: lipgloss.Color("#00FF80"),
			yellow: lipgloss.Color("#FFFF00"), red: lipgloss.Color("#FF5F5F"), dim: lipgloss.Color("#CCCCCC")}, nil
	default:
		return themePalette{}, fmt.Errorf("unknown theme %q", name)
	}
}

// applyTheme rebuilds every shared style. Call it from the TUI event loop;
// styles are package globals and are not intended for concurrent mutation.
func applyTheme(name string) error {
	p, err := paletteFor(name)
	if err != nil {
		return err
	}
	themeBackground, themeForeground = p.background, p.foreground
	accent, green, yellow, red, grey = p.accent, p.green, p.yellow, p.red, p.dim

	base := lipgloss.NewStyle()
	if p.foreground != nil {
		base = base.Foreground(p.foreground)
	}
	titleStyle = base.Foreground(p.accent).Bold(true)
	textStyle = base
	dimStyle = base.Foreground(p.dim)
	faintStyle = base.Foreground(p.dim).Faint(true)
	accentStyle = base.Foreground(p.accent)
	playingStyle = base.Foreground(p.accent)
	errorStyle = base.Foreground(p.red)
	selectedStyle = base.Foreground(p.accent).Bold(true)
	tabStyle = base.Foreground(p.dim)
	activeTab = base.Foreground(p.accent).Reverse(true).Bold(true)
	onStyle = activeTab
	keyStyle = base.Foreground(p.accent).Bold(true)
	spectrumStyles = [3]lipgloss.Style{
		base.Foreground(p.green), base.Foreground(p.yellow), base.Foreground(p.red),
	}
	return nil
}

func themeSettingsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config directory: %w", err)
	}
	return filepath.Join(dir, "cider-tui", "settings.json"), nil
}

type appearanceSettings struct {
	Theme      string `json:"theme"`
	Background bool   `json:"background"`
	Visualizer string `json:"visualizer,omitempty"`
}

func loadTheme(path string) (string, error) {
	config, err := loadAppearance(path)
	return config.Theme, err
}

func loadAppearance(path string) (appearanceSettings, error) {
	config := appearanceSettings{Theme: "Terminal", Background: true, Visualizer: "spectrum"}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return config, nil
	}
	if err != nil {
		return config, err
	}
	defer f.Close()
	config.Theme = "" // a present settings file must name a valid theme
	decoder := json.NewDecoder(f)
	if err := decoder.Decode(&config); err != nil {
		return config, fmt.Errorf("decode theme settings: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("trailing JSON value")
		}
		return config, fmt.Errorf("decode theme settings: %w", err)
	}
	if _, err := paletteFor(config.Theme); err != nil {
		return config, err
	}
	if config.Visualizer != "spectrum" && config.Visualizer != "waveform" {
		return config, fmt.Errorf("unknown visualizer %q", config.Visualizer)
	}
	return config, nil
}

// saveTheme writes a complete replacement beside the destination and renames
// it into place, so an interrupted write cannot leave truncated settings.
func saveTheme(path, name string) error {
	return saveAppearance(path, appearanceSettings{Theme: name, Background: true})
}

func saveAppearance(path string, config appearanceSettings) error {
	if _, err := paletteFor(config.Theme); err != nil {
		return err
	}
	if config.Visualizer != "" && config.Visualizer != "spectrum" && config.Visualizer != "waveform" {
		return fmt.Errorf("unknown visualizer %q", config.Visualizer)
	}
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create settings directory: %w", err)
	}
	f, err := os.CreateTemp(dir, ".settings-*")
	if err != nil {
		return fmt.Errorf("create temporary settings: %w", err)
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return fmt.Errorf("replace settings: %w", err)
	}
	return nil
}
