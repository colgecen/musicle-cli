"""
MusicLe Engine — play.py
Music playback using pygame. Designed as a singleton used by main.py daemon.
Falls back gracefully if pygame is unavailable.
"""
import os
import time
import threading
import math
import random

try:
    import pygame
    import pygame.mixer
    PYGAME_AVAILABLE = True
except ImportError:
    PYGAME_AVAILABLE = False


class MusicPlayer:
    def __init__(self):
        self._initialized = False
        self._lock = threading.Lock()
        self._current_file: str = ""
        self._length: float = 0.0
        self._start_time: float = 0.0
        self._pause_offset: float = 0.0
        self._paused: bool = False
        self._volume: float = 0.7
        # Audio metadata
        self._format: str = ""
        self._sample_rate: int = 0
        self._bitrate: int = 0
        # Spectrum profile: list of 16-band energy arrays per time chunk
        self._spectrum_profile: list = []     # [[b0..b15], ...]
        self._spectrum_chunk_duration: float = 0.05
        self._last_spectrum: list = [0.0] * 16
        self._last_spectrum_time: float = 0.0
        self._spectrum_bands: int = 16

    def init(self):
        if self._initialized:
            return
        if not PYGAME_AVAILABLE:
            return
        try:
            pygame.mixer.pre_init(44100, -16, 2, 4096)
            pygame.mixer.init()
            self._initialized = True
        except Exception:
            pass  # fall back to simulated playback

    def _ensure_init(self):
        if not self._initialized:
            self.init()

    def _compute_spectrum(self, file_path: str):
        """Precompute 16-band frequency spectrum using real FFT or simulation."""
        try:
            from metadata import extract_metadata
            meta = extract_metadata(file_path)
            self._format = meta.get("format", "")
            self._sample_rate = meta.get("sample_rate", 0)
            self._bitrate = meta.get("bitrate", 0)

            try:
                import soundfile as sf
                import numpy as np

                data, sr = sf.read(file_path)
                if data.ndim == 2:
                    data = data.mean(axis=1)
                self._sample_rate = int(sr)

                fft_size = 2048
                hop_length = fft_size // 4
                window = np.hanning(fft_size)

                bands = self._spectrum_bands
                freq_min = 30.0
                freq_max = min(sr / 2, 18000.0)
                band_edges = np.logspace(np.log10(freq_min), np.log10(freq_max), bands + 1)

                n_chunks = max(1, (len(data) - fft_size) // hop_length + 1)
                profile = []
                for i in range(n_chunks):
                    start = i * hop_length
                    frame = data[start:start + fft_size] * window
                    fft = np.fft.rfft(frame)
                    mag = np.abs(fft) / fft_size
                    freqs = np.fft.rfftfreq(fft_size, 1.0 / sr)

                    energies = []
                    for b in range(bands):
                        mask = (freqs >= band_edges[b]) & (freqs < band_edges[b + 1])
                        energy = float(np.sqrt(np.mean(mag[mask]**2))) if np.any(mask) else 0.0
                        energies.append(energy)
                    profile.append(energies)

                # Normalize each band across time
                arr = np.array(profile)
                for b in range(bands):
                    col = arr[:, b]
                    mx = np.max(col)
                    if mx > 0:
                        arr[:, b] = np.clip(col / mx, 0.0, 1.0)
                self._spectrum_profile = arr.tolist()
                self._spectrum_chunk_duration = hop_length / sr
            except ImportError:
                self._simulate_spectrum()
            except Exception:
                self._simulate_spectrum()
        except Exception:
            self._simulate_spectrum()

    def _simulate_spectrum(self):
        """Generate a simulated 16-band spectrum when real FFT unavailable."""
        if self._length <= 0:
            self._length = 180.0
        n_chunks = max(10, int(self._length / self._spectrum_chunk_duration))
        random.seed(abs(hash(self._current_file)) & 0xFFFFFFFF) if self._current_file else random.seed(0)
        bands = self._spectrum_bands
        profile = []
        for i in range(n_chunks):
            energies = []
            phase = i * 0.05
            for b in range(bands):
                freq_factor = b / bands
                val = 0.3 + 0.7 * (math.sin(phase * (1.0 + freq_factor * 3.0)) * 0.5 + 0.5)
                val *= 1.0 - freq_factor * 0.4
                val += random.uniform(-0.05, 0.05)
                energies.append(max(0.0, min(1.0, val)))
            profile.append(energies)
        self._spectrum_profile = profile

    def _get_spectrum(self) -> list:
        """Return 16-band spectrum values 0.0-1.0 at current playback position."""
        now = time.time()
        if not self._current_file or self._length <= 0:
            return [0.0] * self._spectrum_bands

        pos = self._current_position()
        if not self._spectrum_profile:
            self._simulate_spectrum()

        chunk_idx = int(pos / self._spectrum_chunk_duration) if self._spectrum_chunk_duration > 0 else 0
        if chunk_idx < 0:
            chunk_idx = 0
        if chunk_idx >= len(self._spectrum_profile):
            chunk_idx = len(self._spectrum_profile) - 1

        if self._paused:
            elapsed = now - self._last_spectrum_time
            decay = max(0.0, 1.0 - elapsed * 2.0)
            if self._spectrum_profile and chunk_idx < len(self._spectrum_profile):
                raw = self._spectrum_profile[chunk_idx]
            else:
                raw = [0.0] * self._spectrum_bands
            result = [v * decay for v in raw]
            self._last_spectrum = result
            self._last_spectrum_time = now
            return result

        if self._spectrum_profile and chunk_idx < len(self._spectrum_profile):
            result = list(self._spectrum_profile[chunk_idx])
        else:
            result = [0.0] * self._spectrum_bands

        # Add micro-variation
        for b in range(self._spectrum_bands):
            jitter = math.sin(now * (4.0 + b * 2.0)) * 0.03 + math.sin(now * (7.0 + b * 1.3)) * 0.02
            result[b] = max(0.0, min(1.0, result[b] + jitter))

        self._last_spectrum = result
        self._last_spectrum_time = now
        return result

    def play(self, file_path: str) -> dict:
        self._ensure_init()
        file_path = os.path.normpath(file_path)
        if not os.path.isfile(file_path):
            # Try common alternative paths
            alt = file_path.replace("\\", "/")
            if os.path.isfile(alt):
                file_path = alt
            else:
                alt2 = file_path.replace("/", "\\")
                if os.path.isfile(alt2):
                    file_path = alt2
                else:
                    return {"status": "error", "error": f"File not found: {file_path}"}

        with self._lock:
            if not PYGAME_AVAILABLE:
                # Simulate playback for testing without pygame
                self._current_file = file_path
                self._length = 180.0
                self._start_time = time.time()
                self._paused = False
                self._compute_spectrum(file_path)
                spec = self._get_spectrum()
                return {
                    "status": "playing", "duration": 180.0, "position": 0.0,
                    "filename": os.path.basename(file_path),
                    "format": self._format, "sample_rate": self._sample_rate,
                    "bitrate": self._bitrate,
                    "spectrum": spec,
                }

            try:
                pygame.mixer.music.load(file_path)
                pygame.mixer.music.set_volume(self._volume)
                pygame.mixer.music.play()
                self._current_file = file_path
                self._start_time = time.time()
                self._pause_offset = 0.0
                self._paused = False

                # Try to get duration via mutagen + precompute spectrum
                try:
                    from metadata import get_duration
                    self._length = get_duration(file_path)
                except Exception:
                    self._length = 0.0

                # Precompute spectrum for analyzer
                self._compute_spectrum(file_path)
                spec = self._get_spectrum()

                return {
                    "status": "playing",
                    "duration": self._length,
                    "position": 0.0,
                    "filename": os.path.basename(file_path),
                    "format": self._format,
                    "sample_rate": self._sample_rate,
                    "bitrate": self._bitrate,
                    "spectrum": spec,
                }
            except Exception as e:
                return {"status": "error", "error": str(e)}

    def pause(self) -> dict:
        self._ensure_init()
        with self._lock:
            if not PYGAME_AVAILABLE:
                self._paused = True
                self._pause_offset = time.time() - self._start_time
                return {"status": "paused"}
            if pygame.mixer.music.get_busy():
                pygame.mixer.music.pause()
                self._pause_offset = time.time() - self._start_time
                self._paused = True
            return {"status": "paused"}

    def resume(self) -> dict:
        self._ensure_init()
        with self._lock:
            if not PYGAME_AVAILABLE:
                self._start_time = time.time() - self._pause_offset
                self._paused = False
                return {"status": "playing"}
            pygame.mixer.music.unpause()
            self._start_time = time.time() - self._pause_offset
            self._paused = False
            return {"status": "playing"}

    def stop(self) -> dict:
        self._ensure_init()
        with self._lock:
            if PYGAME_AVAILABLE:
                pygame.mixer.music.stop()
            self._paused = False
            self._current_file = ""
            return {"status": "stopped"}

    def seek(self, delta_seconds: float) -> dict:
        """Seek forward (positive) or backward (negative) by delta_seconds."""
        self._ensure_init()
        with self._lock:
            pos = self._current_position()
            new_pos = max(0.0, pos + delta_seconds)
            if PYGAME_AVAILABLE and self._current_file:
                try:
                    pygame.mixer.music.rewind()
                    pygame.mixer.music.set_pos(new_pos)
                    self._start_time = time.time() - new_pos
                    self._pause_offset = new_pos
                except Exception:
                    pass
            else:
                self._start_time = time.time() - new_pos
            return {"status": "ok", "position": new_pos}

    def set_volume(self, vol: float) -> dict:
        self._ensure_init()
        vol = max(0.0, min(1.0, vol))
        self._volume = vol
        if PYGAME_AVAILABLE:
            pygame.mixer.music.set_volume(vol)
        return {"status": "ok", "volume": vol}

    def status(self) -> dict:
        self._ensure_init()
        with self._lock:
            pos = self._current_position()
            is_busy = False
            if PYGAME_AVAILABLE and self._current_file:
                is_busy = pygame.mixer.music.get_busy()
            elif self._current_file and not self._paused:
                # Simulate: playing until length
                is_busy = (self._length == 0 or pos < self._length)

            spec = self._get_spectrum()

            if not self._current_file:
                return {"status": "idle", "position": 0.0, "duration": 0.0, "volume": self._volume}

            if self._paused:
                return {
                    "status": "paused", "position": pos, "duration": self._length,
                    "volume": self._volume, "spectrum": spec,
                    "format": self._format, "sample_rate": self._sample_rate, "bitrate": self._bitrate,
                }

            if not is_busy:
                return {
                    "status": "stopped", "position": pos, "duration": self._length,
                    "volume": self._volume, "spectrum": spec,
                    "format": self._format, "sample_rate": self._sample_rate, "bitrate": self._bitrate,
                }

            return {
                "status": "playing", "position": pos, "duration": self._length,
                "volume": self._volume, "spectrum": spec,
                "format": self._format, "sample_rate": self._sample_rate, "bitrate": self._bitrate,
            }

    def _current_position(self) -> float:
        if self._paused:
            return self._pause_offset
        if self._start_time == 0:
            return 0.0
        return time.time() - self._start_time


# Singleton player instance used by main.py
player_instance = MusicPlayer()
