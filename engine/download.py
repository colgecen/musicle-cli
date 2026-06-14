"""
MusicLe Engine — download.py
Downloads music from YouTube (yt-dlp) and Spotify (spotdl).
After downloading, extracts metadata and returns structured result.
"""
import os
import re
import subprocess
import sys
import json


def _emit(obj: dict):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()


def _find_exe(names: list) -> str:
    """Return the first executable name found in PATH."""
    import shutil
    for name in names:
        if shutil.which(name):
            return name
    return names[0]  # fallback, will fail with a clear error


def _run_with_progress(cmd: list, timeout: int = 300) -> tuple:
    """Run a subprocess and emit progress lines from stderr. Returns (stdout, stderr, returncode)."""
    proc = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    stdout_lines = []
    stderr_lines = []

    def read_stream(stream, is_stderr):
        for line in iter(stream.readline, ""):
            line = line.rstrip("\n\r")
            if is_stderr:
                stderr_lines.append(line)
                m = re.search(r'(\d+\.?\d*)%', line)
                if m:
                    pct = float(m.group(1))
                    _emit({"status": "progress", "percent": pct, "message": f"{pct:.0f}%"})
            else:
                stdout_lines.append(line)

    import threading
    t1 = threading.Thread(target=read_stream, args=(proc.stdout, False))
    t2 = threading.Thread(target=read_stream, args=(proc.stderr, True))
    t1.start()
    t2.start()
    t1.join()
    t2.join()

    proc.wait(timeout=timeout)
    return "\n".join(stdout_lines), "\n".join(stderr_lines), proc.returncode


def download_youtube(url: str, output_dir: str) -> dict:
    """Download a YouTube URL using yt-dlp, extract audio as mp3."""
    if not url.startswith("http"):
        return {"status": "error", "error": "Invalid URL"}

    os.makedirs(output_dir, exist_ok=True)
    out_template = os.path.join(output_dir, "%(title)s.%(ext)s")

    cmd = [sys.executable, "-m", "yt_dlp",
           url,
           "--extract-audio",
           "--audio-format", "mp3",
           "--audio-quality", "192K",
           "--output", out_template,
           "--no-playlist",
           "--print", "after_move:filepath",
           "--newline",
           "--add-metadata",
           "--parse-metadata", "%(uploader)s:%(artist)s",
           "--embed-thumbnail",
           ]

    try:
        stdout, stderr, rc = _run_with_progress(cmd)
        if rc != 0:
            err = stderr.strip() or "yt-dlp failed"
            return {"status": "error", "error": err}

        filepath = stdout.strip().splitlines()[-1] if stdout.strip() else ""
        if not filepath or not os.path.isfile(filepath):
            filepath = _latest_file(output_dir, ".mp3")

        if not filepath:
            return {"status": "error", "error": "Downloaded file not found"}

        return _finalize_download(filepath, output_dir)

    except FileNotFoundError:
        return {"status": "error", "error": "yt-dlp not installed. Run: pip install yt-dlp"}
    except subprocess.TimeoutExpired:
        return {"status": "error", "error": "Download timed out"}
    except Exception as e:
        return {"status": "error", "error": str(e)}


def download_spotify(url: str, output_dir: str) -> dict:
    """Download a Spotify URL (track or playlist) using spotdl.
    For playlists, downloads ALL songs with Title - Artist naming."""
    if not url.startswith("http"):
        return {"status": "error", "error": "Invalid URL"}

    os.makedirs(output_dir, exist_ok=True)

    # Snapshot existing mp3s before download
    before = set()
    try:
        for f in os.listdir(output_dir):
            if f.lower().endswith('.mp3'):
                before.add(f)
    except Exception:
        pass

    out_template = os.path.join(output_dir, "{title} - {artist}.{ext}")
    cmd = [
        sys.executable, "-m", "spotdl",
        url,
        "--output", out_template,
        "--format", "mp3",
        "--bitrate", "192k",
    ]
    try:
        _emit({"status": "progress", "percent": 0, "message": "Starting spotdl..."})
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=300)
        if result.returncode != 0:
            err = result.stderr.strip() or "spotdl failed"
            _emit({"status": "progress", "percent": 100, "message": "spotdl error"})
            return {"status": "error", "error": err}

        # Find new mp3 files created by spotdl
        after = set()
        for f in os.listdir(output_dir):
            if f.lower().endswith('.mp3'):
                after.add(f)
        new_files = sorted(after - before)

        if not new_files:
            return {"status": "error", "error": "Downloaded file not found"}

        # Process each new file (extract metadata, append to song_list.txt)
        songs = []
        for fname in new_files:
            filepath = os.path.join(output_dir, fname)
            meta = _finalize_download(filepath, output_dir)
            songs.append({
                "status": "ok",
                "filename": meta.get("filename", fname),
                "title": meta.get("title", ""),
                "artist": meta.get("artist", ""),
                "duration": meta.get("duration", 0),
                "art_path": meta.get("art_path", ""),
            })

        return {
            "status": "ok",
            "message": f"Downloaded {len(songs)} song(s)",
            "songs": songs,
        }

    except FileNotFoundError:
        return {"status": "error", "error": "spotdl not installed. Run: pip install spotdl"}
    except subprocess.TimeoutExpired:
        return {"status": "error", "error": "Download timed out"}
    except Exception as e:
        return {"status": "error", "error": str(e)}


def _finalize_download(filepath: str, output_dir: str) -> dict:
    """Extract metadata after download and return result dict."""
    try:
        from metadata import extract_metadata
        meta = extract_metadata(filepath)
    except Exception:
        meta = {"title": os.path.splitext(os.path.basename(filepath))[0], "artist": "Unknown", "duration": 0.0}

    filename = os.path.basename(filepath)
    duration = meta.get("duration", 0.0)
    dur_str = _fmt_dur(duration)

    # Append to song_list.txt in output_dir
    try:
        from playlist import append_song
        list_path = os.path.join(output_dir, "song_list.txt")
        append_song(list_path, filename, meta.get("title", filename), meta.get("artist", "Unknown"), dur_str)
    except Exception:
        pass

    return {
        "status": "ok",
        "filename": filename,
        "title": meta.get("title", filename),
        "artist": meta.get("artist", "Unknown"),
        "duration": duration,
        "art_path": meta.get("art_path", ""),
    }


def _latest_file(directory: str, ext: str) -> str:
    """Return the most recently modified file with given extension in directory."""
    try:
        files = [
            os.path.join(directory, f)
            for f in os.listdir(directory)
            if f.lower().endswith(ext)
        ]
        if not files:
            return ""
        return max(files, key=os.path.getmtime)
    except Exception:
        return ""


def _fmt_dur(seconds: float) -> str:
    s = int(seconds)
    return f"{s // 60:02d}:{s % 60:02d}"
