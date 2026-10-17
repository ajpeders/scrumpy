package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestApplyTheme(t *testing.T) {
	t.Cleanup(func() {
		if err := applyTheme("Terminal"); err != nil {
			t.Error(err)
		}
	})
	if !reflect.DeepEqual(themeNames, []string{"Terminal", "Dark", "Light", "Paper", "High contrast"}) {
		t.Fatalf("theme names = %v", themeNames)
	}
	for _, name := range themeNames {
		if err := applyTheme(name); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
		if name == "Terminal" {
			if themeBackground != nil || themeForeground != nil || textStyle.GetForeground() != (lipgloss.NoColor{}) {
				t.Fatal("Terminal must retain the terminal's default background and foreground")
			}
			if accent != lipgloss.ANSIColor(1) || grey != lipgloss.ANSIColor(8) {
				t.Fatal("Terminal did not restore ANSI palette")
			}
		} else {
			if themeBackground == nil || themeForeground == nil || textStyle.GetForeground() != themeForeground {
				t.Fatalf("%s lacks explicit background/foreground", name)
			}
			if name == "Paper" {
				for _, color := range []lipgloss.TerminalColor{themeForeground, accent, green, yellow, red, grey} {
					if color != lipgloss.Color("#000000") {
						t.Fatal("Paper must use black for all text and visualizer colors")
					}
				}
			} else if themeBackground == themeForeground || grey == accent || red == accent || grey == themeForeground {
				t.Fatalf("%s does not distinguish background, text, dim, accent and error", name)
			}
		}
		if selectedStyle.GetForeground() != accent || errorStyle.GetForeground() != red ||
			dimStyle.GetForeground() != grey || spectrumStyles[0].GetForeground() != green ||
			spectrumStyles[1].GetForeground() != yellow || spectrumStyles[2].GetForeground() != red ||
			!reflect.DeepEqual(onStyle, activeTab) {
			t.Fatalf("%s styles not updated with palette", name)
		}
	}
	if err := applyTheme("Dark"); err != nil {
		t.Fatal(err)
	}
	background, foreground := themeBackground, themeForeground
	if err := applyTheme("invalid"); err == nil {
		t.Fatal("unknown theme accepted")
	}
	if themeBackground != background || themeForeground != foreground {
		t.Fatal("invalid theme mutated palette")
	}
}

func TestThemeSettingsPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path, err := themeSettingsPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "cider-tui", "settings.json"); path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}

func TestThemePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "cider-tui", "settings.json")
	if got, err := loadTheme(path); err != nil || got != "Terminal" {
		t.Fatalf("missing settings = %q, %v", got, err)
	}
	for _, name := range themeNames {
		if err := saveTheme(path, name); err != nil {
			t.Fatalf("save %s: %v", name, err)
		}
		if got, err := loadTheme(path); err != nil || got != name {
			t.Fatalf("load = %q, %v; want %q", got, err, name)
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "{\"theme\":\""+name+"\",\"background\":true}\n" {
			t.Fatalf("saved JSON = %q, %v", data, err)
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("settings permissions = %v, %v", info, err)
		}
	}
	info, err := os.Stat(filepath.Dir(path))
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatalf("settings directory permissions = %v, %v", info, err)
	}
	if err := saveTheme(path, "unknown"); err == nil {
		t.Fatal("saved unknown theme")
	}
	if got, err := loadTheme(path); err != nil || got != "High contrast" {
		t.Fatalf("invalid save changed settings: %q, %v", got, err)
	}
}

func TestLoadThemeRejectsBadSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	for _, data := range []string{`{"theme":"No such theme"}`, `{}`, `{`, `{"theme":"Dark"} {}`} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadTheme(path); err == nil {
			t.Fatalf("accepted bad settings %q", data)
		}
	}
}

func TestThemeSettingsIOErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(path, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadTheme(filepath.Join(path, "settings.json")); err == nil {
		t.Fatal("load suppressed path error")
	}
	if err := saveTheme(filepath.Join(path, "settings.json"), "Dark"); err == nil {
		t.Fatal("save suppressed path error")
	}
}

func TestVisualizerSettingsPersistenceAndLegacyDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"theme":"Dark","background":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	config, err := loadAppearance(path)
	if err != nil || config.Visualizer != "spectrum" {
		t.Fatalf("legacy visualizer = %q, %v", config.Visualizer, err)
	}
	config.Visualizer = "waveform"
	if err := saveAppearance(path, config); err != nil {
		t.Fatal(err)
	}
	if got, err := loadAppearance(path); err != nil || got != config {
		t.Fatalf("loaded = %+v, %v; want %+v", got, err, config)
	}
	if err := saveAppearance(path, appearanceSettings{Theme: "Dark", Background: true, Visualizer: "invalid"}); err == nil {
		t.Fatal("saved invalid visualizer")
	}
	if err := os.WriteFile(path, []byte(`{"theme":"Dark","visualizer":"invalid"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadAppearance(path); err == nil {
		t.Fatal("loaded invalid visualizer")
	}
}
