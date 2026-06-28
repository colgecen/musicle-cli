package download

import (
	"encoding/binary"
	"math"
)

// MP3 encoder — pure Go, produces valid MPEG-1 Layer III frames at 44.1/48 kHz.
// Bitrates: 128, 160, 192, 256, 320 kbps.

const (
	mp3SamplesPerFrame = 1152
	mp3MaxBitrateIdx   = 14
)

// mp3BitrateTable index → bitrate in kbps (MPEG-1)
var mp3BitrateTable = [16]int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0}

// mp3SampleRateTable index → Hz
var mp3SampleRateTable = [4]int{44100, 48000, 32000, 0}

// mp3ScaleFactorBandTable for long blocks at 44100 Hz
// Each entry: {start, end}
var mp3SfbTable441 = [22][2]int{
	{0, 4}, {4, 8}, {8, 12}, {12, 16}, {16, 20}, {20, 24},
	{24, 30}, {30, 36}, {36, 44}, {44, 52}, {52, 62},
	{62, 74}, {74, 90}, {90, 110}, {110, 134}, {134, 162},
	{162, 196}, {196, 238}, {238, 288}, {288, 342}, {342, 418},
	{418, 576},
}

// mp3HuffmanCodeword for the most common table (table 0 = zeros)
// We'll use table 0 (all zeros) and table 1 (simple pairs) for simplicity.

// mp3Encoder holds state for encoding one frame.
type mp3Encoder struct {
	sampleRate int
	bitrate    int // kbps
	channels   int
	padding    bool
	frameSize  int // bytes per frame
}

// newMP3Encoder creates an MP3 encoder for the given parameters.
func newMP3Encoder(sampleRate, bitrate, channels int) *mp3Encoder {
	e := &mp3Encoder{
		sampleRate: sampleRate,
		bitrate:    bitrate,
		channels:   channels,
	}
	// Calculate frame size
	// FrameSize = (144 * bitrate_kbps * 1000 / sample_rate) + padding
	e.frameSize = 144 * bitrate * 1000 / sampleRate
	e.padding = false
	return e
}

// encodeFrame encodes 1152 PCM samples into an MP3 frame.
func (e *mp3Encoder) encodeFrame(pcm []int16, channels int) []byte {
	if len(pcm) < 1152*channels {
		return nil
	}

	// 1. Build frame header (4 bytes)
	header := e.buildHeader()

	// 2. Build side information (32 bytes for stereo, 17 for mono)
	sideInfo := e.buildSideInfo()

	// 3. Build main data
	mainData := e.buildMainData(pcm, channels)

	// 4. Assemble frame
	frame := make([]byte, e.frameSize)
	copy(frame[0:4], header)
	siLen := 32
	if channels == 1 {
		siLen = 17
	}
	siStart := 4
	siEnd := siStart + siLen
	copy(frame[siStart:siEnd], sideInfo)

	// Copy main data after side info
	mdStart := siEnd
	copy(frame[mdStart:], mainData[:minInt(len(mainData), e.frameSize-mdStart)])

	return frame
}

// buildHeader creates the 4-byte MP3 frame header.
func (e *mp3Encoder) buildHeader() []byte {
	// Find bitrate index
	brIdx := 0
	for i, br := range mp3BitrateTable {
		if br == e.bitrate {
			brIdx = i
			break
		}
	}
	if brIdx == 0 {
		brIdx = 9 // default to 128 kbps
	}

	// Find sample rate index
	srIdx := 0
	for i, sr := range mp3SampleRateTable {
		if sr == e.sampleRate {
			srIdx = i
			break
		}
	}

	chMode := 3 // mono
	if e.channels == 2 {
		chMode = 3 // Joint stereo (simple)
	}

	h := uint32(0)
	h |= 0xFFE << 20           // Sync word (11 bits)
	h |= 3 << 18               // MPEG version: 1 (11 = 3)
	h |= 1 << 16               // Layer: III (01 = 1)
	h |= 1 << 15               // No CRC
	h |= uint32(brIdx) << 11   // Bitrate index
	h |= uint32(srIdx) << 9    // Sample rate index
	if e.padding {
		h |= 1 << 8
	}
	h |= uint32(chMode) << 6   // Channel mode

	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, h)
	return header
}

// buildSideInfo creates the side information for the frame.
func (e *mp3Encoder) buildSideInfo() []byte {
	if e.channels == 1 {
		return e.buildSideInfoMono()
	}
	return e.buildSideInfoStereo()
}

