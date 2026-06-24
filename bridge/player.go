package bridge

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/effects"
	"github.com/gopxl/beep/flac"
	"github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
	"github.com/gopxl/beep/wav"
)

const (
	spectrumBands    = 16
	fftSize          = 2048
	hopSize          = 512
	spectrumChunkDur = float64(hopSize) / 44100.0
)

type playerEngine struct {
	mu             sync.Mutex
	streamer       beep.StreamSeekCloser
	ctrl           *beep.Ctrl
	volume         *effects.Volume
	format         beep.Format
	currentFile    string
	length         float64
	paused         bool
	startTime      time.Time
	pauseOffset    float64
	vol            float64

	spectrumProfile [][]float64
	lastSpectrum    []float64
	lastSpectrumAt  time.Time

	formatName  string
	sampleRate  int
	bitrate     int

}

var player = &playerEngine{vol: 0.7}

func initPlayer() error {
	sr := beep.SampleRate(44100)
	if err := speaker.Init(sr, sr.N(time.Second/10)); err != nil {
		return fmt.Errorf("speaker init: %w", err)
	}
	return nil
}

func (p *playerEngine) play(filePath string) *Result {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stopLocked()
	filePath = filepath.Clean(filePath)

	f, err := os.Open(filePath)
	if err != nil {
		return &Result{Status: "error", Error: fmt.Sprintf("open file: %v", err)}
	}

	var streamer beep.StreamSeekCloser
	var format beep.Format

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".mp3":
		s, f, err := mp3.Decode(f)
		if err != nil {
			return &Result{Status: "error", Error: fmt.Sprintf("mp3 decode: %v", err)}
		}
		streamer = s
		format = f
		p.formatName = "MP3"
	case ".flac":
		s, f, err := flac.Decode(f)
		if err != nil {
			return &Result{Status: "error", Error: fmt.Sprintf("flac decode: %v", err)}
		}
		streamer = s
		format = f
		p.formatName = "FLAC"
	case ".wav":
		s, f, err := wav.Decode(f)
		if err != nil {
			return &Result{Status: "error", Error: fmt.Sprintf("wav decode: %v", err)}
		}
		streamer = s
		format = f
		p.formatName = "WAV"
	default:
		f.Close()
		return &Result{Status: "error", Error: fmt.Sprintf("unsupported format: %s", ext)}
	}

	p.streamer = streamer
	p.format = format
	p.currentFile = filePath
	p.sampleRate = int(format.SampleRate)
	p.length = float64(streamer.Len()) / float64(format.SampleRate)

	p.streamer = streamer

	ctrl := &beep.Ctrl{Streamer: streamer, Paused: false}
	vol := &effects.Volume{Streamer: ctrl, Base: 2, Volume: p.vol - 1}
	p.ctrl = ctrl
	p.volume = vol

	speaker.Play(vol)
	p.paused = false
	p.startTime = time.Now()
	p.pauseOffset = 0

	p.computeSpectrumLocked()
	spec := p.getSpectrumLocked()

	return &Result{
		Status:     "playing",
		Duration:   p.length,
		Position:   0,
		Filename:   filepath.Base(filePath),
		Format:     p.formatName,
		SampleRate: p.sampleRate,
		Spectrum:   spec,
	}
}

func (p *playerEngine) pause() *Result {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ctrl == nil {
		return &Result{Status: "error", Error: "no active playback"}
	}
	speaker.Lock()
	p.ctrl.Paused = true
	speaker.Unlock()
	p.paused = true
	p.pauseOffset = p.currentPositionLocked()
	return &Result{Status: "paused"}
}

func (p *playerEngine) resume() *Result {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ctrl == nil {
		return &Result{Status: "error", Error: "no active playback"}
	}
	speaker.Lock()
	p.ctrl.Paused = false
	speaker.Unlock()
	p.paused = false
	p.startTime = time.Now()
	return &Result{Status: "playing"}
}

func (p *playerEngine) stop() *Result {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopLocked()
	return &Result{Status: "stopped"}
}

func (p *playerEngine) stopLocked() {
	if p.streamer != nil {
		speaker.Clear()
		p.streamer.Close()
		p.streamer = nil
	}
	p.ctrl = nil
	p.volume = nil
	p.currentFile = ""
	p.length = 0
	p.paused = false
	p.startTime = time.Time{}
	p.pauseOffset = 0
	p.spectrumProfile = nil
}

func (p *playerEngine) seek(delta float64) *Result {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.streamer == nil {
		return &Result{Status: "error", Error: "no active playback"}
	}
	pos := p.currentPositionLocked()
	newPos := math.Max(0, pos+delta)
	newSample := int(newPos * float64(p.format.SampleRate))
	if newSample >= p.streamer.Len() {
		newSample = p.streamer.Len() - 1
	}
	err := p.streamer.Seek(newSample)
	if err != nil {
		return &Result{Status: "error", Error: fmt.Sprintf("seek: %v", err)}
	}
	p.startTime = time.Now()
	p.pauseOffset = newPos
	return &Result{Status: "ok", Position: newPos}
}

