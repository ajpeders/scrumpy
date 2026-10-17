package viz

import (
	"context"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Once the FFT window has filled, every hop must produce fresh bands so the
// spectrum updates at the hop rate (60 Hz), not once per full window.
func TestCaptureUpdatesBandsEveryHop(t *testing.T) {
	const hops = 8
	dir := t.TempDir()
	raw := make([]byte, 0, hops*hopFrames*8)
	for h := range hops {
		freq := 200.0 * float64(h+1) // a different tone each hop so the spectrum moves
		for i := range hopFrames {
			s := float32(0.5 * math.Sin(2*math.Pi*freq*float64(h*hopFrames+i)/48000))
			raw = binary.LittleEndian.AppendUint32(raw, math.Float32bits(s))
			raw = binary.LittleEndian.AppendUint32(raw, math.Float32bits(s))
		}
	}
	data := filepath.Join(dir, "audio.raw")
	if err := os.WriteFile(data, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	recorder := filepath.Join(dir, "pw-record")
	if err := os.WriteFile(recorder, []byte("#!/bin/sh\ncat "+data+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	out := make(chan Frame, hops)
	(&Analyzer{Bands: 32}).capture(context.Background(), recorder, "1", out)
	close(out)
	var frames []Frame
	for f := range out {
		frames = append(frames, f)
	}
	if len(frames) != hops {
		t.Fatalf("got %d frames, want %d", len(frames), hops)
	}
	warm := (fftSize + hopFrames - 1) / hopFrames // first hop with a full window
	for i := warm; i < hops; i++ {
		if slices.Equal(frames[i].Bands, frames[i-1].Bands) {
			t.Errorf("hop %d reused the previous hop's bands", i)
		}
	}
}

// Near-silent input (PipeWire dither) must not be blown up to full scale, or
// the UI's noise gate can't hide it. Normal-level audio still fills ±1.
func TestNormaliseCapsGainOnQuietInput(t *testing.T) {
	dither := []float64{1e-5, -1e-5, 0.5e-5}
	for _, v := range normalise(dither) {
		if math.Abs(v) >= 0.01 {
			t.Fatalf("dither normalised to %v; want below the 0.01 waveform noise floor", v)
		}
	}
	loud := normalise([]float64{0.4, -0.2})
	if loud[0] != 1 || loud[1] != -0.5 {
		t.Fatalf("normalise(0.4, -0.2) = %v; want [1 -0.5]", loud)
	}
}
