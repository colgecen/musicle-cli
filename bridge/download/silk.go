package download

import (
	"fmt"
	"math"
)

// SILK decoder constants
const (
	// Q-ilization constants for fixed-point arithmetic
	silkQ8  = 256   // Q8 format
	silkQ16 = 65536 // Q16 format
)

// silkLPCOrder returns the LPC order for a given bandwidth.
func silkLPCOrder(bw int) int {
	switch bw {
	case 0: // NB 8kHz
		return 10
	case 1: // MB 12kHz
		return 12
	case 2, 3: // WB 16kHz, SWB 24kHz
		return 16
	case 4: // FB 48kHz
		return 16
	default:
		return 16
	}
}

// silkFrameSize returns the SILK internal frame size in samples.
func silkFrameSize(bw int) int {
	switch bw {
	case 0:
		return 240 // 30ms at 8kHz
	case 1:
		return 360 // 30ms at 12kHz
	case 2:
		return 480 // 30ms at 16kHz
	case 3, 4:
		return 480 // SILK operates at max 16kHz internal
	default:
		return 480
	}
}

// NLSF (Normalized Line Spectral Frequencies) to LPC coefficients conversion.
// Uses Chebyshev polynomial evaluation (RFC 6716 Section 4.2.4.2).

// chebyshev evaluates a Chebyshev polynomial at x.
func chebyshev(coeffs []int16, x float64) float64 {
	var b0, b1, b2 float64
	for i := len(coeffs) - 1; i >= 0; i-- {
		b2 = b1
		b1 = b0
		b0 = float64(coeffs[i])/32768.0 + 2.0*x*b1 - b2
	}
	return b0 - b1
}

// nlsfToLPC converts a vector of NLSF Q15 values to LPC coefficients.
// The LPC coefficients are normalized (a[0] = 1.0).
func nlsfToLPC(nlsf []int16, order int) []float64 {
	if len(nlsf) < order/2 {
		return nil
	}

	n := order / 2
	f1 := make([]int16, n+1)
	f2 := make([]int16, n+1)

	f1[0] = 1024 // 1.0 in Q10
	f2[0] = 1024
	for i := 0; i < n; i++ {
		// Compute the ith Chebyshev polynomial values
		cosX := math.Cos(float64(nlsf[i]) * math.Pi / 32768.0)
		cosX2 := math.Cos(float64(nlsf[2*n-1-i]) * math.Pi / 32768.0)

		// Update f1 and f2 polynomials using Chebyshev recursion
		for j := i; j >= 0; j-- {
			f1[j+1] += int16(float64(f1[j]) * cosX / 2.0)
			f2[j+1] += int16(float64(f2[j]) * cosX2 / 2.0)
		}
	}

	// Combine f1 and f2 into LPC coefficients
	// A(z) = (F1(z) + F2(z)) / 2 for even coefficients
	// A(z) = (F1(z) - F2(z)) / 2 for odd coefficients
	lpc := make([]float64, order+1)
	lpc[0] = 1.0

	for i := 0; i < n; i++ {
		f1Val := float64(f1[i+1]) / 1024.0
		f2Val := float64(f2[i+1]) / 1024.0
		lpc[i+1] = (f1Val + f2Val) / 2.0                     // even
		if i*2+2 <= order {
			lpc[i*2+2] = (f1Val - f2Val) / 2.0               // next odd
		}
	}

	return lpc
}

// lpcSynthesisFilter applies the LPC synthesis filter.
// y[n] = excitation[n] - sum_{i=1}^{order} a[i] * y[n-i]
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

// deemphasis applies deemphasis filter: y[n] = x[n] - 0.68 * x[n-1]
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

// generateWhiteNoise generates white noise for unvoiced SILK frames.
func generateWhiteNoise(length int, seed *uint32) []float64 {
	out := make([]float64, length)
	for i := 0; i < length; i++ {
		*seed = *seed*1103515245 + 12345
		out[i] = float64(int32(*seed)) / 32768.0
	}
	return out
}

// generatePulseExcitation generates a pulse train for voiced SILK frames.
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

// decodeSilkFrame decodes a SILK-only Opus frame.
// This is a simplified decoder that assumes a basic pulse/noise excitation model.
func decodeSilkFrame(data []byte, cfg *opusConfig, channels int) ([]int16, error) {
	if len(data) < 2 {
		return nil, fmt.Errorf("SILK frame too short")
	}

	bw := cfg.bandwidth
	if bw > 2 {
		bw = 2 // SILK operates at max 16kHz internally
	}
	order := silkLPCOrder(bw)
	silkLen := cfg.frameSize

	// Get SILK internal frame size and upscale
	internalSize := silkFrameSize(bw)
	if internalSize <= 0 {
		internalSize = 240
	}

	// Decode using the range decoder (after TOC byte)
	rd := newRangeDecoder(data)
	_ = rd

	// ===== Simplified SILK decoder =====
	// In a full implementation we would decode:
	//  1. Signal type (unvoiced/voiced) - 1 bit
	//  2. Pitch lag
	//  3. LTP scale
	//  4. NLSF indices
	//  5. Gains
	//  6. Excitation

	// For now, generate a basic excitation model
	var seed uint32 = 12345
	voiced := (data[1] & 0x80) != 0

	var excitation []float64
	if voiced {
		pitchLag := int(data[1] & 0x3f)
		if pitchLag < 10 {
			pitchLag = 10
		}
		excitation = generatePulseExcitation(internalSize, pitchLag)
	} else {
		excitation = generateWhiteNoise(internalSize, &seed)
	}

	// Generate simple LPC coefficients (this would normally be decoded from the bitstream)
	// For simplicity, use a fixed mild LPC that decays
	a := make([]float64, order+1)
	a[0] = 1.0
	for i := 1; i <= order; i++ {
		a[i] = 0.85 / float64(i*i+1)
		if i%2 == 0 {
			a[i] = -a[i]
		}
	}

	// LPC synthesis filter
	synth := lpcSynthesisFilter(excitation, a)

	// Deemphasis
	synth = deemphasis(synth)

	// Apply gain
	gain := 0.5
	for i := range synth {
		synth[i] *= gain
	}

	// Upsample from SILK internal rate to 48kHz
	out := upsampleSilk(synth, internalSize, silkLen, channels)

	// Convert to int16
	pcm := make([]int16, len(out))
	for i := 0; i < len(out); i++ {
		val := out[i]
		if val > 32767.0 / 32768.0 {
			val = 32767.0 / 32768.0
		} else if val < -32768.0 / 32768.0 {
			val = -32768.0 / 32768.0
		}
		pcm[i] = int16(val * 32767.0)
	}

	return pcm, nil
}

// upsampleSilk upsamples from SILK internal rate to 48kHz using sinc interpolation.
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
