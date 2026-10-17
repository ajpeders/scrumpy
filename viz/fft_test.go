package viz

import (
	"math"
	"testing"
)

func TestSineLandsInLogBand(t *testing.T) {
	const bands = 24
	const frequency = 750
	samples := make([]float64, fftSize)
	for i := range samples {
		samples[i] = 0.7 * math.Sin(2*math.Pi*frequency*float64(i)/sampleRate)
	}
	var g gain
	bars := g.apply(spectrum(samples, bands))
	want := int(math.Floor(math.Log(frequency/40.0) / math.Log(16000.0/40) * bands))
	for i, level := range bars {
		if i != want && i != want-1 && i != want+1 && level >= bars[want] {
			t.Fatalf("750 Hz peak outside expected band %d: bars=%v", want, bars)
		}
	}
	if bars[want] < 0.9 {
		t.Fatalf("expected the loudest band %d near full height: bars=%v", want, bars)
	}
	for _, level := range g.apply(spectrum(make([]float64, fftSize), bands)) {
		if level != 0 {
			t.Fatalf("silence produced nonzero bar: %v", level)
		}
	}
}

func TestGainScalesToVolume(t *testing.T) {
	// The same spectrum shape at -40 dB and at -10 dB fills the same height.
	var quiet, loud gain
	q := quiet.apply([]float64{-40, -55})
	l := loud.apply([]float64{-10, -25})
	if q[0] != 1 || l[0] != 1 || math.Abs(q[1]-l[1]) > 1e-9 {
		t.Fatalf("quiet %v loud %v", q, l)
	}
	// Faint noise below the gate stays empty.
	if n := (&gain{}).apply([]float64{-75, -78}); n[0] != 0 || n[1] != 0 {
		t.Fatalf("noise = %v", n)
	}
	// After a loud frame, a quieter one drops instead of being rescaled up.
	if after := loud.apply([]float64{-20, -35}); after[0] > 0.8 {
		t.Fatalf("reference fell too fast: %v", after)
	}
}