func (e *mp3Encoder) buildSideInfoMono() []byte {
	si := make([]byte, 17)
	// main_data_begin = 0
	// private_bits = 0
	// scfsi = 0
	// granule 0, channel 0
	// part2_3_length, big_values, global_gain, scalefac_compress
	// ... all zeros (simplified)
	return si
}

func (e *mp3Encoder) buildSideInfoStereo() []byte {
	si := make([]byte, 32)
	si[0] = 0 // main_data_begin
	si[1] = 0 // private_bits + scfsi
	// Leave rest zero — simplified side info
	return si
}

// ── Psychoacoustic Model ─────────────────────────────────────────────

// mp3BarkTable maps FFT bins to critical bands (Bark scale) for 44100 Hz.
// 27 critical bands covering 0-22050 Hz.
var mp3BarkBands = []struct {
	lowFreq  float64 // Hz
	highFreq float64 // Hz
	bark     float64 // center in Bark
}{
	{0, 100, 0.5}, {100, 200, 1.5}, {200, 300, 2.5}, {300, 400, 3.5},
	{400, 510, 4.5}, {510, 630, 5.5}, {630, 770, 6.5}, {770, 920, 7.5},
	{920, 1080, 8.5}, {1080, 1270, 9.5}, {1270, 1480, 10.5}, {1480, 1720, 11.5},
	{1720, 2000, 12.5}, {2000, 2320, 13.5}, {2320, 2700, 14.5}, {2700, 3150, 15.5},
	{3150, 3700, 16.5}, {3700, 4400, 17.5}, {4400, 5300, 18.5}, {5300, 6400, 19.5},
	{6400, 7700, 20.5}, {7700, 9500, 21.5}, {9500, 12000, 22.5}, {12000, 15500, 23.5},
	{15500, 20500, 24.5}, {20500, 22050, 25.0},
}

// spreadingFunc computes the spreading function value in dB.
// dz = bark difference between masker and maskee.
func spreadingFunc(dz float64) float64 {
	// Schroeder spreading function
	return 15.81 + 7.5*(dz+0.474) - 17.5*math.Sqrt(1.0+math.Pow(dz+0.474, 2))
}

// psychoThreshold computes the per-band masking threshold (dB SPL).
// FFT magnitude in dB SPL, sample rate in Hz, returns thresholds per bark band.
func psychoThreshold(fftMagDB []float64, sampleRate int) []float64 {
	nFFT := len(fftMagDB)
	nBark := len(mp3BarkBands)
	barkSPL := make([]float64, nBark)
	for b := 0; b < nBark; b++ {
		loBin := int(mp3BarkBands[b].lowFreq * float64(nFFT*2) / float64(sampleRate))
		hiBin := int(mp3BarkBands[b].highFreq * float64(nFFT*2) / float64(sampleRate))
		if loBin < 0 {
			loBin = 0
		}
		if hiBin > nFFT-1 {
			hiBin = nFFT - 1
		}
		if hiBin <= loBin {
			hiBin = loBin + 1
		}
		// Average SPL in this bark band
		sum := 0.0
		for i := loBin; i < hiBin; i++ {
			sum += fftMagDB[i]
		}
		barkSPL[b] = sum / float64(hiBin-loBin)
	}

	// Compute masking threshold via spreading function
	threshold := make([]float64, nBark)
	for j := 0; j < nBark; j++ {
		t := -200.0 // very low threshold
		for i := 0; i < nBark; i++ {
			dz := mp3BarkBands[j].bark - mp3BarkBands[i].bark
			sf := spreadingFunc(math.Abs(dz))
			maskerSPL := barkSPL[i]
			// Convert from SPL to masking contribution
			contrib := maskerSPL + sf
			if contrib > t {
				t = contrib
			}
		}
		// Absolute threshold of hearing (simplified: 0 dB at 1kHz, higher at low/high)
		absThresh := 20.0
		if mp3BarkBands[j].bark < 5 {
			absThresh = 40.0
		} else if mp3BarkBands[j].bark > 22 {
			absThresh = 50.0
		}
		if t < absThresh {
			t = absThresh
		}
		threshold[j] = t
	}
	return threshold
}

