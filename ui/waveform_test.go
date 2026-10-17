package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/ajpeders/cider-tui/viz"
	"github.com/charmbracelet/x/ansi"
)

func TestWaveformEnvelopeAndSilence(t *testing.T) {
	lines := renderWaveform([]float64{-1, 1, 0, 0}, 2, 5)
	if len(lines) != 5 {
		t.Fatalf("rows = %d", len(lines))
	}
	for _, line := range lines {
		if ansi.StringWidth(line) != 2 {
			t.Fatalf("width: %q", line)
		}
	}
	if got := ansi.Strip(lines[0]); got != "│ " {
		t.Fatalf("top = %q", got)
	}
	if got := ansi.Strip(lines[4]); got != "│ " {
		t.Fatalf("bottom = %q", got)
	}
	if got := ansi.Strip(renderWaveform(nil, 2, 5)[2]); got != "││" {
		t.Fatalf("silence = %q", got)
	}
}

func TestSwitchVisualizerUsesCapturedWaveform(t *testing.T) {
	frames := make(chan viz.Frame, 1)
	frames <- viz.Frame{Bands: []float64{1, 1}, Samples: []float64{-1, 1}}
	m := New(&fakeClient{}).WithFrames(frames)
	m.now.Playing = true
	updated, _ := m.Update(waitFrames(frames)())
	m = updated.(Model)
	updated, _ = m.Update(animMsg(time.Now()))
	m = updated.(Model)
	m, _ = sendKey(t, m, runeKey('b'))
	if !m.waveform || !strings.Contains(ansi.Strip(m.View()), "│") {
		t.Fatal("waveform not shown after switching")
	}
	m, _ = sendKey(t, m, runeKey('b'))
	if m.waveform || !strings.Contains(ansi.Strip(m.View()), "█") {
		t.Fatal("spectrum not restored")
	}
	close(frames)
	updated, _ = m.Update(waitFrames(frames)())
	m = updated.(Model)
	if rows, _ := m.layout(); rows != 0 {
		t.Fatalf("visualizer still shown after close: %d rows", rows)
	}
}

func TestWaveformDisplaysLatestHopAtSlowerRate(t *testing.T) {
	m := New(&fakeClient{})
	start := time.Now()
	m.waveTarget = []float64{0.1}
	updated, _ := m.Update(animMsg(start))
	m = updated.(Model)
	if m.samples[0] != 0.1 {
		t.Fatal("first waveform hop was delayed")
	}
	m.waveTarget = []float64{0.2}
	updated, _ = m.Update(animMsg(start.Add(waveformInterval / 2)))
	m = updated.(Model)
	if m.samples[0] != 0.1 {
		t.Fatal("waveform changed before interval elapsed")
	}
	m.waveTarget = []float64{0.3}
	updated, _ = m.Update(animMsg(start.Add(waveformInterval)))
	m = updated.(Model)
	if m.samples[0] != 0.3 {
		t.Fatal("waveform did not show latest hop after interval")
	}
}
