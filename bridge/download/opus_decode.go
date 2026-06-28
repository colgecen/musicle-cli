package download

import (
	"fmt"
)

const opusMaxChannels = 2

// DecodeOpusToPCM decodes raw Opus packets to PCM s16le samples.
// Returns interleaved samples for stereo.
func DecodeOpusToPCM(packets []OpusPacket, sampleRate int) ([]int16, error) {
	if len(packets) == 0 {
		return nil, fmt.Errorf("no Opus packets")
	}

	var allSamples []int16
	for _, pkt := range packets {
		samples, err := decodeOneFrame(&pkt)
		if err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}
		allSamples = append(allSamples, samples...)
	}

	// Resample if needed
	if sampleRate > 0 && sampleRate != 48000 {
		allSamples = resample(allSamples, 48000, sampleRate, 2)
	}

	return allSamples, nil
}

// resample performs simple linear interpolation.
func resample(input []int16, fromRate, toRate, channels int) []int16 {
	if fromRate == toRate {
		return input
	}
	inputLen := len(input) / channels
	outputLen := inputLen * toRate / fromRate
	output := make([]int16, outputLen*channels)

	for i := 0; i < outputLen; i++ {
		srcPos := float64(i) * float64(fromRate) / float64(toRate)
		srcIdx := int(srcPos)
		frac := srcPos - float64(srcIdx)
		if srcIdx >= inputLen-1 {
			srcIdx = inputLen - 2
			frac = 1.0
		}
		for c := 0; c < channels; c++ {
			a := float64(input[srcIdx*channels+c])
			b := float64(input[(srcIdx+1)*channels+c])
			val := a + frac*(b-a)
			if val > 32767 {
				val = 32767
			} else if val < -32768 {
				val = -32768
			}
			output[i*channels+c] = int16(val)
		}
	}
	return output
}

// decodeOneFrame decodes a single parsed Opus packet to PCM.
func decodeOneFrame(pkt *OpusPacket) ([]int16, error) {
	channels := 1
	if pkt.Stereo {
		channels = 2
	}

	// Frame config for our decoder
	cfg := &opusConfig{
		stereo:        pkt.Stereo,
		bandwidth:     pkt.Bandwidth,
		frameSize:     pkt.FrameSize,
		codec:         pkt.Mode,
		frameDuration: pkt.FrameSize / 48, // in 2.5ms units at 48kHz
	}

	var allSamples []int16
	for _, frameData := range pkt.Frames {
		var samples []int16
		var err error

		switch pkt.Mode {
		case 0: // SILK-only
			samples, err = decodeSilkFrame(frameData, cfg, channels)
		case 1: // CELT-only
			samples, err = decodeCeltFrame(frameData, cfg, channels)
		case 2: // Hybrid
			samples, err = decodeHybridFrame(frameData, cfg, channels)
		default:
			return nil, fmt.Errorf("unknown Opus mode: %d", pkt.Mode)
		}
		if err != nil {
			return nil, err
		}
		allSamples = append(allSamples, samples...)
	}
	return allSamples, nil
}

// opusConfig holds the decoded configuration for a single Opus frame.
type opusConfig struct {
	stereo        bool
	codec         int // 0=SILK, 1=CELT, 2=hybrid
	bandwidth     int // 0=NB 8k...4=FB 48k
	frameDuration int // in 2.5ms units
	frameSize     int // samples per channel
}

// --- Range Decoder (arithmetic decoder, shared by SILK and CELT) ---

type rangeDecoder struct {
	data   []byte
	pos    int
	low    uint32
	range_ uint32
}

func newRangeDecoder(data []byte) *rangeDecoder {
	rd := &rangeDecoder{
		data:   data,
		pos:    0,
		low:    0,
		range_: 0xFFFFFFFF,
	}
	// Read first 8 bytes into low
	for rd.pos < len(data) && rd.pos < 8 {
		rd.low = (rd.low << 8) | uint32(data[rd.pos])
		rd.pos++
	}
	return rd
}

func (rd *rangeDecoder) decSym(cdf []uint16) int {
	if len(cdf) < 2 {
		return 0
	}
	total := int(cdf[len(cdf)-1])
	if total <= 0 {
		return 0
	}
	scale := rd.range_ / uint32(total)
	val := rd.low / scale
	rd.range_ = scale

	for i := 0; i < len(cdf)-1; i++ {
		if val < uint32(cdf[i]) {
			if i > 0 {
				rd.low -= uint32(cdf[i-1]) * scale
			}
			rd.renormalize()
			return i
		}
	}
	rd.low = 0
	rd.range_ = 1
	return len(cdf) - 2
}

func (rd *rangeDecoder) decBit() int {
	return rd.decSym([]uint16{1, 2})
}

func (rd *rangeDecoder) decUniform(n int) int {
	if n <= 1 {
		return 0
	}
	k := 0
	for (1 << k) < n {
		k++
	}
	v := 0
	bits := k - 1
	for i := 0; i < bits; i++ {
		v = (v << 1) | rd.decBit()
	}
	extra := 1 << bits
	if extra < n {
		bit := rd.decBit()
		v = v<<1 | bit
	}
	return v
}

func (rd *rangeDecoder) decInt(n int) int {
	v := 0
	for i := 0; i < n; i++ {
		v = v<<1 | rd.decBit()
	}
	return v
}

func (rd *rangeDecoder) decLaplace(decay uint16) int {
	if rd.decBit() == 0 {
		return 0
	}
	mag := 0
	for rd.decBit() == 1 {
		mag++
	}
	return mag + 1
}

func (rd *rangeDecoder) renormalize() {
	for rd.range_ < 1<<24 {
		rd.range_ <<= 8
		rd.low = (rd.low << 8) & 0xFFFFFFFF
		if rd.pos < len(rd.data) {
			rd.low |= uint32(rd.data[rd.pos])
			rd.pos++
		}
	}
}
