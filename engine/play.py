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
        # Energy profile for VU meter (precomputed RMS values)
        self._energy_profile: list = []
        self._energy_chunk_duration: float = 0.05  # 50ms per chunk
        self._last_audio_levels: tuple = (0.0, 0.0)
        self._level_time: float = 0.0

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

    def _compute_energy(self, file_path: str):
        """Precompute RMS energy profile for VU meter."""
        try:
            from metadata import extract_metadata
            meta = extract_metadata(file_path)
            self._format = meta.get("format", "")
            self._sample_rate = meta.get("sample_rate", 0)
            self._bitrate = meta.get("bitrate", 0)

            # Try to read audio data for real energy
            waveform = None
            try:
                import soundfile as sf
                data, sr = sf.read(file_path)
                if data.ndim == 2:
                    data = data.mean(axis=1)
                waveform = data
                self._sample_rate = int(sr) if self._sample_rate == 0 else self._sample_rate
            except ImportError:
                pass
            except Exception:
                pass

            if waveform is not None:
                chunk_size = max(1, int(self._sample_rate * self._energy_chunk_duration))
                n_chunks = max(1, len(waveform) // chunk_size)
                profile = []
                for i in range(n_chunks):
                    chunk = waveform[i * chunk_size : (i + 1) * chunk_size]
                    rms = math.sqrt(sum(s * s for s in chunk) / len(chunk))
                    profile.append(rms)
                mx = max(profile) if profile else 1.0
                if mx > 0:
                    profile = [v / mx for v in profile]
                self._energy_profile = profile
            else:
                self._simulate_energy()
        except Exception:
            self._simulate_energy()

    def _simulate_energy(self, n_chunks: int = 0):
        """Generate a simulated energy profile for VU when real data unavailable."""
        if n_chunks == 0 and self._length > 0:
            n_chunks = max(10, int(self._length / self._energy_chunk_duration))
        else:
            n_chunks = max(10, n_chunks)
        random.seed(abs(hash(self._current_file)) & 0xFFFFFFFF) if self._current_file else random.seed(0)
        profile = []
        phase = 0.0
        for i in range(n_chunks):
            phase += 0.05 + random.uniform(-0.02, 0.08)
            val = 0.3 + 0.7 * (math.sin(phase * 2.0) * 0.5 + 0.5)
            val *= 0.5 + 0.5 * math.sin(i * 0.01)
            val += random.uniform(-0.1, 0.1)
            profile.append(max(0.0, min(1.0, val)))
        self._energy_profile = profile

    def _get_audio_levels(self) -> tuple:
        """Return (left, right) audio levels 0.0-1.0 based on current position."""
        now = time.time()
        if not self._current_file or self._length <= 0:
            return (0.0, 0.0)
        if self._paused:
            # Fade out when paused
            elapsed = now - self._level_time
            decay = max(0.0, 1.0 - elapsed * 2.0)
            lvl = self._last_audio_levels[0] * decay
            return (lvl, lvl * 0.8)

        pos = self._current_position()
        if not self._energy_profile:
            self._simulate_energy()

        if self._energy_profile:
            chunk_idx = int(pos / self._energy_chunk_duration)
            if chunk_idx < 0:
                chunk_idx = 0
            if chunk_idx >= len(self._energy_profile):
                chunk_idx = len(self._energy_profile) - 1
            base = self._energy_profile[chunk_idx] if chunk_idx < len(self._energy_profile) else 0.0
        else:
            base = 0.0

        # Add micro-variation for organic feel
        variation = math.sin(now * 8.0) * 0.05 + math.sin(now * 13.0) * 0.03
        l = max(0.0, min(1.0, base + variation))
        r = max(0.0, min(1.0, l * (0.8 + 0.2 * math.sin(now * 7.0))))
        self._last_audio_levels = (l, r)
        self._level_time = now
        return (l, r)

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
                self._compute_energy(file_path)
                l, r = self._get_audio_levels()
                return {
                    "status": "playing", "duration": 180.0, "position": 0.0,
                    "filename": os.path.basename(file_path),
                    "format": self._format, "sample_rate": self._sample_rate,
                    "bitrate": self._bitrate,
                    "audio_level_l": l, "audio_level_r": r,
                }

            try:
                pygame.mixer.music.load(file_path)
                pygame.mixer.music.set_volume(self._volume)
                pygame.mixer.music.play()
                self._current_file = file_path
                self._start_time = time.time()
                self._pause_offset = 0.0
                self._paused = False

                # Try to get duration via mutagen + precompute energy
                try:
                    from metadata import get_duration
                    self._length = get_duration(file_path)
                except Exception:
                    self._length = 0.0

                # Precompute energy profile for VU meter
                self._compute_energy(file_path)
                l, r = self._get_audio_levels()

                return {
                    "status": "playing",
                    "duration": self._length,
                    "position": 0.0,
                    "filename": os.path.basename(file_path),
                    "format": self._format,
                    "sample_rate": self._sample_rate,
                    "bitrate": self._bitrate,
                    "audio_level_l": l,
                    "audio_level_r": r,
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

            lvl_l, lvl_r = self._get_audio_levels()

            if not self._current_file:
                return {"status": "idle", "position": 0.0, "duration": 0.0, "volume": self._volume}

            if self._paused:
                return {
                    "status": "paused", "position": pos, "duration": self._length,
                    "volume": self._volume, "audio_level_l": lvl_l, "audio_level_r": lvl_r,
                    "format": self._format, "sample_rate": self._sample_rate, "bitrate": self._bitrate,
                }

            if not is_busy:
                return {
                    "status": "stopped", "position": pos, "duration": self._length,
                    "volume": self._volume, "audio_level_l": lvl_l, "audio_level_r": lvl_r,
                    "format": self._format, "sample_rate": self._sample_rate, "bitrate": self._bitrate,
                }

            return {
                "status": "playing", "position": pos, "duration": self._length,
                "volume": self._volume, "audio_level_l": lvl_l, "audio_level_r": lvl_r,
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
