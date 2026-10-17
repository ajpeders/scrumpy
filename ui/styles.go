package ui

import "github.com/charmbracelet/lipgloss"

// Terminal uses ANSI 16 colours so the UI follows the user's palette.
// State never relies on hue alone: active things are reversed or bold.
var (
	accent lipgloss.TerminalColor = lipgloss.ANSIColor(1) // terminal red
	green  lipgloss.TerminalColor = lipgloss.ANSIColor(10)
	yellow lipgloss.TerminalColor = lipgloss.ANSIColor(11)
	red    lipgloss.TerminalColor = lipgloss.ANSIColor(9)
	grey   lipgloss.TerminalColor = lipgloss.ANSIColor(8)

	titleStyle    = lipgloss.NewStyle().Foreground(accent).Bold(true)
	textStyle     = lipgloss.NewStyle() // terminal foreground
	dimStyle      = lipgloss.NewStyle().Foreground(grey)
	faintStyle    = lipgloss.NewStyle().Foreground(grey).Faint(true)
	accentStyle   = lipgloss.NewStyle().Foreground(accent)
	playingStyle  = lipgloss.NewStyle().Foreground(accent)
	errorStyle    = lipgloss.NewStyle().Foreground(red)
	selectedStyle = lipgloss.NewStyle().Foreground(accent).Bold(true)
	tabStyle      = lipgloss.NewStyle().Foreground(grey)
	activeTab     = lipgloss.NewStyle().Foreground(accent).Reverse(true).Bold(true)
	onStyle       = activeTab // a toggle that is on
	keyStyle      = lipgloss.NewStyle().Foreground(accent).Bold(true)

	// Spectrum tiers by row height: low green, middle yellow, top red.
	spectrumStyles = [3]lipgloss.Style{
		lipgloss.NewStyle().Foreground(green),
		lipgloss.NewStyle().Foreground(yellow),
		lipgloss.NewStyle().Foreground(red),
	}
)

func spectrumStyle(rowBottom float64) lipgloss.Style {
	switch {
	case rowBottom >= 0.6:
		return spectrumStyles[2]
	case rowBottom >= 0.3:
		return spectrumStyles[1]
	}
	return spectrumStyles[0]
}
