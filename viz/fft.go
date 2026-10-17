package viz

import "math"

const (
	sampleRate = 48000
	fftSize    = 2048
	silenceDB  = -80 // band energy treated as silence (-inf for digital zero)
)

// spectrum maps a mono PCM window to the mean energy, in dB relative to a
// full-scale sine, of logarithmically spaced bands from 40 Hz to 16 kHz. Bands
// narrower than one FFT bin read the bin they fall in. The Hann window limits
// leakage. Averaging energy (as cliamp does) rather than taking the peak bin
// keeps wide treble bands from dominating.
func spectrum(samples []float64, bands int) []float64 {
	if bands <= 0 {
		return nil
	}
	out := make([]float64, bands)
	if len(samples) != fftSize {
		for i := range out {
			out[i] = silenceDB
		}
		return out
	}
	x := make([]complex128, fftSize)
	for i, sample := range samples {
		x[i] = complex(sample*(0.5-0.5*math.Cos(2*math.Pi*float64(i)/float64(fftSize-1))), 0)
	}
	fft(x)
	edge := func(band int) float64 {
		return 40 * math.Pow(16000.0/40, float64(band)/float64(bands)) * fftSize / sampleRate
	}
	for band := range out {
		lo, hi := int(edge(band)), int(math.Ceil(edge(band+1)))
		hi = min(max(hi, lo+1), fftSize/2)
		var sum float64
		for bin := lo; bin < hi; bin++ {
			m := cmplxAbs(x[bin]) * 4 / fftSize
			sum += m * m
		}
		out[band] = max(silenceDB, 10*math.Log10(sum/float64(hi-lo)+1e-12))
	}
	return out
}

// gain maps band energies to [0, 1] levels relative to the loudest recent
// band, so bars use the full height at any playback volume. The reference
// rises instantly and falls at refFall dB per frame; bands rangeDB or more
// below it are empty. Frames whose loudest band is below gateDB are silence.
type gain struct {
	ref float64
	set bool // the first audible frame sets ref
}

const (
	rangeDB = 42
	refFall = 0.08 // dB per analysis frame (~5 dB/s at 60 frames/s)
	minRef  = -45  // keep quiet passages from being amplified into noise
	gateDB  = -70
)

func (g *gain) apply(db []float64) []float64 {
	loudest := silenceDB * 1.0
	for _, v := range db {
		loudest = max(loudest, v)
	}
	levels := make([]float64, len(db))
	if loudest < gateDB {
		return levels
	}
	if !g.set {
		g.ref, g.set = loudest, true
	}
	g.ref = max(loudest, g.ref-refFall, minRef)
	for i, v := range db {
		levels[i] = min(1, max(0, (v-g.ref+rangeDB)/rangeDB))
	}
	return levels
}

func cmplxAbs(x complex128) float64 { return math.Hypot(real(x), imag(x)) }

// In-place radix-2 Cooley–Tukey FFT.
func fft(x []complex128) {
	n := len(x)
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for j&bit != 0 {
			j ^= bit
			bit >>= 1
		}
		j ^= bit
		if i < j {
			x[i], x[j] = x[j], x[i]
		}
	}
	for width := 2; width <= n; width *= 2 {
		angle := -2 * math.Pi / float64(width)
		step := complex(math.Cos(angle), math.Sin(angle))
		for start := 0; start < n; start += width {
			w := complex(1, 0)
			for j := 0; j < width/2; j++ {
				a := x[start+j]
				b := w * x[start+j+width/2]
				x[start+j], x[start+j+width/2] = a+b, a-b
				w *= step
			}
		}
	}
}
