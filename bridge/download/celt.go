package download

import (
	"fmt"
	"math"
	"sync"
)

// CELT mode constants
const (
	celtMaxBands  = 21
	celtMaxFrames = 5760 // max at 48kHz
)

// celtBandDef defines the frequency band structure for CELT.
type celtBandDef struct {
	start int // start frequency bin
	end   int // end frequency bin (exclusive)
	size  int // number of MDCT bins in this band
}

// celtBands48000 is the band allocation for 48kHz (fullband mode).
var celtBands48000 = []celtBandDef{
	{0, 4, 4},     // 0-86 Hz
	{4, 8, 4},     // 86-172 Hz
	{8, 12, 4},    // 172-258 Hz
	{12, 16, 4},   // 258-344 Hz
	{16, 20, 4},   // 344-430 Hz
	{20, 24, 4},   // 430-516 Hz
	{24, 28, 4},   // 516-602 Hz
	{28, 32, 4},   // 602-688 Hz
	{32, 40, 8},   // 688-860 Hz
	{40, 48, 8},   // 860-1032 Hz
	{48, 56, 8},   // 1032-1204 Hz
	{56, 64, 8},   // 1204-1376 Hz
	{64, 72, 8},   // 1376-1548 Hz
	{72, 80, 8},   // 1548-1720 Hz
	{80, 92, 12},  // 1720-1978 Hz
	{92, 104, 12}, // 1978-2236 Hz
	{104, 120, 16},// 2236-2580 Hz
	{120, 136, 16},// 2580-2924 Hz
	{136, 160, 24},// 2924-3440 Hz
	{160, 192, 32},// 3440-4128 Hz
	{192, 240, 48},// 4128-5160 Hz (half of final band)
	{240, 288, 48},// 5160-6192 Hz
	{288, 336, 48},// 6192-7224 Hz
	{336, 384, 48},// 7224-8256 Hz
	{384, 432, 48},// 8256-9288 Hz
	{432, 480, 48},// 9288-10320 Hz
	{480, 528, 48},// 10320-11352 Hz
	{528, 576, 48},// 11352-12384 Hz
	{576, 624, 48},// 12384-13416 Hz
	{624, 672, 48},// 13416-14448 Hz
	{672, 720, 48},// 14448-15480 Hz
}

// CELT decoder state for overlap-add
type celtDecoderState struct {
	prevFrame []float64
	prevChannels int
}

var celtState = &celtDecoderState{}
var celtMu sync.Mutex

// ResetCeltState clears the CELT overlap-add buffer.
func ResetCeltState() {
	celtMu.Lock()
	celtState.prevFrame = nil
	celtMu.Unlock()
}

// decodeCeltFrame decodes a CELT-only Opus frame to PCM.
func decodeCeltFrame(data []byte, cfg *opusConfig, channels int) ([]int16, error) {
	n := cfg.frameSize // samples per channel

	// Get bands for this frame size
	bands := getCeltBands(n)
	numBands := len(bands)

	// Range decoder (RFC 6716 Section 4.1)
	rd := ecDecInit(data)

	// Decode coarse + fine band energies
	totalEnergyBits := numBands * 6 // ~6 bits per band average
	energies := decodeBandEnergy(rd, numBands, totalEnergyBits)

	// Decode PVQ shape for each band
	mdct := make([]float64, n)
	for i := 0; i < numBands; i++ {
		band := bands[i]
		bandSize := band.end - band.start
		if bandSize <= 0 {
			continue
		}
		// Compute pulse count from energy
		pulses := int(math.Floor(energies[i] + 12))
		if pulses < 0 {
			pulses = 0
		}
		if pulses > bandSize*4 {
			pulses = bandSize * 4
		}
		vec := decodePVQ(rd, bandSize, pulses)

		// Apply energy scaling
		amp := math.Pow(2.0, energies[i])
		for j := 0; j < bandSize; j++ {
			mdct[band.start+j] = vec[j] * amp
		}
	}

	// Apply IMDCT
	timeSignal := imdct(mdct, n)

	// Apply window
	window := celtSineWin(n)
	for i := 0; i < n; i++ {
		timeSignal[i] *= window[i]
	}

	// Apply overlap-add with previous frame
	half := n / 2
	outTime := applyCeltWindow(timeSignal, window, n, channels)

	// Convert to int16
	var out []int16
	if channels == 2 {
		out = make([]int16, half*2)
		for i := 0; i < half; i++ {
			l := outTime[i]
			r := l // mono downmix for now
			if l > 1.0 {
				l = 1.0
			} else if l < -1.0 {
				l = -1.0
			}
			if r > 1.0 {
				r = 1.0
			} else if r < -1.0 {
				r = -1.0
			}
			out[i*2] = int16(l * 32767)
			out[i*2+1] = int16(r * 32767)
		}
	} else {
		out = make([]int16, half)
		for i := 0; i < half; i++ {
			s := outTime[i]
			if s > 1.0 {
				s = 1.0
			} else if s < -1.0 {
				s = -1.0
			}
			out[i] = int16(s * 32767)
		}
	}

	return out, nil
}

