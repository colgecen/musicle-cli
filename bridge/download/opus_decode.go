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


