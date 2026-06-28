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

// EncodePCMToMP3 encodes PCM s16le samples to MP3 using ffmpeg.
// Returns MP3 bytes.
// Will be replaced with pure Go encoder in later stages.
func EncodePCMToMP3(pcm []int16, sampleRate, channels int, bitrate string, cb ProgressCallback) ([]byte, error) {
	if cb != nil {
		cb(0, "Encoding MP3...")
	}

	if bitrate == "" {
		bitrate = "192k"
	}

	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, pcm); err != nil {
		return nil, fmt.Errorf("binary write PCM: %w", err)
	}

	args := []string{
		"-f", "s16le",
		fmt.Sprintf("-ar:%d", sampleRate),
		fmt.Sprintf("-ac:%d", channels),
		"-i", "pipe:0",
		"-codec", "libmp3lame",
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
		cb(100, "Encoded")
	}
	return stdout.Bytes(), nil
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

// WriteMP3File writes MP3 data to an io.WriteSeeker, returning bytes written.
func WriteMP3File(w io.Writer, mp3Data []byte) (int64, error) {
	n, err := w.Write(mp3Data)
	return int64(n), err
}
