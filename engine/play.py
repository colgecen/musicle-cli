"""
MusicLe Engine — play.py
Music playback using pygame. Designed as a singleton used by main.py daemon.
Falls back gracefully if pygame is unavailable.
"""
import os
import time
import threading

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

    def init(self):
        if self._initialized:
            return
        if not PYGAME_AVAILABLE:
            return
        pygame.mixer.pre_init(44100, -16, 2, 4096)
        pygame.mixer.init()
        self._initialized = True

    def _ensure_init(self):
        if not self._initialized:
            self.init()

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
                return {"status": "playing", "duration": 180.0, "position": 0.0, "filename": os.path.basename(file_path)}

            try:
                pygame.mixer.music.load(file_path)
                pygame.mixer.music.set_volume(self._volume)
                pygame.mixer.music.play()
                self._current_file = file_path
                self._start_time = time.time()
                self._pause_offset = 0.0
                self._paused = False

                # Try to get duration via mutagen
                try:
                    from metadata import get_duration
                    self._length = get_duration(file_path)
                except Exception:
                    self._length = 0.0

                return {
                    "status": "playing",
                    "duration": self._length,
                    "position": 0.0,
                    "filename": os.path.basename(file_path),
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

            if not self._current_file:
                return {"status": "idle", "position": 0.0, "duration": 0.0, "volume": self._volume}

            if self._paused:
                return {"status": "paused", "position": pos, "duration": self._length, "volume": self._volume}

            if not is_busy:
                return {"status": "stopped", "position": pos, "duration": self._length, "volume": self._volume}

            return {"status": "playing", "position": pos, "duration": self._length, "volume": self._volume}

    def _current_position(self) -> float:
        if self._paused:
            return self._pause_offset
        if self._start_time == 0:
            return 0.0
        return time.time() - self._start_time


# Singleton player instance used by main.py
player_instance = MusicPlayer()