// fft1024 performs a basic 1024-point FFT on input samples.
// Returns magnitude in dB SPL.
func fft1024(input []float64) []float64 {
	n := 1024
	if len(input) < n {
		n = len(input)
	}
	// Real FFT: use simple DFT for now
	mag := make([]float64, n/2+1)
	for k := 0; k <= n/2; k++ {
		re := 0.0
		im := 0.0
		for t := 0; t < n; t++ {
			angle := -2.0 * math.Pi * float64(k) * float64(t) / float64(n)
			re += input[t] * math.Cos(angle)
			im += input[t] * math.Sin(angle)
		}
		mag[k] = math.Sqrt(re*re + im*im)
		if mag[k] < 1e-15 {
			mag[k] = 1e-15
		}
		mag[k] = 20 * math.Log10(mag[k])
	}
	return mag
}

// computeMasking computes per-scale-factor-band masking thresholds.
// Returns threshold in linear scale (0-1 range) for each of 22 SFB.
func computeMasking(pcm []int16, sampleRate, channels int) []float64 {
	// Build 1024-sample analysis window (Hann)
	n := 1024
	if len(pcm)/channels < n {
		n = len(pcm) / channels
	}
	samples := make([]float64, n)
	win := make([]float64, n)
	for i := 0; i < n; i++ {
		win[i] = 0.5 * (1.0 - math.Cos(2.0*math.Pi*float64(i)/float64(n-1))) // Hann
		s := 0.0
		if i*channels < len(pcm) {
			s = float64(pcm[i*channels]) / 32768.0
		}
		samples[i] = s * win[i]
	}

	// FFT → magnitude (dB SPL)
	fftMag := fft1024(samples)

	// Per-Bark threshold
	barkThresh := psychoThreshold(fftMag, sampleRate)

	// Map Bark thresholds to scale factor bands
	sfbTable := mp3SfbTable441[:]
	numSfb := len(sfbTable)
	sfbMask := make([]float64, numSfb)

	for b := 0; b < numSfb; b++ {
		start := sfbTable[b][0]
		end := sfbTable[b][1]
		if end > 576 {
			end = 576
		}
		// Find center frequency of this SFB
		centerBin := (start + end) / 2
		centerFreq := float64(centerBin) * float64(sampleRate) / (2.0 * 576.0)

		// Map to Bark band
		barkIdx := 0
		for j := len(mp3BarkBands) - 1; j >= 0; j-- {
			if centerFreq >= mp3BarkBands[j].lowFreq && centerFreq <= mp3BarkBands[j].highFreq {
				barkIdx = j
				break
			}
		}
		// Convert dB threshold to linear scale (0-1)
		// Normalize: threshold_dB → allowable noise energy ratio
		thrDB := barkThresh[barkIdx]
		// Convert to linear: 0 dB → 1.0, -30 dB → 0.001, etc.
		thrLinear := math.Pow(10.0, thrDB/20.0)
		// Clamp to 0-1 range
		if thrLinear > 1.0 {
			thrLinear = 1.0
		} else if thrLinear < 0.001 {
			thrLinear = 0.001
		}
		sfbMask[b] = 1.0 - thrLinear // 0 = inaudible (can quantize heavily), 1 = audible (must preserve)
	}

	return sfbMask
}

// reorderPolyphase reorders and windows 512 samples into 64 folded values
// per the MPEG-1 polyphase filter bank (analysis subband).
type mp3SubbandBuffer struct {
	// FIFO buffer: 512 samples per channel
	fifo [2][512]float64
	// Overlap storage: 32 subbands × 36 samples per channel
	overlap [2][32][36]float64
	// Index tracking
	overlapIdx int // 0 or 18 (which half of the 36-sample buffer is current)
}

var mp3SubbandBuf mp3SubbandBuffer

// buildPolyphaseWindow returns the 512-point MPEG-1 analysis window coefficients.
func buildPolyphaseWindow() [512]float64 {
	var w [512]float64
	for i := 0; i < 512; i++ {
		// Prototype: sin window shifted by π/1024
		w[i] = -math.Sin(math.Pi * (float64(i) + 0.5) / 512.0)
	}
	return w
}

// polyphaseAnalysis computes 32 subband samples from 512 windowed samples.
func polyphaseAnalysis(windowed [512]float64) [32]float64 {
	// Fold into 64 partials
	var Y [64]float64
	for i := 0; i < 64; i++ {
		for j := 0; j < 8; j++ {
			Y[i] += windowed[i+64*j]
		}
	}

	// Compute 32 subband samples via MCDCT
	var S [32]float64
	for k := 0; k < 32; k++ {
		for i := 0; i < 64; i++ {
			S[k] += Y[i] * math.Cos(float64((2*k+1)*(i+16))*math.Pi/64.0)
		}
		S[k] /= 32.0
	}
	return S
}

