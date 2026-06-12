package bridge

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// Action is sent from Go to Python as a JSON line
type Action struct {
	Action string      `json:"action"`
	File   string      `json:"file,omitempty"`
	URL    string      `json:"url,omitempty"`
	Output string      `json:"output,omitempty"`
	Value  interface{} `json:"value,omitempty"`
	Path   string      `json:"path,omitempty"`
}

// Result is received from Python as a JSON line
type Result struct {
	Status   string  `json:"status"`
	Error    string  `json:"error,omitempty"`
	Title    string  `json:"title,omitempty"`
	Artist   string  `json:"artist,omitempty"`
	Album    string  `json:"album,omitempty"`
	Duration float64 `json:"duration,omitempty"`
	Position float64 `json:"position,omitempty"`
	Volume   float64 `json:"volume,omitempty"`
	ArtPath  string  `json:"art_path,omitempty"`
	Filename string  `json:"filename,omitempty"`
	Message  string  `json:"message,omitempty"`
}

// playerDaemon manages the persistent Python player subprocess
type playerDaemon struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	writer  *bufio.Writer
	reader  *bufio.Reader
	running bool
}

var (
	daemon    = &playerDaemon{}
	engineDir string
	pythonBin string
)

// Init must be called once with the project root before any bridge calls
func Init(projectDir string) {
	engineDir = projectDir
	pythonBin = findPython()
}

func findPython() string {
	for _, name := range []string{"python3", "python", "py"} {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return "python"
}

func setUTF8Env(cmd *exec.Cmd) {
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")
}

func (d *playerDaemon) start() error {
	scriptPath := filepath.Join(engineDir, "engine", "main.py")
	cmd := exec.Command(pythonBin, scriptPath, "--daemon")
	setUTF8Env(cmd)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start python daemon: %w", err)
	}

	d.cmd = cmd
	d.writer = bufio.NewWriter(stdin)
	d.reader = bufio.NewReader(stdout)
	d.running = true
	return nil
}

// PlayerCall sends an action to the persistent daemon (play, pause, seek, volume, status)
func PlayerCall(action Action) (*Result, error) {
	daemon.mu.Lock()
	defer daemon.mu.Unlock()

	if !daemon.running {
		if err := daemon.start(); err != nil {
			return &Result{Status: "error", Error: err.Error()}, err
		}
	}

	data, err := json.Marshal(action)
	if err != nil {
		return nil, err
	}

	if _, err := fmt.Fprintf(daemon.writer, "%s\n", data); err != nil {
		daemon.running = false
		return &Result{Status: "error", Error: err.Error()}, err
	}
	if err := daemon.writer.Flush(); err != nil {
		daemon.running = false
		return &Result{Status: "error", Error: err.Error()}, err
	}

	line, err := daemon.reader.ReadString('\n')
	if err != nil {
		daemon.running = false
		return &Result{Status: "error", Error: err.Error()}, err
	}

	var result Result
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &result); err != nil {
		return nil, fmt.Errorf("parse response: %w (raw: %s)", err, line)
	}
	return &result, nil
}

// RunScript spawns a one-shot Python process for downloads, metadata extraction, etc.
func RunScript(action Action) (*Result, error) {
	data, err := json.Marshal(action)
	if err != nil {
		return nil, err
	}
	scriptPath := filepath.Join(engineDir, "engine", "main.py")
	cmd := exec.Command(pythonBin, scriptPath)
	setUTF8Env(cmd)
	cmd.Stdin = strings.NewReader(string(data) + "\n")

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return &Result{
				Status: "error",
				Error:  fmt.Sprintf("exit %d: %s", exitErr.ExitCode(), string(exitErr.Stderr)),
			}, err
		}
		return &Result{Status: "error", Error: err.Error()}, err
	}

	out = bytes.TrimSpace(out)
	var result Result
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("parse output: %w", err)
	}
	return &result, nil
}

// GetPythonBin returns the resolved Python executable name
func GetPythonBin() string { return pythonBin }

// GetEngineDir returns the resolved engine directory
func GetEngineDir() string { return engineDir }

// StopAll kills the persistent daemon if running
func StopAll() {
	daemon.mu.Lock()
	defer daemon.mu.Unlock()
	if daemon.running && daemon.cmd != nil {
		_ = daemon.cmd.Process.Kill()
		daemon.running = false
	}
}