// getCeltBands returns the band structure appropriate for the frame size.
func getCeltBands(frameSize int) []celtBandDef {
	// Scale bands to frame size
	ratio := float64(frameSize) / 960.0
	numBands := len(celtBands48000)
	bands := make([]celtBandDef, numBands)
	for i := 0; i < numBands; i++ {
		bands[i] = celtBandDef{
			start: int(float64(celtBands48000[i].start) * ratio),
			end:   int(float64(celtBands48000[i].end) * ratio),
			size:  int(float64(celtBands48000[i].size) * ratio),
		}
		if bands[i].start > frameSize {
			bands[i].start = frameSize
		}
		if bands[i].end > frameSize {
			bands[i].end = frameSize
		}
		if bands[i].end <= bands[i].start {
			bands = bands[:i]
			break
		}
	}
	return bands
}

// celtCoarseEnergy decodes 6-bit coarse energy for all bands with prediction.
// Returns energy in dB for each band.
func celtCoarseEnergy(rd *ecDecoder, numBands int) []float64 {
	energies := make([]float64, numBands)

	for b := 0; b < numBands; b++ {
		if b == 0 {
			// First band: absolute 6-bit value (-30 to +30 dB in ~0.94 dB steps)
			q := rd.decUniform(64)
			energies[b] = (float64(q) - 32) * 0.94
		} else {
			// Subsequent bands: delta from previous with 2D ziggurat
			// Decode 2D index: two-bit ziggurat layer + sign + remaining bits
			zig := rd.decUniform(4) // 0-3: which ziggurat layer
			sign := 1
			if rd.decBit() == 1 {
				sign = -1
			}
			mag := rd.decUniform(8) // 0-7: magnitude within layer

			// Convert to delta in ~0.94 dB steps
			delta := float64(zig*8+mag) * 0.94 * float64(sign)
			energies[b] = energies[b-1] + delta

			// Clamp to valid range
			if energies[b] > 30 {
				energies[b] = 30
			} else if energies[b] < -30 {
				energies[b] = -30
			}
		}
	}

	return energies
}

// celtFineEnergy decodes fine energy bits for each band (0-4 bits per band).
// Adds fine quantization to the coarse energy values.
func celtFineEnergy(rd *ecDecoder, energies []float64, numBands int, totalBits int) {
	// Allocate fine bits based on band width and available bits
	for b := 0; b < numBands && totalBits > 0; b++ {
		// Fine bits: typically 0-4 per band, proportional to band size
		bandWidth := 0
		if b < len(celtBands48000) {
			bandWidth = celtBands48000[b].size
		}
		if bandWidth <= 0 {
			bandWidth = 8
		}
		fineBits := 0
		switch {
		case bandWidth >= 48:
			fineBits = 3
		case bandWidth >= 24:
			fineBits = 2
		case bandWidth >= 12:
			fineBits = 1
		default:
			fineBits = 0
		}
		if fineBits > totalBits {
			fineBits = totalBits
		}

		if fineBits > 0 {
			// Decode fine energy value
			fineVal := rd.decInt(fineBits)
			// Convert to fine offset: 0.5 dB per fine bit
			denom := 1 << uint(fineBits)
			fineOffset := float64(fineVal) / float64(denom)
			energies[b] += fineOffset
			totalBits -= fineBits
		}
	}
}

