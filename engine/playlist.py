"""
MusicLe Engine — playlist.py
Manages song_list.txt: append, remove, reorder, read.
Also handles local file import (copy + metadata).
"""
import os
import shutil
from datetime import date


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
    """Import a single audio file into the playlist directory."""
    ext = os.path.splitext(source_path)[1].lower()
    allowed = {".mp3", ".mp4", ".flac", ".m4a", ".aac", ".ogg", ".wav", ".opus"}
    if ext not in allowed:
        return {"status": "error", "error": f"Unsupported format: {ext}"}

    os.makedirs(playlist_dir, exist_ok=True)
    filename = os.path.basename(source_path)
    dest_path = os.path.join(playlist_dir, filename)

    if os.path.abspath(source_path) != os.path.abspath(dest_path):
        base, e = os.path.splitext(filename)
        counter = 1
        while os.path.exists(dest_path):
            filename = f"{base}_{counter}{e}"
            dest_path = os.path.join(playlist_dir, filename)
            counter += 1
        shutil.copy2(source_path, dest_path)

    try:
        from metadata import extract_metadata
        meta = extract_metadata(dest_path)
    except Exception:
        meta = {
            "title": os.path.splitext(filename)[0],
            "artist": "Unknown",
            "duration": 0.0,
        }

    duration = meta.get("duration", 0.0)
    s = int(duration)
    dur_str = f"{s // 60:02d}:{s % 60:02d}"

    list_path = os.path.join(playlist_dir, "song_list.txt")
    append_song(list_path, filename, meta.get("title", filename), meta.get("artist", "Unknown"), dur_str)

    return {
        "status": "ok",
        "filename": filename,
        "title": meta.get("title", filename),
        "artist": meta.get("artist", "Unknown"),
        "duration": duration,
        "art_path": meta.get("art_path", ""),
    }


def add_local_file(source_path: str, playlist_dir: str) -> dict:
    """
    Import audio file(s) into the playlist directory.
    If source_path is a directory, scan recursively for audio files.
    If source_path is a single file, import just that file.
    Returns structured result dict.
    """
    if os.path.isdir(source_path):
        imported = []
        errors = []
        allowed = {".mp3", ".mp4", ".flac", ".m4a", ".aac", ".ogg", ".wav", ".opus"}
        for root, _, files in os.walk(source_path):
            for f in files:
                ext = os.path.splitext(f)[1].lower()
                if ext in allowed:
                    full = os.path.join(root, f)
                    result = _import_single_file(full, playlist_dir)
                    if result["status"] == "ok":
                        imported.append(result["filename"])
                    else:
                        errors.append(f"{f}: {result['error']}")
        msg = f"Imported {len(imported)} file(s)"
        if errors:
            msg += f", {len(errors)} error(s)"
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
