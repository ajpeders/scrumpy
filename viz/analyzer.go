// Package viz captures PipeWire output for terminal visualizers.
package viz

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os/exec"
	"time"
)

const hopFrames = 800 // 60 windows/second at 48 kHz

// retryInterval is how often pw-dump is re-scanned for Cider's stream.
// stallTimeout ends a capture that delivers no audio: PipeWire sometimes never
// links a pw-record started as Cider's stream appears, and it then waits
// forever. Variables so tests can shorten them.
var (
	retryInterval = 2 * time.Second
	stallTimeout  = 3 * time.Second
)

// Analyzer produces bands of levels in [0, 1]. Construct one per UI
// session; Run owns its channel and stops all children when ctx is cancelled.
type Analyzer struct {
	Bands int
}

// Frame carries both frequency bands and a short mono PCM waveform from the
// same capture. Samples are copies, safe to retain until the next UI update.
type Frame struct {
	Bands, Samples []float64
}

func NewAnalyzer(bands int) *Analyzer { return &Analyzer{Bands: bands} }

// Run returns immediately. Missing PipeWire executables disable the visualizer
// quietly rather than accidentally recording the default microphone.
func (a *Analyzer) Run(ctx context.Context) <-chan []float64 {
	frames := a.RunFrames(ctx)
	out := make(chan []float64, 1)
	go func() {
		defer close(out)
		for frame := range frames {
			publish(out, frame.Bands)
		}
	}()
	return out
}

// Available reports whether the PipeWire tools the visualizer needs are on PATH.
func Available() error {
	for _, tool := range []string{"pw-record", "pw-dump"} {
		if _, err := exec.LookPath(tool); err != nil {
			return fmt.Errorf("%s not found; install PipeWire's command-line tools", tool)
		}
	}
	return nil
}

// RunFrames returns spectrum and waveform data from one PipeWire capture.
func (a *Analyzer) RunFrames(ctx context.Context) <-chan Frame {
	out := make(chan Frame, 1)
	if a == nil || a.Bands <= 0 {
		close(out)
		return out
	}
	if Available() != nil {
		close(out)
		return out
	}
	path, _ := exec.LookPath("pw-record")
	go a.run(ctx, path, out)
	return out
}

func (a *Analyzer) run(ctx context.Context, recordPath string, out chan Frame) {
	defer close(out)
	ticker := time.NewTicker(retryInterval)
	defer ticker.Stop()
	var stop context.CancelFunc
	var finished chan struct{}
	var target string
	start := func() {
		cmdCtx, cancel := context.WithCancel(ctx)
		stop = cancel
		finished = make(chan struct{})
		go func(done chan struct{}) {
			defer close(done)
			a.capture(cmdCtx, recordPath, target, out)
		}(finished)
	}
	defer func() {
		if stop != nil {
			stop()
			<-finished
		}
	}()
	refresh := func() {
		next := ""
		if dump, err := exec.LookPath("pw-dump"); err == nil {
			probe, cancel := context.WithTimeout(ctx, time.Second)
			data, err := exec.CommandContext(probe, dump).Output()
			cancel()
			if err == nil {
				next = discoverTarget(data)
			}
		}
		if finished != nil && next == target {
			return
		}
		if stop != nil {
			stop()
			<-finished
			// Clear both: a closed finished left behind would fire in the
			// main loop with no capture to stop.
			stop, finished = nil, nil
		}
		target = next
		if target != "" {
			start()
		} else {
			publish(out, Frame{Bands: make([]float64, a.Bands)})
		}
	}
	refresh()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refresh()
		case <-finished:
			stop() // release the exited capture's context
			finished = nil
			stop = nil
			publish(out, Frame{Bands: make([]float64, a.Bands)})
		}
	}
}

func publish[T any](out chan T, frame T) {
	select {
	case out <- frame:
	default:
		// Keep the latest frame when the UI is busy.
		select {
		case <-out:
		default:
		}
		select {
		case out <- frame:
		default:
		}
	}
}

func (a *Analyzer) capture(ctx context.Context, path, target string, out chan Frame) {
	// target is always a Cider stream serial; without --target pw-record would
	// capture the default microphone.
	args := []string{"--raw", "--format", "f32", "--rate", "48000", "--channels", "2", "--target", target, "-"}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	watchdog := time.AfterFunc(stallTimeout, cancel) // reset on every block
	defer watchdog.Stop()
	cmd := exec.CommandContext(ctx, path, args...)
	pipe, err := cmd.StdoutPipe()
	if err != nil || cmd.Start() != nil {
		return
	}
	defer cmd.Wait()
	block := make([]byte, hopFrames*2*4)
	window := make([]float64, fftSize)
	filled := 0
	samples := make([]float64, hopFrames)
	var g gain
	for {
		_, err := io.ReadFull(pipe, block)
		if err != nil {
			return
		}
		watchdog.Reset(stallTimeout)
		copy(window, window[hopFrames:])
		for i := 0; i < hopFrames; i++ {
			left := math.Float32frombits(binary.LittleEndian.Uint32(block[i*8:]))
			right := math.Float32frombits(binary.LittleEndian.Uint32(block[i*8+4:]))
			samples[i] = float64(left+right) / 2
			window[fftSize-hopFrames+i] = samples[i]
		}
		filled += hopFrames
		if filled >= fftSize {
			publish(out, Frame{Bands: g.apply(spectrum(window, a.Bands)), Samples: append([]float64(nil), samples...)})
		}
	}
}