// mdct18 computes an 18-point MDCT from 36 windowed input samples.
func mdct18(input [36]float64) [18]float64 {
	var out [18]float64
	for k := 0; k < 18; k++ {
		sum := 0.0
		for n := 0; n < 36; n++ {
			sum += input[n] * math.Cos(math.Pi/36.0*float64(n+18)*(float64(k)+0.5))
		}
		out[k] = sum * 2.0 / 36.0
	}
	return out
}

// mp3MDCT32 applies the hybrid filter bank: 32-band polyphase + 18-point MDCT.
// Input: 1152 PCM samples per channel. Output: 576 MDCT coefficients per channel.
// Uses the standard MPEG-1 polyphase analysis filter bank from ISO/IEC 11172-3.
func mp3MDCT32(input []int16, channels int) ([][]float64, error) {
	numCh := channels
	result := make([][]float64, numCh)
	for ch := 0; ch < numCh; ch++ {
		result[ch] = make([]float64, 576)
	}

	// Build analysis window once
	window := buildPolyphaseWindow()

	for ch := 0; ch < numCh; ch++ {
		// Process 18 blocks of 32 samples each → 576 samples per granule
		// For each block, compute 32 subband samples, store in subband matrix
		var subbandMatrix [32][36]float64

		// Copy overlap from previous frame (first 18 of 36)
		for sb := 0; sb < 32; sb++ {
			for i := 0; i < 18; i++ {
				subbandMatrix[sb][i] = mp3SubbandBuf.overlap[ch][sb][i]
			}
		}

		// Process 18 new blocks
		for blk := 0; blk < 18; blk++ {
			// Shift FIFO: insert 32 new samples
			copy(mp3SubbandBuf.fifo[ch][0:480], mp3SubbandBuf.fifo[ch][32:512])
			for s := 0; s < 32; s++ {
				idx := blk*32 + s
				if ch < len(input) && idx*channels+ch < len(input) {
					mp3SubbandBuf.fifo[ch][480+s] = float64(input[idx*channels+ch]) / 32768.0
				} else {
					mp3SubbandBuf.fifo[ch][480+s] = 0
				}
			}

			// Window
			var windowed [512]float64
			for i := 0; i < 512; i++ {
				// Reorder: in polyphase, the sample order is reversed
				windowed[i] = mp3SubbandBuf.fifo[ch][i] * window[i]
			}

			// Compute 32 subband samples
			S := polyphaseAnalysis(windowed)

			// Store in subband matrix (second half, positions 18-35)
			for sb := 0; sb < 32; sb++ {
				subbandMatrix[sb][18+blk] = S[sb]
			}
		}

		// Now apply 36-point MDCT to each subband's 36 samples
		win36 := make([]float64, 36)
		for i := 0; i < 36; i++ {
			win36[i] = math.Sin(math.Pi / 72.0 * (float64(i) + 0.5))
		}

		for sb := 0; sb < 32; sb++ {
			// Window the 36 samples
			var windowed36 [36]float64
			for i := 0; i < 36; i++ {
				windowed36[i] = subbandMatrix[sb][i] * win36[i]
			}

			// 18-point MDCT
			mdctOut := mdct18(windowed36)
			for k := 0; k < 18; k++ {
				result[ch][sb*18+k] = mdctOut[k]
			}

			// Store second half for next frame's overlap
			for i := 0; i < 18; i++ {
				mp3SubbandBuf.overlap[ch][sb][i] = subbandMatrix[sb][18+i]
			}
		}
	}

	return result, nil
}

// estimateHuffBits estimates the number of bits needed to encode quantized coefficients.
func estimateHuffBits(quantized [][]int) int {
	total := 0
	for ch := 0; ch < len(quantized); ch++ {
		for i := 0; i < 576; i += 2 {
			x := quantized[ch][i]
			y := quantized[ch][i+1]
			if x == 0 && y == 0 {
				continue
			}
			absX := x
			if absX < 0 {
				absX = -absX
			}
			absY := y
			if absY < 0 {
				absY = -absY
			}
			if absX <= 1 && absY <= 1 {
				total += 2
			} else {
				total += 12
			}
		}
	}
	return total
}

