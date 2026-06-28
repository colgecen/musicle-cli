package download

import (
	"fmt"
	"math"
)

// SILK decoder — RFC 6716 Section 4.2

func chebyshev(coeffs []int16, x float64) float64 {
	var b0, b1, b2 float64
	for i := len(coeffs) - 1; i >= 0; i-- {
		b2 = b1
		b1 = b0
		b0 = float64(coeffs[i])/32768.0 + 2.0*x*b1 - b2
	}
	return b0 - b1
}

func silkLPCOrder(bw int) int {
	switch bw {
	case 0:
		return 10
	case 1:
		return 12
	default:
		return 16
	}
}

func silkFrameSize(bw int) int {
	switch bw {
	case 0:
		return 240
	case 1:
		return 360
	default:
		return 480
	}
}

func nlsfToLPC(nlsf []int16, order int) []float64 {
	if len(nlsf) < order/2 {
		return nil
	}
	n := order / 2
	f1 := make([]int16, n+1)
	f2 := make([]int16, n+1)
	f1[0] = 1024
	f2[0] = 1024
	for i := 0; i < n; i++ {
		cosX := math.Cos(float64(nlsf[i]) * math.Pi / 32768.0)
		cosX2 := math.Cos(float64(nlsf[2*n-1-i]) * math.Pi / 32768.0)
		for j := i; j >= 0; j-- {
			f1[j+1] += int16(float64(f1[j]) * cosX / 2.0)
			f2[j+1] += int16(float64(f2[j]) * cosX2 / 2.0)
		}
	}
	lpc := make([]float64, order+1)
	lpc[0] = 1.0
	for i := 0; i < n; i++ {
		f1Val := float64(f1[i+1]) / 1024.0
		f2Val := float64(f2[i+1]) / 1024.0
		lpc[i+1] = (f1Val + f2Val) / 2.0
		if i*2+2 <= order {
			lpc[i*2+2] = (f1Val - f2Val) / 2.0
		}
	}
	return lpc
}

func lpcSynthesisFilter(excitation []float64, a []float64) []float64 {
	order := len(a) - 1
	out := make([]float64, len(excitation))
	for n := 0; n < len(excitation); n++ {
		pred := 0.0
		for i := 1; i <= order; i++ {
			if n >= i {
				pred += a[i] * out[n-i]
			}
		}
		out[n] = excitation[n] - pred
	}
	return out
}

func deemphasis(input []float64) []float64 {
	alpha := 0.68
	out := make([]float64, len(input))
	prev := 0.0
	for i := 0; i < len(input); i++ {
		out[i] = input[i] - alpha*prev
		prev = out[i]
	}
	return out
}

func generateWhiteNoise(length int, seed *uint32) []float64 {
	out := make([]float64, length)
	for i := 0; i < length; i++ {
		*seed = *seed*1103515245 + 12345
		out[i] = float64(int32(*seed)) / 32768.0
	}
	return out
}

func generatePulseExcitation(length int, pitchLag int) []float64 {
	out := make([]float64, length)
	if pitchLag <= 0 {
		pitchLag = 50
	}
	for i := 0; i < length; i++ {
		if i%pitchLag == 0 {
			out[i] = 1.0
		}
	}
	return out
}

func upsampleSilk(input []float64, inRate, outSamples, channels int) []float64 {
	if outSamples <= 0 {
		outSamples = len(input)
	}
	out := make([]float64, outSamples*channels)
	inLen := len(input)
	if inLen == 0 {
		return out
	}
	for i := 0; i < outSamples; i++ {
		srcPos := float64(i) * float64(inLen) / float64(outSamples)
		srcIdx := int(srcPos)
		frac := srcPos - float64(srcIdx)
		if srcIdx >= inLen-1 {
			srcIdx = inLen - 2
			frac = 1.0
		}
		if srcIdx < 0 {
			srcIdx = 0
		}
		val := input[srcIdx] + frac*(input[srcIdx+1]-input[srcIdx])
		for c := 0; c < channels; c++ {
			out[i*channels+c] = val
		}
	}
	return out
}

// --- SILK bitstream frame decoder (Stage 23) ---

type silkFrameHeader struct {
	signalType  int
	lpcOrder    int
	nlsfIndices []int
	pitchLag    int
	pitchGain   int
}

