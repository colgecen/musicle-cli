"""
MusicLe Engine — playlist.py
Manages song_list.txt: append, remove, reorder, read.
Also handles local file import (copies mp3 into playlist directory).
"""
import os
import shutil
from datetime import date


SUPPORTED = {".mp3"}


def append_song(list_path: str, filename: str, title: str, artist: str, duration: str):
    """Append a song entry to song_list.txt."""
    today = date.today().isoformat()
    entry = f"{filename}|{title}|{artist}|{today}|{duration}\n"
    with open(list_path, "a", encoding="utf-8") as f:
        f.write(entry)


def read_songs(list_path: str) -> list:
    """Read all songs from song_list.txt. Returns list of dicts."""
    if not os.path.isfile(list_path):
        return []
    songs = []
    with open(list_path, "r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            parts = line.split("|", 4)
            if len(parts) == 5:
                songs.append({
                    "filename": parts[0],
                    "title": parts[1],
                    "artist": parts[2],
                    "date_added": parts[3],
                    "duration": parts[4],
                })
    return songs


def update_song(list_path: str, filename: str, title: str = "", artist: str = "", duration: str = "") -> dict:
    """Update a song entry in song_list.txt by filename."""
    if not os.path.isfile(list_path):
        return {"status": "error", "error": "song_list.txt not found"}
    songs = read_songs(list_path)
    found = False
    for s in songs:
        if s["filename"] == filename:
            if title:
                s["title"] = title
            if artist:
                s["artist"] = artist
            if duration:
                s["duration"] = duration
            found = True
            break
    if not found:
        return {"status": "error", "error": f"Song not found: {filename}"}
    _write_songs(list_path, songs)
    return {"status": "ok", "title": title, "artist": artist}


def remove_song(list_path: str, filename: str) -> dict:
    """Remove a song entry from song_list.txt by filename."""
    if not os.path.isfile(list_path):
        return {"status": "error", "error": "song_list.txt not found"}
    songs = read_songs(list_path)
    original_count = len(songs)
    songs = [s for s in songs if s["filename"] != filename]
    if len(songs) == original_count:
        return {"status": "error", "error": f"Song not found: {filename}"}
    _write_songs(list_path, songs)
    return {"status": "ok", "removed": filename}


def _write_songs(list_path: str, songs: list):
    """Write all songs back to song_list.txt."""
    with open(list_path, "w", encoding="utf-8") as f:
        for s in songs:
            f.write(f"{s['filename']}|{s['title']}|{s['artist']}|{s['date_added']}|{s['duration']}\n")


def _import_single_file(source_path: str, playlist_dir: str) -> dict:
    """Copy a single mp3 into playlist_dir and register it in song_list.txt."""
    ext = os.path.splitext(source_path)[1].lower()
    if ext not in SUPPORTED:
        return {"status": "error", "error": f"Unsupported format: {ext} (only mp3 allowed)"}

    os.makedirs(playlist_dir, exist_ok=True)

    basename = os.path.basename(source_path)
    if not basename.lower().endswith(".mp3"):
        return {"status": "error", "error": "Only mp3 files are supported"}

    dest_path = os.path.join(playlist_dir, basename)
    counter = 1
    while os.path.isfile(dest_path):
        name, _ = os.path.splitext(basename)
        dest_path = os.path.join(playlist_dir, f"{name}_{counter}.mp3")
        counter += 1
    try:
        shutil.copy2(source_path, dest_path)
    except Exception as e:
        return {"status": "error", "error": f"Copy failed: {e}"}

    try:
        from metadata import extract_metadata
        meta = extract_metadata(dest_path)
    except Exception:
        meta = {
            "title": os.path.splitext(os.path.basename(dest_path))[0],
            "artist": "Unknown",
            "duration": 0.0,
        }

    basename = os.path.basename(dest_path)
    duration = meta.get("duration", 0.0)
    s = int(duration)
    dur_str = f"{s // 60:02d}:{s % 60:02d}"

    list_path = os.path.join(playlist_dir, "song_list.txt")
    append_song(list_path, basename, meta.get("title", os.path.splitext(basename)[0]), meta.get("artist", "Unknown"), dur_str)

    return {
        "status": "ok",
        "filename": basename,
        "title": meta.get("title", os.path.splitext(basename)[0]),
        "artist": meta.get("artist", "Unknown"),
        "duration": duration,
        "art_path": meta.get("art_path", ""),
    }


def add_local_file(source_path: str, playlist_dir: str) -> dict:
    """
    Import mp3 file(s) into the playlist directory.
    If source_path is a directory, scan recursively for mp3 files.
    Rejects the entire directory if any non-mp3 audio file is found.
    If source_path is a single file, import just that file.
    Returns structured result dict.
    """
    if os.path.isdir(source_path):
        # First pass: validate all audio files are mp3
        errors = []
        allowed_exts = {".mp4", ".flac", ".m4a", ".aac", ".ogg", ".wav", ".opus"}
        for root, _, files in os.walk(source_path):
            for f in files:
                ext = os.path.splitext(f)[1].lower()
                if ext in allowed_exts:
                    errors.append(f"'{f}' is {ext}, only mp3 allowed")

        if errors:
            return {"status": "error", "error": "Non-mp3 files found:\n" + "\n".join(errors[:5])}

        imported = []
        dir_errors = []
        for root, _, files in os.walk(source_path):
            for f in files:
                ext = os.path.splitext(f)[1].lower()
                if ext == ".mp3":
                    full = os.path.join(root, f)
                    result = _import_single_file(full, playlist_dir)
                    if result["status"] == "ok":
                        imported.append(result["filename"])
                    else:
                        dir_errors.append(f"{f}: {result['error']}")
        msg = f"Imported {len(imported)} file(s)"
        if dir_errors:
            msg += f", {len(dir_errors)} error(s)"
        return {
            "status": "ok",
            "filename": msg,
            "title": msg,
            "artist": "",
            "duration": 0,
            "art_path": "",
        }

    if not os.path.isfile(source_path):
        return {"status": "error", "error": f"File not found: {source_path}"}

    return _import_single_file(source_path, playlist_dir)