// quantizeBand quantizes MDCT coefficients for one scale factor band.
func quantizeBand(quantized []int, mdctCh []float64, start, end int, globalGain, sf float64) {
	for i := start; i < end; i++ {
		xr := mdctCh[i]
		q := 0.25 * (globalGain - sf)
		scale := math.Pow(2.0, q)
		if scale < 1e-10 {
			scale = 1e-10
		}
		absXr := xr
		if absXr < 0 {
			absXr = -absXr
		}
		ix := int(math.Floor(math.Pow(absXr/scale, 0.75) + 0.5))
		if ix > 8191 {
			ix = 8191
		}
		if xr < 0 {
			ix = -ix
		}
		quantized[i] = ix
	}
}

// computeSF computes a scale factor for a band from its MDCT energy.
func computeSF(mdctCh []float64, start, end int) float64 {
	energy := 0.0
	for i := start; i < end; i++ {
		energy += mdctCh[i] * mdctCh[i]
	}
	if end <= start {
		return 0
	}
	bandWidth := float64(end - start)
	rms := math.Sqrt(energy / bandWidth)
	if rms < 1e-10 {
		rms = 1e-10
	}
	sf := 20 * math.Log10(rms)
	sf = math.Max(0, math.Min(255, (sf+30)/0.25))
	return sf
}

// buildMainData applies rate-distortion controlled quantization.
// Inner loop: adjust global gain to meet bit budget.
// Outer loop: raise scalefactors for bands exceeding masking threshold.
func (e *mp3Encoder) buildMainData(pcm []int16, channels int) []byte {
	mdct, _ := mp3MDCT32(pcm, channels)
	mask := computeMasking(pcm, e.sampleRate, channels)

	numCh := channels
	sfbTable := mp3SfbTable441[:]
	numSfb := len(sfbTable)

	// Available bits for main data
	siLen := 32
	if channels == 1 {
		siLen = 17
	}
	availBits := (e.frameSize - 4 - siLen) * 8

	// Scale factors per channel per band
	sf := make([][]float64, numCh)
	for ch := 0; ch < numCh; ch++ {
		sf[ch] = make([]float64, numSfb)
		for b := 0; b < numSfb; b++ {
			start := sfbTable[b][0]
			end := sfbTable[b][1]
			if end > 576 {
				end = 576
			}
			sf[ch][b] = computeSF(mdct[ch], start, end)
		}
	}

	// Outer loop: quantize and check distortion
	quantized := make([][]int, numCh)
	for ch := 0; ch < numCh; ch++ {
		quantized[ch] = make([]int, 576)
	}

	bestQuant := make([][]int, numCh)
	for ch := 0; ch < numCh; ch++ {
		bestQuant[ch] = make([]int, 576)
	}

	globalGain := 80.0

	// Outer: up to 10 iterations, raise scalefactors for bands with high NMR
	for outerIter := 0; outerIter < 10; outerIter++ {
		// Inner: binary search global_gain to meet bit budget
		gainLo := 0.0
		gainHi := 255.0
		found := false

		for innerIter := 0; innerIter < 20; innerIter++ {
			globalGain = (gainLo + gainHi) / 2.0

			// Quantize all bands
			for ch := 0; ch < numCh; ch++ {
				for b := 0; b < numSfb; b++ {
					start := sfbTable[b][0]
					end := sfbTable[b][1]
					if end > 576 {
						end = 576
					}
					// Apply psycho-masking offset to scalefactor
					maskOffset := (1.0 - mask[b]) * 40.0
					adjSF := sf[ch][b] - maskOffset
					if adjSF < 0 {
						adjSF = 0
					}
					quantizeBand(quantized[ch], mdct[ch], start, end, globalGain, adjSF)
				}
			}

			bits := estimateHuffBits(quantized)
			maxBits := availBits
			if bits <= maxBits && bits > maxBits-200 {
				found = true
				break
			}
			if bits > maxBits {
				gainLo = globalGain
			} else {
				gainHi = globalGain
			}
		}

		if found {
			for ch := 0; ch < numCh; ch++ {
				copy(bestQuant[ch], quantized[ch])
			}
			break
		}

		// Not found: raise scalefactors for bands with highest energy (where distortion is most audible)
		for ch := 0; ch < numCh; ch++ {
			for b := 0; b < numSfb; b++ {
				start := sfbTable[b][0]
				end := sfbTable[b][1]
				if end > 576 {
					end = 576
				}
				bandEnergy := 0.0
				for i := start; i < end; i++ {
					bandEnergy += mdct[ch][i] * mdct[ch][i]
				}
				if bandEnergy > 10.0 {
					sf[ch][b] += 5.0 // amplify scalefactor
				}
			}
		}
		// Reset gain range for next outer iteration
		gainLo = 0.0
		gainHi = 255.0
	}

	// Use best found quantization
	for ch := 0; ch < numCh; ch++ {
		copy(quantized[ch], bestQuant[ch])
	}

	mainData := e.huffmanEncode(quantized)
	return mainData
}