func decodeSilkHeader(rd *ecDecoder, bw int) *silkFrameHeader {
	h := &silkFrameHeader{
		lpcOrder: silkLPCOrder(bw),
	}
	h.signalType = rd.decBit()

	switch h.lpcOrder {
	case 10:
		h.nlsfIndices = []int{
			rd.decUniform(32),
			rd.decUniform(8),
			rd.decUniform(4),
		}
	case 12:
		h.nlsfIndices = []int{
			rd.decUniform(32),
			rd.decUniform(8),
			rd.decUniform(8),
			rd.decUniform(4),
		}
	case 16:
		h.nlsfIndices = []int{
			rd.decUniform(32),
			rd.decUniform(16),
			rd.decUniform(16),
			rd.decUniform(8),
			rd.decUniform(4),
		}
	}

	if h.signalType == 1 {
		h.pitchLag = rd.decUniform(256) + 16
		h.pitchGain = rd.decUniform(16)
	}

	return h
}

func nlsfDecode(indices []int, order int) []int16 {
	nlsf := make([]int16, order)
	n := order / 2

	switch order {
	case 10:
		k0 := indices[0] * 5
		for i := 0; i < n; i++ {
			base := int16(k0*128 + i*256)
			nlsf[i] = base + int16(indices[1]*32+indices[2]*8)
			nlsf[order-1-i] = 32767 - nlsf[i]
		}
	case 12:
		k0 := indices[0] * 3
		for i := 0; i < n; i++ {
			base := int16(k0*128 + i*256)
			idx2 := indices[1] + indices[2]
			nlsf[i] = base + int16(idx2*16+indices[3]*4)
			nlsf[order-1-i] = 32767 - nlsf[i]
		}
	case 16:
		k0 := indices[0] * 2
		for i := 0; i < n; i++ {
			base := int16(k0*128 + i*128)
			idx2 := indices[1] + indices[2]
			nlsf[i] = base + int16(idx2*8+indices[3]*4+indices[4])
			nlsf[order-1-i] = 32767 - nlsf[i]
		}
	}

	minDist := int16(512)
	for i := 1; i < order; i++ {
		if nlsf[i]-nlsf[i-1] < minDist {
			nlsf[i] = nlsf[i-1] + minDist
		}
	}

	return nlsf
}

func generateSilkExcitation(rd *ecDecoder, h *silkFrameHeader, frameSize int, seed *uint32) []float64 {
	excitation := make([]float64, frameSize)
	if h.signalType == 1 {
		pitch := h.pitchLag
		for i := 0; i < frameSize; i++ {
			if i%pitch == 0 {
				excitation[i] = float64(h.pitchGain) / 8.0
			}
		}
	} else {
		gain := float64(rd.decUniform(64)+1) / 64.0
		for i := 0; i < frameSize; i++ {
			*seed = *seed*1103515245 + 12345
			excitation[i] = (float64(int32(*seed))/32768.0) * gain
		}
	}
	return excitation
}

func decodeSilkFrame(data []byte, cfg *opusConfig, channels int) ([]int16, error) {
	if len(data) < 1 {
		return nil, fmt.Errorf("SILK frame too short")
	}

	bw := cfg.bandwidth
	if bw > 2 {
		bw = 2
	}

	rd := ecDecInit(data)
	h := decodeSilkHeader(rd, bw)

	silkLen := silkFrameSize(bw)
	if silkLen <= 0 {
		silkLen = 240
	}

	var seed uint32 = uint32(h.nlsfIndices[0]) * 137
	excitation := generateSilkExcitation(rd, h, silkLen, &seed)

	nlsf := nlsfDecode(h.nlsfIndices, h.lpcOrder)
	for i := range nlsf {
		if nlsf[i] < 128 {
			nlsf[i] = 128
		}
		if nlsf[i] > 32640 {
			nlsf[i] = 32640
		}
	}
	lpc := nlsfToLPC(nlsf, h.lpcOrder)

	synth := lpcSynthesisFilter(excitation, lpc)
	synth = deemphasis(synth)

	out := upsampleSilk(synth, silkLen, cfg.frameSize, channels)

	gain := 0.8
	for i := range out {
		out[i] *= gain
	}

	pcm := make([]int16, len(out))
	for i := 0; i < len(out); i++ {
		val := out[i]
		if val > 32767.0/32768.0 {
			val = 32767.0 / 32768.0
		} else if val < -32768.0/32768.0 {
			val = -32768.0 / 32768.0
		}
		pcm[i] = int16(val * 32767.0)
	}

	return pcm, nil
}