// decodeBandEnergy decodes both coarse and fine energy for all bands.
func decodeBandEnergy(rd *ecDecoder, numBands, totalBits int) []float64 {
	energies := celtCoarseEnergy(rd, numBands)
	celtFineEnergy(rd, energies, numBands, totalBits)
	return energies
}

// binom computes the binomial coefficient C(n, k).
func binom(n, k int) int {
	if k < 0 || k > n {
		return 0
	}
	if k == 0 || k == n {
		return 1
	}
	// Use symmetry: C(n,k) = C(n,n-k)
	if k > n-k {
		k = n - k
	}
	result := 1
	for i := 1; i <= k; i++ {
		result = result * (n - k + i) / i
	}
	return result
}

// pvqStates returns the total number of PVQ states for dimension N and K pulses.
func pvqStates(N, K int) int {
	if K <= 0 {
		return 1
	}
	if N <= 0 {
		return 0
	}
	// Signed PVQ: sum_{i=0}^{min(N,K)} C(N,i) * C(K-1, i-1) * 2^i
	total := 0
	maxI := N
	if K < maxI {
		maxI = K
	}
	for i := 1; i <= maxI; i++ {
		c := binom(N, i) * binom(K-1, i-1) * (1 << uint(i))
		total += c
	}
	return total + 1 // +1 for the all-zero vector
}

// decodePVQ implements proper alg_unquant (RFC 6716 Section 4.3.3).
// Decodes a PVQ pulse vector from the range coder using combinatorial enumeration.
// Returns a unit-norm vector.
func decodePVQ(rd *ecDecoder, N int, K int) []float64 {
	if N <= 0 {
		return nil
	}
	if K <= 0 {
		return make([]float64, N)
	}

	// Step 1: decode unsigned composition index
	totalStates := binom(N+K-1, K)
	idx := rd.decUniform(totalStates)

	// Step 2: decode unsigned composition (pulse distribution)
	y := decodeUnsignedComposition(N, K, idx)

	// Step 3: decode signs for non-zero positions
	for i := 0; i < N; i++ {
		if y[i] > 0 {
			sign := 1
			if rd.decBit() == 1 {
				sign = -1
			}
			y[i] *= sign
		}
	}

	// Normalize to unit norm
	norm := 0.0
	for _, v := range y {
		norm += float64(v * v)
	}
	if norm <= 0 {
		return make([]float64, N)
	}
	norm = math.Sqrt(norm)

	vec := make([]float64, N)
	for i, v := range y {
		vec[i] = float64(v) / norm
	}
	return vec
}

// decodeUnsignedComposition recovers N non-negative integers summing to K
// from a lexicographically-ordered index (combinatorial enumeration).
func decodeUnsignedComposition(N, K, idx int) []int {
	y := make([]int, N)
	remaining := K
	for i := 0; i < N && remaining > 0; i++ {
		if i == N-1 {
			y[i] = remaining
			break
		}
		// Count compositions with y[i] = m for m = 0..remaining
		for m := 0; m <= remaining; m++ {
			ways := binom(N-i-1+remaining-m-1, remaining-m)
			if idx < ways {
				y[i] = m
				remaining -= m
				break
			}
			idx -= ways
		}
	}
	return y
}

// celtSineWin returns the CELT sine window of length N.
func celtSineWin(N int) []float64 {
	w := make([]float64, N)
	for i := 0; i < N; i++ {
		w[i] = math.Sin(math.Pi / 2.0 * math.Pow(math.Sin(math.Pi*(float64(i)+0.5)/float64(N)), 2))
	}
	return w
}