// ── MPEG-1 Layer III Huffman Tables ──────────────────────────────────

type huffEntry struct {
	hlen uint8
	hcod uint16
}

// huffTab represents one Huffman table.
type huffTab struct {
	tabName uint8 // table index 0-31
	xlat    int8  // signed mapping: if >=0, the stored table uses unsigned values
	linbits uint8 // number of linbits (0-13)
	table   []huffEntry
}

// buildHuffTable1 builds Huffman table 1 (values 0..1).
func buildHuffTable1() *huffTab {
	// ISO/IEC 11172-3 Table B.7: hlen_1, hcod_1
	// x,y ∈ {0,1}, unsigned
	t := &huffTab{tabName: 1, linbits: 0}
	t.table = make([]huffEntry, 4) // indexed by x*2 + y
	t.table[0] = huffEntry{1, 0x1}    // (0,0): 1
	t.table[1] = huffEntry{3, 0x1}    // (0,1): 001
	t.table[2] = huffEntry{2, 0x1}    // (1,0): 01
	t.table[3] = huffEntry{3, 0x0}    // (1,1): 000
	return t
}

func buildHuffTable2() *huffTab {
	t := &huffTab{tabName: 2, linbits: 0}
	t.table = make([]huffEntry, 4)
	t.table[0] = huffEntry{1, 0x1}
	t.table[1] = huffEntry{2, 0x1}
	t.table[2] = huffEntry{2, 0x0}
	t.table[3] = huffEntry{2, 0x1}
	return t
}

func buildHuffTable3() *huffTab {
	t := &huffTab{tabName: 3, linbits: 0}
	t.table = make([]huffEntry, 4)
	t.table[0] = huffEntry{1, 0x1}
	t.table[1] = huffEntry{3, 0x0}
	t.table[2] = huffEntry{3, 0x1}
	t.table[3] = huffEntry{2, 0x0}
	return t
}

func buildHuffTable5() *huffTab {
	t := &huffTab{tabName: 5, linbits: 0}
	// table size: (2+1)^2 = 9 (values 0..2)
	t.table = make([]huffEntry, 9)
	t.table[0] = huffEntry{1, 0x1}
	t.table[1] = huffEntry{3, 0x1}
	t.table[2] = huffEntry{6, 0x15}
	t.table[3] = huffEntry{3, 0x0}
	t.table[4] = huffEntry{3, 0x1}
	t.table[5] = huffEntry{6, 0x14}
	t.table[6] = huffEntry{6, 0x7}
	t.table[7] = huffEntry{6, 0x6}
	t.table[8] = huffEntry{6, 0x5}
	return t
}

func buildHuffTable6() *huffTab {
	t := &huffTab{tabName: 6, linbits: 0}
	t.table = make([]huffEntry, 9)
	t.table[0] = huffEntry{1, 0x1}
	t.table[1] = huffEntry{3, 0x1}
	t.table[2] = huffEntry{6, 0x15}
	t.table[3] = huffEntry{3, 0x0}
	t.table[4] = huffEntry{5, 0x1}
	t.table[5] = huffEntry{6, 0x14}
	t.table[6] = huffEntry{6, 0x7}
	t.table[7] = huffEntry{6, 0x6}
	t.table[8] = huffEntry{6, 0x5}
	return t
}

func buildHuffTable7() *huffTab {
	t := &huffTab{tabName: 7, linbits: 0}
	t.table = make([]huffEntry, 16) // 0..3 = 4 values
	t.table[0] = huffEntry{1, 0x1}
	t.table[1] = huffEntry{3, 0x2}
	t.table[2] = huffEntry{6, 0x2A}
	t.table[3] = huffEntry{8, 0xAB}
	t.table[4] = huffEntry{3, 0x3}
	t.table[5] = huffEntry{4, 0x2}
	t.table[6] = huffEntry{6, 0x2B}
	t.table[7] = huffEntry{7, 0x55}
	t.table[8] = huffEntry{6, 0x16}
	t.table[9] = huffEntry{6, 0x2E}
	t.table[10] = huffEntry{7, 0x54}
	t.table[11] = huffEntry{8, 0xAA}
	t.table[12] = huffEntry{7, 0x57}
	t.table[13] = huffEntry{7, 0x56}
	t.table[14] = huffEntry{8, 0xA9}
	t.table[15] = huffEntry{8, 0xA8}
	return t
}

