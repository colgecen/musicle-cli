package bridge

import (
	"sync"

	"MusicLeCLI/state"
)

// Action is sent to the audio or playlist engine
type Action struct {
	Action string      `json:"action"`
	File   string      `json:"file,omitempty"`
	URL    string      `json:"url,omitempty"`
	Output string      `json:"output,omitempty"`
	Value  interface{} `json:"value,omitempty"`
	Path   string      `json:"path,omitempty"`
}

// Result is returned from any engine operation
type Result struct {
	Status      string   `json:"status"`
	Error       string   `json:"error,omitempty"`
	Title       string   `json:"title,omitempty"`
	Artist      string   `json:"artist,omitempty"`
	Album       string   `json:"album,omitempty"`
	Duration    float64  `json:"duration,omitempty"`
	Position    float64  `json:"position,omitempty"`
	Volume      float64  `json:"volume,omitempty"`
	ArtPath     string   `json:"art_path,omitempty"`
	Filename    string   `json:"filename,omitempty"`
	Message     string   `json:"message,omitempty"`
	Percent     float64  `json:"percent,omitempty"`
	Songs       []Result `json:"songs,omitempty"`
	Format      string   `json:"format,omitempty"`
	SampleRate  int      `json:"sample_rate,omitempty"`
	Bitrate     int      `json:"bitrate,omitempty"`
	AudioLevelL float64  `json:"audio_level_l,omitempty"`
	AudioLevelR float64  `json:"audio_level_r,omitempty"`
	Spectrum    []float64 `json:"spectrum,omitempty"`
}

// DownloadProgress tracks the current download state shared between goroutines
type DownloadProgress struct {
	mu      sync.RWMutex
	Active  bool
	Percent float64
	Message string
}

var CurrentDownload DownloadProgress

func (dp *DownloadProgress) Set(active bool, pct float64, msg string) {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	dp.Active = active
	dp.Percent = pct
	dp.Message = msg
}

func (dp *DownloadProgress) Get() (bool, float64, string) {
	dp.mu.RLock()
	defer dp.mu.RUnlock()
	return dp.Active, dp.Percent, dp.Message
}

// Init initializes the audio engine.
func Init(projectDir string) {
	_ = initPlayer()
}

// PlayerCall handles audio engine actions.
func PlayerCall(action Action) (*Result, error) {
	switch action.Action {
	case "play":
		return player.play(action.File), nil
	case "pause":
		return player.pause(), nil
	case "resume":
		return player.resume(), nil
	case "stop":
		return player.stop(), nil
	case "seek":
		val, _ := action.Value.(float64)
		if intVal, ok := action.Value.(int); ok {
			val = float64(intVal)
		}
		return player.seek(val), nil
	case "volume":
		val, _ := action.Value.(float64)
		if intVal, ok := action.Value.(int); ok {
			val = float64(intVal)
		}
		return player.setVolume(val), nil
	case "status":
		return player.status(), nil
	default:
		return &Result{Status: "error", Error: "unknown action: " + action.Action}, nil
	}
}

// RunScript handles one-shot operations (metadata, playlist, import).
func RunScript(action Action) (*Result, error) {
	switch action.Action {
	case "update_song":
		vals, _ := action.Value.(map[string]interface{})
		title, _ := vals["title"].(string)
		artist, _ := vals["artist"].(string)
		duration, _ := vals["duration"].(string)
		if err := state.UpdateSong(action.File, action.Path, title, artist, duration); err != nil {
			return &Result{Status: "error", Error: err.Error()}, err
		}
		return &Result{Status: "ok", Title: title, Artist: artist}, nil

	case "remove_song":
		if err := state.RemoveSong(action.File, action.Path); err != nil {
			return &Result{Status: "error", Error: err.Error()}, err
		}
		return &Result{Status: "ok"}, nil

	case "metadata":
		return extractMetadata(action.File), nil

	case "add_local":
		return addLocalFile(action.File, action.Output), nil

	default:
		return &Result{Status: "error", Error: "unknown action: " + action.Action}, nil
	}
}

// RunScriptDownload handles downloads with progress streaming.
func RunScriptDownload(action Action) (*Result, error) {
	var result *Result
	switch action.Action {
	case "download_youtube":
		result = downloadYouTube(action.URL, action.Output)
	case "download_spotify":
		result = downloadSpotify(action.URL, action.Output)
	default:
		return &Result{Status: "error", Error: "unknown download action: " + action.Action}, nil
	}
	if result == nil {
		return &Result{Status: "error", Error: "download returned nil"}, nil
	}
	return result, nil
}

// StopAll cleans up the audio engine.
func StopAll() {
	player.stop()
}