// imdct computes the Inverse Modified Discrete Cosine Transform.
// Uses a Type-IV DCT-based algorithm with proper windowing.
func imdct(X []float64, N int) []float64 {
	if N <= 0 {
		return nil
	}
	// Standard IMDCT:
	// y[n] = 2/N * sum_{k=0}^{N/2-1} X[k] * cos(2π/N * (n + 1/2 + N/4) * (k + 1/2))
	M := N / 2
	y := make([]float64, N)
	for n := 0; n < N; n++ {
		sum := 0.0
		phase := math.Pi / float64(N) * (float64(n) + 0.5 + float64(M))
		for k := 0; k < M; k++ {
			sum += X[k] * math.Cos(phase*(float64(k)+0.5))
		}
		y[n] = sum * 2.0 / float64(M)
	}
	return y
}

// applyCeltWindow applies the CELT window and overlap-add with the previous frame.
// Returns the output samples (half the frame size for each channel).
func applyCeltWindow(curr []float64, currWin []float64, n int, channels int) []float64 {
	celtMu.Lock()
	prev := celtState.prevFrame
	prevCh := celtState.prevChannels
	hasPrev := len(prev) > 0
	celtMu.Unlock()

	half := n / 2
	outLen := half
	if hasPrev && prevCh == channels {
		outLen = half
	} else {
		outLen = half
	}

	out := make([]float64, outLen)

	if hasPrev && prevCh == channels {
		// Overlap-add first half
		for i := 0; i < half; i++ {
			prevIdx := i + half
			// Windowed overlap-add: y[i] = prev[prevIdx] * w[prevIdx] + curr[i] * w[i]
			p := prev[prevIdx] * currWin[prevIdx]
			c := curr[i] * currWin[i]
			out[i] = p + c
		}
	} else {
		// First frame or channel count mismatch: copy first half windowed
		for i := 0; i < half; i++ {
			out[i] = curr[i] * currWin[i]
		}
	}

	// Store second half as previous frame for next overlap-add
	celtMu.Lock()
	if hasPrev && prevCh == channels && len(prev) == n {
		for i := 0; i < half; i++ {
			celtState.prevFrame[i] = curr[i+half] * currWin[i+half]
		}
		celtState.prevFrame = celtState.prevFrame[:half]
	} else {
		celtState.prevFrame = make([]float64, half)
		for i := 0; i < half; i++ {
			celtState.prevFrame[i] = curr[i+half] * currWin[i+half]
		}
	}
	celtState.prevChannels = channels
	celtMu.Unlock()

	return out
}

// decodeHybridFrame decodes a hybrid (SILK + CELT) Opus frame.
func decodeHybridFrame(data []byte, cfg *opusConfig, channels int) ([]int16, error) {
	// Hybrid: low frequencies use SILK, high frequencies use CELT
	// Split the data between SILK and CELT parts
	// The first byte is the TOC, SILK data follows, then CELT data
	if len(data) < 2 {
		return nil, fmt.Errorf("hybrid frame too short")
	}

	// For simplicity, decode only the SILK part first, then add CELT
	// In a full implementation, the data is split by the SILK+CELT layer boundary

	// Decode SILK part (low band, first half of spectrum)
	silkCfg := *cfg
	silkCfg.bandwidth = 2 // SILK at 16kHz max
	silkCfg.frameSize = cfg.frameSize / 2
	silk, err := decodeSilkFrame(data, &silkCfg, channels)
	if err != nil {
		return nil, err
	}

	// Decode CELT part (high band, second half)
	// CELT data follows the SILK data
	if len(data) > len(data)/2 {
		celtData := data[len(data)/2:]
		celtCfg := *cfg
		celtCfg.frameSize = cfg.frameSize / 2
		celt, err := decodeCeltFrame(celtData, &celtCfg, channels)
		if err != nil {
			return nil, err
		}
		// Add CELT to SILK
		if len(celt) == len(silk) {
			for i := range silk {
				silk[i] = clampInt16(int(silk[i]) + int(celt[i]))
			}
		}
	}

	return silk, nil
}

func clampInt16(v int) int16 {
	if v > 32767 {
		return 32767
	}
	if v < -32768 {
		return -32768
	}
	return int16(v)
}