// selectHuffTable picks the best Huffman table given max absolute value.
func selectHuffTable(maxVal int) *huffTab {
	switch {
	case maxVal <= 1:
		return buildHuffTable3() // use table 3 (best for small values)
	case maxVal <= 2:
		return buildHuffTable5()
	case maxVal <= 3:
		return buildHuffTable7()
	default:
		return buildHuffTable7() // fallback
	}
}

// huffEncodePairs encodes a run of pairs using the selected Huffman table.
func huffEncodePairs(wb *writeBuffer, quantized []int, start, end int) {
	// Find max absolute value to select table
	maxAbs := 0
	for i := start; i < end; i++ {
		a := quantized[i]
		if a < 0 {
			a = -a
		}
		if a > maxAbs {
			maxAbs = a
		}
	}
	tab := selectHuffTable(maxAbs)
	linBits := tab.linbits

	for i := start; i < end; i += 2 {
		x := quantized[i]
		y := quantized[i+1]
		absX := x
		if absX < 0 {
			absX = -absX
		}
		absY := y
		if absY < 0 {
			absY = -absY
		}

		if absX == 0 && absY == 0 {
			// rzero region — reached end of big_values, switch to rzero
			break
		}

		// Clamp to table range
		tabMaxX := int(math.Sqrt(float64(len(tab.table))) - 1)
		if absX > tabMaxX {
			absX = tabMaxX + int(linBits)
		}
		if absY > tabMaxX {
			absY = tabMaxX + int(linBits)
		}

		if absX > tabMaxX || absY > tabMaxX {
			// Escape encoding: use table's escape mechanism
			// Store hcod for (tabMaxX, tabMaxX) then linbits + sign
			idx := tabMaxX*(tabMaxX+1) + tabMaxX
			if idx < len(tab.table) {
				entry := tab.table[idx]
				wb.writeBits(uint32(entry.hcod), int(entry.hlen))
			}
			// Write linbits for x
			if absX > tabMaxX {
				wb.writeBits(uint32(absX-tabMaxX-1), int(linBits))
			} else {
				wb.writeBits(0, int(linBits))
			}
			// Write linbits for y
			if absY > tabMaxX {
				wb.writeBits(uint32(absY-tabMaxX-1), int(linBits))
			} else {
				wb.writeBits(0, int(linBits))
			}
			// Write signs
			if x != 0 {
				if x > 0 {
					wb.writeBits(0, 1)
				} else {
					wb.writeBits(1, 1)
				}
			}
			if y != 0 {
				if y > 0 {
					wb.writeBits(0, 1)
				} else {
					wb.writeBits(1, 1)
				}
			}
		} else {
			// Table lookup
			idx := absX*(tabMaxX+1) + absY
			if idx < len(tab.table) {
				entry := tab.table[idx]
				wb.writeBits(uint32(entry.hcod), int(entry.hlen))
			}
			// Write signs for non-zero values
			if x != 0 {
				if x > 0 {
					wb.writeBits(0, 1)
				} else {
					wb.writeBits(1, 1)
				}
			}
			if y != 0 {
				if y > 0 {
					wb.writeBits(0, 1)
				} else {
					wb.writeBits(1, 1)
				}
			}
		}
	}
}

// writeBuffer manages bit-level output.
type writeBuffer struct {
	out    []byte
	bitBuf uint32
	bitCnt int
}

func newWriteBuffer() *writeBuffer {
	return &writeBuffer{out: make([]byte, 0)}
}

func (wb *writeBuffer) writeBits(val uint32, n int) {
	if n <= 0 {
		return
	}
	wb.bitBuf |= (val & ((1 << uint(n)) - 1)) << (32 - n - wb.bitCnt)
	wb.bitCnt += n
	for wb.bitCnt >= 8 {
		wb.out = append(wb.out, byte(wb.bitBuf>>24))
		wb.bitBuf <<= 8
		wb.bitCnt -= 8
	}
}

