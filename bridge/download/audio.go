package download

import (
	"fmt"
	"io"
)

// EncodePCMToMP3 encodes PCM s16le samples to MP3 using pure Go encoder.
func EncodePCMToMP3(pcm []int16, sampleRate, channels int, bitrate string, cb ProgressCallback) ([]byte, error) {
	if cb != nil {
		cb(0, "Encoding MP3...")
	}

	br := 192
	if bitrate != "" {
		brStr := bitrate
		if len(brStr) > 0 && brStr[len(brStr)-1] == 'k' {
			brStr = brStr[:len(brStr)-1]
		}
		if b, err := fmt.Sscanf(brStr, "%d", &br); err != nil || b != 1 {
			br = 192
		}
	}

	mp3, err := EncodePCMToMP3Go(pcm, sampleRate, channels, br)
	if err != nil {
		return nil, fmt.Errorf("mp3 encode: %w", err)
	}

	if cb != nil {
		cb(100, "Encoded")
	}
	return mp3, nil
}

// WebMToMP3 is the full pipeline: WebM bytes → decode to PCM → encode to MP3.
func WebMToMP3(webmData []byte, bitrate string, artist string, cb ProgressCallback) ([]byte, error) {
	if cb != nil {
		cb(0, "WebM → PCM...")
	}

	res, err := DecodeWebMToPCM(webmData, func(pct int, msg string) {
		if cb != nil {
			cb(pct/2, msg)
		}
	})
	if err != nil {
		return nil, fmt.Errorf("decode webm: %w", err)
	}

	if cb != nil {
		cb(50, "PCM → MP3...")
	}

	mp3, err := EncodePCMToMP3(res.Samples, res.SampleRate, res.Channels, bitrate, func(pct int, msg string) {
		if cb != nil {
			cb(50+pct/2, msg)
		}
	})
	if err != nil {
		return nil, fmt.Errorf("encode mp3: %w", err)
	}

	if cb != nil {
		cb(100, "Done")
	}
	return mp3, nil
}

// GetAudioDurationSec returns duration of audio in seconds from WebM metadata.
func GetAudioDurationSec(input []byte, format string) (float64, error) {
	pr, err := ParseWebM(input)
	if err != nil {
		return 0, fmt.Errorf("parse webm: %w", err)
	}
	if pr.Info == nil {
		return 0, fmt.Errorf("no info in WebM")
	}
	if pr.Info.Duration <= 0 || pr.Info.TimecodeScale <= 0 {
		return 0, fmt.Errorf("no duration in WebM")
	}
	return pr.Info.Duration * float64(pr.Info.TimecodeScale) / 1e9, nil
}

// WriteMP3File writes MP3 data to an io.Writer, returning bytes written.
func WriteMP3File(w io.Writer, mp3Data []byte) (int64, error) {
	n, err := w.Write(mp3Data)
	return int64(n), err
}

// WebMDecodeResult holds the result of decoding a WebM file to PCM.
type WebMDecodeResult struct {
	Samples    []int16
	SampleRate int
	Channels   int
	DurationNs int64
	TrackInfo  *EBMLAudioTrack
}

// DecodeWebMToPCM decodes raw WebM bytes to PCM s16le using pure Go Opus decoder.
func DecodeWebMToPCM(webmData []byte, cb ProgressCallback) (*WebMDecodeResult, error) {
	if cb != nil {
		cb(0, "Parsing WebM...")
	}

	pr, err := ParseWebM(webmData)
	if err != nil {
		return nil, fmt.Errorf("webm parse: %w", err)
	}
	if len(pr.Tracks) == 0 {
		return nil, fmt.Errorf("no audio tracks")
	}
	track := pr.Tracks[0]

	sampleRate := int(track.SampleRate)
	if sampleRate <= 0 {
		sampleRate = 48000
	}
	channels := track.Channels
	if channels <= 0 {
		channels = 2
	}

	var durationNs int64
	if pr.Info != nil && pr.Info.Duration > 0 && pr.Info.TimecodeScale > 0 {
		durationNs = int64(pr.Info.Duration * float64(pr.Info.TimecodeScale))
	}

	if cb != nil {
		cb(20, fmt.Sprintf("WebM: %dch %dHz", channels, sampleRate))
	}

	packets, _, _, err := DecodeWebMOpusPackets(webmData)
	if err != nil {
		return nil, fmt.Errorf("extract packets: %w", err)
	}
	if len(packets) == 0 {
		return nil, fmt.Errorf("no Opus packets")
	}

	if cb != nil {
		cb(30, "Decoding Opus...")
	}

	pcm, err := DecodeOpusToPCM(packets, sampleRate)
	if err != nil {
		return nil, fmt.Errorf("opus decode: %w", err)
	}

	if cb != nil {
		cb(90, "Decoded")
	}
	return &WebMDecodeResult{
		Samples:    pcm,
		SampleRate: sampleRate,
		Channels:   channels,
		DurationNs: durationNs,
		TrackInfo:  &track,
	}, nil
}

// DecodeWebMOpusPackets extracts Opus packets from WebM using pure Go parser.
func DecodeWebMOpusPackets(webmData []byte) ([]OpusPacket, *EBMLInfo, *EBMLAudioTrack, error) {
	frames, info, track, err := ExtractAudioFrames(webmData)
	if err != nil {
		return nil, nil, nil, err
	}
	packets, err := ExtractOpusPackets(frames)
	if err != nil {
		return nil, nil, nil, err
	}
	return packets, info, track, nil
}
