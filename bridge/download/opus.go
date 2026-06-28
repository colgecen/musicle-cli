package download

import (
	"encoding/binary"
	"fmt"
)

// Opus frame modes.
const (
	OpusModeSilk = 0
	OpusModeCelt = 1
	OpusModeHybrid = 2
)

// Opus bandwidths (from config bits 0-31).
const (
	OpusBandwidthNB   = 0 // 4 kHz  (config 0-3)
	OpusBandwidthMB   = 1 // 6 kHz  (config 4-7)
	OpusBandwidthWB   = 2 // 8 kHz  (config 8-11)
	OpusBandwidthSWB  = 3 // 12 kHz (config 12-15)
	OpusBandwidthFB   = 4 // 20 kHz (config 16-19)
	// config 20-23: SILK-only FB
	// config 24-31: CELT-only NB..FB
)

// OpusPacket holds a parsed Opus packet.
type OpusPacket struct {
	Config      int    // 0-31, combined codec/bandwidth
	Mode        int    // 0=SILK, 1=CELT, 2=hybrid
	Stereo      bool
	Bandwidth   int    // 0-4
	FrameSize   int    // number of audio samples per channel
	Frames      [][]byte // raw frame data (1-48 frames per packet)
	TotalBytes  int    // total bytes consumed by this packet (for skipping)
}

// ParseOpusPacket parses a single Opus packet (TOC + optional frame count + optional lengths + data).
func ParseOpusPacket(data []byte) (*OpusPacket, error) {
	if len(data) < 1 {
		return nil, fmt.Errorf("opus packet too short: %d bytes", len(data))
	}

	toc := data[0]
	config := int(toc >> 3)      // bits 7-3
	mode := int(toc >> 1) & 0x3  // bits 2-1
	stereo := toc & 0x1          // bit 0

	pkt := &OpusPacket{
		Config:    config,
		Mode:      mode,
		Stereo:    stereo == 1,
		Bandwidth: config / 4,
	}

	offset := 1 // start after TOC

	switch mode {
	case OpusModeSilk, OpusModeCelt:
		codecNumber := config / 4
		switch codecNumber {
		case 0, 1:
			pkt.Frames = [][]byte{data[offset:]}
			pkt.TotalBytes = len(data)
			goto doneSize
		case 2:
			if len(data) < 3 {
				return nil, fmt.Errorf("packet too short for 2-frame VBR: %d bytes", len(data))
			}
			frameSize0 := int(binary.BigEndian.Uint16(data[offset : offset+2]))
			offset += 2
			if frameSize0 < 0 || offset+frameSize0 > len(data) {
				return nil, fmt.Errorf("invalid frame0 size %d", frameSize0)
			}
			pkt.Frames = make([][]byte, 2)
			pkt.Frames[0] = data[offset : offset+frameSize0]
			offset += frameSize0
			pkt.Frames[1] = data[offset:]
			pkt.TotalBytes = len(data)
			goto doneSize
		case 3:
			if len(data) < 2 {
				return nil, fmt.Errorf("packet too short for multi-frame: %d bytes", len(data))
			}
			frameCount := int(data[offset]) + 1
			offset++
			if frameCount < 2 || frameCount > 48 {
				return nil, fmt.Errorf("invalid frame count %d", frameCount)
			}
			pkt.Frames = make([][]byte, frameCount)
			for i := 0; i < frameCount-1; i++ {
				if offset+2 > len(data) {
					return nil, fmt.Errorf("packet too short for frame lengths")
				}
				frameLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
				offset += 2
				if offset+frameLen > len(data) {
					return nil, fmt.Errorf("frame %d exceeds packet bounds", i)
				}
				pkt.Frames[i] = data[offset : offset+frameLen]
				offset += frameLen
			}
			pkt.Frames[frameCount-1] = data[offset:]
			pkt.TotalBytes = len(data)
			goto doneSize
		}

	case OpusModeHybrid:
		if len(data) < 3 {
			return nil, fmt.Errorf("hybrid packet too short: %d bytes", len(data))
		}
		silkLen := int(data[offset])
		offset++
		if silkLen >= 252 {
			if offset >= len(data) {
				return nil, fmt.Errorf("hybrid packet too short for 2-byte length")
			}
			silkLen = (silkLen-252)*256 + int(data[offset]) + 256
			offset++
		}
		if offset+silkLen > len(data) {
			return nil, fmt.Errorf("hybrid silk frame exceeds packet bounds")
		}
		pkt.Frames = make([][]byte, 2)
		pkt.Frames[0] = data[offset : offset+silkLen]
		offset += silkLen
		pkt.Frames[1] = data[offset:]
		pkt.TotalBytes = len(data)
		goto doneSize
	}

	// Fallback: single frame
	pkt.Frames = [][]byte{data[1:]}
	pkt.TotalBytes = len(data)

doneSize:
	pkt.FrameSize = frameSizeInSamples(config, mode)

	return pkt, nil
}

// frameSizeInSamples returns the number of samples per channel for one frame.
// Opus supports 120, 240, 480, 960, 1920, 2880 sample frames.
func frameSizeInSamples(config, mode int) int {
	// From RFC 6716 Table 1:
	// config 0-3:   120 samples (2.5ms at 48kHz)
	// config 4-7:   240 samples (5ms)
	// config 8-11:  480 samples (10ms)
	// config 12-15: 960 samples (20ms)
	// config 16-19: 1920 samples (40ms)
	// config 20-23: 960 samples (SILK-only) - actually frame size determined differently
	// config 24-31: 120-960 samples (CELT-only)
	sizes := []int{120, 240, 480, 960, 1920, 2880}
	idx := config / 4
	if idx >= len(sizes) {
		// Config 24-31 are CELT-only, frame size = 120 * 2^(config-24)
		if config >= 24 {
			return 120 << uint(config-24)
		}
		return 960
	}
	return sizes[idx]
}

// ExtractOpusPackets collects all raw Opus packets from a set of audio frames.
func ExtractOpusPackets(frames []AudioFrame) ([]OpusPacket, error) {
	var packets []OpusPacket
	for _, f := range frames {
		data := f.Data
		for len(data) > 0 {
			pkt, err := ParseOpusPacket(data)
			if err != nil {
				return packets, fmt.Errorf("parse opus at offset %d: %w", len(f.Data)-len(data), err)
			}
			consumed := pkt.TotalBytes
			if consumed <= 0 || consumed > len(data) {
				consumed = len(data)
			}
			packets = append(packets, *pkt)
			data = data[consumed:]
		}
	}
	return packets, nil
}

// OpusSampleRate returns the output sample rate for the given bandwidth index.
func OpusSampleRate(bandwidth int) int {
	switch bandwidth {
	case OpusBandwidthNB:
		return 8000
	case OpusBandwidthMB:
		return 12000
	case OpusBandwidthWB:
		return 16000
	case OpusBandwidthSWB:
		return 24000
	case OpusBandwidthFB:
		return 48000
	default:
		return 48000
	}
}