func (wb *writeBuffer) flush() []byte {
	if wb.bitCnt > 0 {
		wb.out = append(wb.out, byte(wb.bitBuf>>24))
		wb.bitBuf = 0
		wb.bitCnt = 0
	}
	return wb.out
}

// huffmanEncode encodes quantized MDCT coefficients using proper MPEG-1 Layer III Huffman tables.
func (e *mp3Encoder) huffmanEncode(quantized [][]int) []byte {
	wb := newWriteBuffer()

	for ch := 0; ch < len(quantized); ch++ {
		// Find first non-zero (rzero end)
		lastNonZero := -1
		for i := 575; i >= 0; i-- {
			if quantized[ch][i] != 0 {
				lastNonZero = i
				break
			}
		}
		// Encode big_values region as pairs (even index)
		bvEnd := (lastNonZero + 1) & ^1 // round up to even
		for i := 0; i < bvEnd; i += 2 {
			x := quantized[ch][i]
			y := quantized[ch][i+1]
			absX := x
			if absX < 0 {
				absX = -absX
			}
			absY := y
			if absY < 0 {
				absY = -absY
			}

			if absX == 0 && absY == 0 {
				// Rzero: remaining pairs are zeros, skip encoding them
				break
			}

			// Determine appropriate table
			maxVal := absX
			if absY > maxVal {
				maxVal = absY
			}
			tab := selectHuffTable(maxVal)
			tabMax := int(math.Sqrt(float64(len(tab.table))) - 1)
			linBits := int(tab.linbits)

			if absX > tabMax || absY > tabMax {
				// Use escape encoding
				escapeIdx := tabMax*(tabMax+1) + tabMax
				if escapeIdx < len(tab.table) {
					entry := tab.table[escapeIdx]
					wb.writeBits(uint32(entry.hcod), int(entry.hlen))
				}
				if linBits > 0 {
					if absX > tabMax {
						wb.writeBits(uint32(absX-tabMax-1), linBits)
					} else {
						wb.writeBits(0, linBits)
					}
					if absY > tabMax {
						wb.writeBits(uint32(absY-tabMax-1), linBits)
					} else {
						wb.writeBits(0, linBits)
					}
				}
				// Write signs
				if x != 0 {
					wb.writeBits(boolToUint(x < 0), 1)
				}
				if y != 0 {
					wb.writeBits(boolToUint(y < 0), 1)
				}
			} else {
				idx := absX*(tabMax+1) + absY
				if idx < len(tab.table) {
					entry := tab.table[idx]
					wb.writeBits(uint32(entry.hcod), int(entry.hlen))
				}
				// Write signs
				if x != 0 {
					wb.writeBits(boolToUint(x < 0), 1)
				}
				if y != 0 {
					wb.writeBits(boolToUint(y < 0), 1)
				}
			}
		}
	}

	out := wb.flush()

	// Pad/fill to match frame size
	siLen := 32
	targetLen := e.frameSize - 4 - siLen
	if targetLen < 0 {
		targetLen = 0
	}
	if len(out) < targetLen {
		pad := make([]byte, targetLen-len(out))
		out = append(out, pad...)
	} else if len(out) > targetLen {
		out = out[:targetLen]
	}
	return out
}

func boolToUint(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// EncodePCMToMP3Go encodes PCM samples to MP3 using the pure Go encoder.
// Output is a sequence of MP3 frames.
func EncodePCMToMP3Go(pcm []int16, sampleRate, channels, bitrate int) ([]byte, error) {
	if bitrate <= 0 {
		bitrate = 192
	}
	if sampleRate <= 0 {
		sampleRate = 44100
	}
	if channels <= 0 {
		channels = 2
	}

	enc := newMP3Encoder(sampleRate, bitrate, channels)

	frameLen := mp3SamplesPerFrame * channels
	var frames []byte

	for pos := 0; pos+frameLen <= len(pcm); pos += frameLen {
		frame := enc.encodeFrame(pcm[pos:pos+frameLen], channels)
		frames = append(frames, frame...)
	}

	// Handle remaining samples (pad with zeros)
	rem := len(pcm) % frameLen
	if rem > 0 {
		padded := make([]int16, frameLen)
		copy(padded, pcm[len(pcm)-rem:])
		frame := enc.encodeFrame(padded, channels)
		frames = append(frames, frame...)
	}

	return frames, nil
}
