package download

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os/exec"
)

// DecodeAudioToPCM decodes raw WebM/Opus or M4A/AAC bytes to PCM s16le samples
// using ffmpeg. Returns samples, sample rate, and error.
// Will be replaced with pure Go decoder in later stages.
func DecodeAudioToPCM(input []byte, format string, cb ProgressCallback) ([]int16, int, error) {
	if cb != nil {
		cb(0, "Decoding audio...")
	}

	var args []string
	switch format {
	case "webm", "opus":
		args = []string{"-i", "pipe:0", "-f", "s16le", "-acodec", "pcm_s16le", "-ar", "44100", "-ac", "2", "pipe:1"}
	case "m4a", "aac":
		args = []string{"-i", "pipe:0", "-f", "s16le", "-acodec", "pcm_s16le", "-ar", "44100", "-ac", "2", "pipe:1"}
	default:
		return nil, 0, fmt.Errorf("unsupported format: %s", format)
	}

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdin = bytes.NewReader(input)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, 0, fmt.Errorf("ffmpeg decode: %w\n%s", err, stderr.String())
	}

	raw := stdout.Bytes()
	if len(raw)%2 != 0 {
		return nil, 0, fmt.Errorf("odd PCM byte length: %d", len(raw))
	}

	samples := make([]int16, len(raw)/2)
	if err := binary.Read(bytes.NewReader(raw), binary.LittleEndian, &samples); err != nil {
		return nil, 0, fmt.Errorf("binary read PCM: %w", err)
	}

	if cb != nil {
		cb(100, "Decoded")
	}
	return samples, 44100, nil
}

// EncodePCMToMP3 encodes PCM s16le samples to MP3.
// Uses pure Go encoder if possible, falls back to ffmpeg.
func EncodePCMToMP3(pcm []int16, sampleRate, channels int, bitrate string, cb ProgressCallback) ([]byte, error) {
	if cb != nil {
		cb(0, "Encoding MP3...")
	}

	br := 192
	if bitrate != "" {
		// Parse "192k" -> 192
		brStr := bitrate
		if len(brStr) > 0 && brStr[len(brStr)-1] == 'k' {
			brStr = brStr[:len(brStr)-1]
		}
		if b, err := fmt.Sscanf(brStr, "%d", &br); err != nil || b != 1 {
			br = 192
		}
	}

	// Try pure Go encoder
	mp3, err := EncodePCMToMP3Go(pcm, sampleRate, channels, br)
	if err == nil && len(mp3) > 4 {
		if cb != nil {
			cb(100, "Encoded (pure Go)")
		}
		return mp3, nil
	}

	// Fallback to ffmpeg
	if cb != nil {
		cb(0, "Pure Go encoder failed, trying ffmpeg...")
	}

	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, pcm); err != nil {
		return nil, fmt.Errorf("binary write PCM: %w", err)
	}

	args := []string{
		"-f", "s16le",
		"-ar", fmt.Sprintf("%d", sampleRate),
		"-ac", fmt.Sprintf("%d", channels),
		"-i", "pipe:0",
		"-codec:a", "libmp3lame",
		"-b:a", bitrate,
		"-f", "mp3",
		"pipe:1",
	}

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdin = &buf

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg encode: %w\n%s", err, stderr.String())
	}

	if cb != nil {
		cb(100, "Encoded (ffmpeg)")
	}
	return stdout.Bytes(), nil
}

// WebMToMP3 is the full pipeline: WebM bytes → decode to PCM → encode to MP3.
// Uses ffmpeg for both decode and encode. Will be pure Go later.
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

// GetAudioDurationSec returns duration of audio in seconds using ffmpeg probe.
func GetAudioDurationSec(input []byte, format string) (float64, error) {
	cmd := exec.Command("ffprobe",
		"-i", "pipe:0",
		"-show_entries", "format=duration",
		"-v", "quiet",
		"-of", "csv=p=0",
	)
	cmd.Stdin = bytes.NewReader(input)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("ffprobe: %w\n%s", err, stderr.String())
	}

	var dur float64
	if _, err := fmt.Sscanf(stdout.String(), "%f", &dur); err != nil {
		return 0, fmt.Errorf("parse duration: %w", err)
	}
	return dur, nil
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
	DurationNs int64 // duration in nanoseconds from WebM Info
	TrackInfo  *EBMLAudioTrack
}

// DecodeWebMToPCM decodes raw WebM bytes to PCM s16le.
// Uses pure Go Opus decoder if possible, falls back to ffmpeg.
func DecodeWebMToPCM(webmData []byte, cb ProgressCallback) (*WebMDecodeResult, error) {
	if cb != nil {
		cb(0, "Parsing WebM header...")
	}

	// Parse WebM metadata using pure Go EBML parser
	pr, err := ParseWebM(webmData)
	if err != nil {
		// Fallback: try pure ffmpeg
		if cb != nil {
			cb(10, "WebM parse failed, trying ffmpeg...")
		}
		pcm, sr, err2 := DecodeAudioToPCM(webmData, "webm", nil)
		if err2 != nil {
			return nil, fmt.Errorf("webm parse: %v; ffmpeg: %v", err, err2)
		}
		return &WebMDecodeResult{
			Samples:    pcm,
			SampleRate: sr,
			Channels:   2,
		}, nil
	}

	// Find audio track
	if len(pr.Tracks) == 0 {
		return nil, fmt.Errorf("no audio tracks in WebM")
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
		cb(20, fmt.Sprintf("WebM: %dch %dHz, track=%d", channels, sampleRate, track.TrackNumber))
	}

	// Try pure Go Opus decoder first
	if cb != nil {
		cb(25, "Decoding Opus (pure Go)...")
	}

	pcm, err := decodeWebMWithGoDecoder(webmData, sampleRate, channels)
	if err != nil {
		if cb != nil {
			cb(30, "Pure Go decoder failed, falling back to ffmpeg...")
		}
		// Fallback to ffmpeg
		var sr int
		pcm, sr, err = DecodeAudioToPCM(webmData, "webm", func(pct int, msg string) {
			if cb != nil {
				cb(30+pct*60/100, msg)
			}
		})
		if err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}
		sampleRate = sr
		channels = 2
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

// decodeWebMWithGoDecoder decodes WebM Opus data using the pure Go decoder.
func decodeWebMWithGoDecoder(webmData []byte, sampleRate, channels int) ([]int16, error) {
	packets, _, _, err := DecodeWebMOpusPackets(webmData)
	if err != nil {
		return nil, fmt.Errorf("extract packets: %w", err)
	}
	if len(packets) == 0 {
		return nil, fmt.Errorf("no Opus packets found")
	}
	out, err := DecodeOpusToPCM(packets, sampleRate)
	if err != nil {
		return nil, fmt.Errorf("opus decode: %w", err)
	}
	return out, nil
}

// DecodeWebMToPCMGo is like DecodeWebMToPCM but uses pure Go only, no ffmpeg fallback.
func DecodeWebMToPCMGo(webmData []byte, cb ProgressCallback) (*WebMDecodeResult, error) {
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
		cb(20, "Decoding Opus (pure Go)...")
	}

	pcm, err := decodeWebMWithGoDecoder(webmData, sampleRate, channels)
	if err != nil {
		return nil, err
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

// DecodeWebMOpusPackets extracts Opus packets from WebM using pure Go parser
// (no ffmpeg). Returns parsed Opus packets for custom decoding.
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