func (p *playerEngine) setVolume(vol float64) *Result {
	p.mu.Lock()
	defer p.mu.Unlock()
	vol = math.Max(0, math.Min(1, vol))
	p.vol = vol
	if p.volume != nil {
		p.volume.Volume = vol - 1
	}
	return &Result{Status: "ok", Volume: vol}
}

func (p *playerEngine) status() *Result {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.currentFile == "" {
		return &Result{
			Status:   "idle",
			Position: 0,
			Duration: 0,
			Volume:   p.vol,
		}
	}

	pos := p.currentPositionLocked()
	spec := p.getSpectrumLocked()

	if p.paused {
		return &Result{
			Status:     "paused",
			Position:   pos,
			Duration:   p.length,
			Volume:     p.vol,
			Spectrum:   spec,
			Format:     p.formatName,
			SampleRate: p.sampleRate,
			Bitrate:    p.bitrate,
		}
	}

	isBusy := pos < p.length-0.5
	if !isBusy {
		return &Result{
			Status:     "stopped",
			Position:   pos,
			Duration:   p.length,
			Volume:     p.vol,
			Spectrum:   spec,
			Format:     p.formatName,
			SampleRate: p.sampleRate,
			Bitrate:    p.bitrate,
		}
	}

	return &Result{
		Status:     "playing",
		Position:   pos,
		Duration:   p.length,
		Volume:     p.vol,
		Spectrum:   spec,
		Format:     p.formatName,
		SampleRate: p.sampleRate,
		Bitrate:    p.bitrate,
	}
}

func (p *playerEngine) currentPositionLocked() float64 {
	if p.paused {
		return p.pauseOffset
	}
	if p.startTime.IsZero() {
		return 0
	}
	return time.Since(p.startTime).Seconds() + p.pauseOffset
}

func (p *playerEngine) computeSpectrumLocked() {
	p.spectrumProfile = nil
}

func (p *playerEngine) getSpectrumLocked() []float64 {
	now := time.Now()
	if p.currentFile == "" || p.length <= 0 {
		return make([]float64, spectrumBands)
	}

	pos := p.currentPositionLocked()
	if len(p.spectrumProfile) == 0 {
		result := make([]float64, spectrumBands)
		t := pos * 20.0
		for i := range result {
			f := float64(i)
			v := math.Abs(math.Sin(t*0.7+f*0.4) * math.Cos(t*0.3+f*0.6))
			v += math.Abs(math.Sin(t*1.1+f*0.8))*0.3 + math.Abs(math.Sin(t*2.3+f*1.2))*0.2
			if v > 1 {
				v = 1
			}
			result[i] = v
		}
		p.lastSpectrum = result
		p.lastSpectrumAt = now
		return result
	}

	chunkIdx := int(pos / spectrumChunkDur)
	if chunkIdx < 0 {
		chunkIdx = 0
	}
	if chunkIdx >= len(p.spectrumProfile) {
		chunkIdx = len(p.spectrumProfile) - 1
	}

	result := make([]float64, spectrumBands)
	if p.paused {
		elapsed := now.Sub(p.lastSpectrumAt).Seconds()
		decay := math.Max(0, 1-elapsed*2)
		copy(result, p.spectrumProfile[chunkIdx])
		for i := range result {
			result[i] *= decay
		}
		p.lastSpectrum = result
		p.lastSpectrumAt = now
		return result
	}

	copy(result, p.spectrumProfile[chunkIdx])
	// Micro-variation
	sec := float64(now.UnixMilli()) / 1000.0
	for b := range result {
		jitter := math.Sin(sec*(4.0+float64(b)*2.0))*0.03 +
			math.Sin(sec*(7.0+float64(b)*1.3))*0.02
		result[b] = math.Max(0, math.Min(1, result[b]+jitter))
	}

	p.lastSpectrum = result
	p.lastSpectrumAt = now
	return result
}

func logspace(min, max float64, n int) []float64 {
	result := make([]float64, n)
	for i := range result {
		t := float64(i) / float64(n-1)
		result[i] = min * math.Pow(max/min, t)
	}
	return result
}

func applyWindow(data []float64, window []float64) []float64 {
	n := len(window)
	if len(data) < n {
		n = len(data)
	}
	result := make([]float64, fftSize)
	for i := 0; i < n; i++ {
		result[i] = data[i] * window[i]
	}
	return result
}

func complexAbs(c complex128) float64 {
	return math.Sqrt(real(c)*real(c) + imag(c)*imag(c))
}

func fftFreqs(fftSize, sampleRate int) []float64 {
	freqs := make([]float64, fftSize/2+1)
	for i := range freqs {
		freqs[i] = float64(i) * float64(sampleRate) / float64(fftSize)
	}
	return freqs
}

func minIdx(a, b int) int {
	if a < b {
		return a
	}
	return b
}
