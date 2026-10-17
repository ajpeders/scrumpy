package ui

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ajpeders/scrumpy/viz"
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

// TestWaveformFlattensSynthSilence: the bug behind CID-11. A captured
// "silent" hop in the real world isn't all zeros — it carries codec DC
// offset, capture jitter and the occasional stray sample. The waveform
// used to track full min/max per column, scaled by (height-1)/2, so any
// stray non-zero sample painted a full-height spike. The fix gates
// samples below a noise floor and percentile-clamps each column to
// dampen isolated spikes. After the fix, a hop that's effectively
// silence must render as a flat centre line — no bars in the visible
// rows except the centre row itself.
func TestWaveformFlattensSynthSilence(t *testing.T) {
	// Mostly silence with a few realistic artefacts: a small DC offset
	// (capture path), a sub-noise-floor jitter sample, and one outlier
	// that's a real signal but isolated to a single sample per column.
	samples := make([]float64, 200)
	for i := range samples {
		switch {
		case i%47 == 0:
			samples[i] = 0.0005 // sub-noise-floor jitter
		case i == 53:
			samples[i] = 0.4 // an outlier: a single loud sample
		default:
			samples[i] = 0.0001 // tiny DC offset
		}
	}
	lines := renderWaveform(samples, 40, 9)
	centre := len(lines) / 2
	// Only the centre row may show bars — every other row must be empty.
	for row, line := range lines {
		plain := ansi.Strip(line)
		hasBars := strings.ContainsRune(plain, '│')
		if row == centre {
			continue
		}
		if hasBars {
			t.Fatalf("row %d had bars on synth silence:\n%s", row, renderRows(lines, centre))
		}
	}
}

func renderRows(lines []string, centre int) string {
	var b strings.Builder
	for i, l := range lines {
		marker := " "
		if i == centre {
			marker = ">"
		}
		fmt.Fprintf(&b, "%s[%d] %q\n", marker, i, ansi.Strip(l))
	}
	return b.String()
}

// TestWaveformPreservesRealSignal: the percentile clamp must not eat
// real music. A column full of legitimately loud samples must still
// paint out from the centre line; the clamp is there to dampen
// outliers, not to flatten the envelope.
func TestWaveformPreservesRealSignal(t *testing.T) {
	// Build 50 samples per column: a half-sine from -0.6 to +0.6, which
	// is well above the noise floor and well below the outlier we'd
	// expect to see in real audio.
	const cols, perCol = 20, 50
	samples := make([]float64, cols*perCol)
	for c := range cols {
		base := c * perCol
		for i := range perCol {
			t := float64(i) / float64(perCol-1) // 0..1
			samples[base+i] = 0.6 * (2*t - 1)
		}
	}
	lines := renderWaveform(samples, cols, 9)
	centre := len(lines) / 2
	// At least one row above and one below the centre must have bars
	// somewhere — otherwise the clamp flattened the signal.
	above := false
	below := false
	for row, line := range lines {
		if row == centre {
			continue
		}
		if strings.ContainsRune(ansi.Strip(line), '│') {
			if row < centre {
				above = true
			} else {
				below = true
			}
		}
	}
	if !above || !below {
		t.Fatalf("real signal collapsed to centre line:\n%s",
			renderRows(lines, centre))
	}
}

func TestSwitchVisualizerUsesCapturedWaveform(t *testing.T) {
	frames := make(chan viz.Frame, 1)
	frames <- viz.Frame{Bands: []float64{1, 1}, Samples: []float64{-1, 1}}
	m := New(&fakeClient{}).WithFrames(frames)
	m.now.Playing = true
	updated, _ := m.Update(animMsg(time.Now())) // the tick reads the frame
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
	updated, _ = m.Update(animMsg(time.Now()))
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

// A short column of symmetric audio must clamp to a symmetric envelope:
// trimming by magnitude used to drop only the louder polarity's peak.
func TestPercentileClampSmallColumnSymmetric(t *testing.T) {
	got := percentileClamp([]float64{0.9, -0.85, 0.3, -0.3})
	hi, lo := slices.Max(got), slices.Min(got)
	if hi != -lo {
		t.Fatalf("percentileClamp = %v; envelope [%v, %v] is lopsided", got, lo, hi)
	}
}
