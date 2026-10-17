package ui

import "math"

// Meter motion, in per-second rates so it is independent of frame timing.
// Bars ease towards the analyzer's latest levels: fast up, slower down (the
// rates cliamp uses). A peak cap holds, then falls at a constant speed.
const (
	barRise   = 34.0
	barFall   = 10.0
	peakHold  = 0.5 // seconds
	peakSpeed = 1.2 // heights per second
)

// meter smooths analyzer levels into what the visualizer draws. It advances
// on the UI's animation clock, not on frame arrival, so uneven delivery from
// pw-record does not show as stutter.
type meter struct {
	levels, peaks, hold []float64
}

// atRest reports that every bar and peak has settled to the floor.
func (m *meter) atRest() bool {
	for i := range m.levels {
		if m.levels[i] > 0.002 || m.peaks[i] > 0.002 {
			return false
		}
	}
	return true
}

func easeToward(cur, target, rise, fall, dt float64) float64 {
	rate := fall
	if target > cur {
		rate = rise
	}
	return cur + (target-cur)*(1-math.Exp(-rate*dt))
}

// advance moves the meter dt seconds towards target. A nil target lets every
// bar fall to rest.
func (m *meter) advance(target []float64, dt float64) {
	if target != nil && len(m.levels) != len(target) {
		m.levels = make([]float64, len(target))
		m.peaks = make([]float64, len(target))
		m.hold = make([]float64, len(target))
	}
	for i := range m.levels {
		goal := 0.0
		if target != nil {
			goal = target[i]
		}
		m.levels[i] = easeToward(m.levels[i], goal, barRise, barFall, dt)
		switch {
		case m.levels[i] >= m.peaks[i]:
			m.peaks[i], m.hold[i] = m.levels[i], peakHold
		case m.hold[i] > 0:
			m.hold[i] -= dt
		default:
			m.peaks[i] = max(m.levels[i], m.peaks[i]-peakSpeed*dt)
		}
	}
}
