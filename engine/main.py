#!/usr/bin/env python3
"""
MusicLe Engine — main.py
Unified Python entry point. Routes JSON actions from Go.
Run with --daemon for persistent playback process.
Run without args for one-shot (reads single JSON from stdin, writes JSON to stdout).
"""
import sys
import json
import os

# Force UTF-8 for stdin/stdout to handle Turkish characters correctly on Windows
if hasattr(sys.stdout, 'reconfigure'):
    sys.stdout.reconfigure(encoding='utf-8')
if hasattr(sys.stdin, 'reconfigure'):
    sys.stdin.reconfigure(encoding='utf-8')

def route(action: dict) -> dict:
    act = action.get("action", "")

    if act == "status":
        from play import player_instance
        return player_instance.status()

    elif act == "play":
        from play import player_instance
        return player_instance.play(action.get("file", ""))

    elif act == "pause":
        from play import player_instance
        return player_instance.pause()

    elif act == "resume":
        from play import player_instance
        return player_instance.resume()

    elif act == "stop":
        from play import player_instance
        return player_instance.stop()

    elif act == "seek":
        from play import player_instance
        return player_instance.seek(float(action.get("value", 0)))

    elif act == "volume":
        from play import player_instance
        return player_instance.set_volume(float(action.get("value", 0.7)))

    elif act == "download_youtube":
        from download import download_youtube
        return download_youtube(action.get("url", ""), action.get("output", "."))

    elif act == "download_spotify":
        from download import download_spotify
        return download_spotify(action.get("url", ""), action.get("output", "."))

    elif act == "add_local":
        from playlist import add_local_file
        return add_local_file(action.get("file", ""), action.get("output", "."))

    elif act == "remove_song":
        from playlist import remove_song
        return remove_song(action.get("file", ""), action.get("path", ""))

    elif act == "update_song":
        from playlist import update_song
        vals = action.get("value", {})
        if not isinstance(vals, dict):
            vals = {}
        return update_song(
            action.get("file", ""), action.get("path", ""),
            title=vals.get("title", ""),
            artist=vals.get("artist", ""),
            duration=vals.get("duration", ""),
        )

    elif act == "metadata":
        from metadata import extract_metadata
        return extract_metadata(action.get("file", ""))

    elif act == "image_ansi":
        from image import image_to_ansi
        return image_to_ansi(action.get("path", ""), int(action.get("value", 20)))

    else:
        return {"status": "error", "error": f"Unknown action: {act}"}


def daemon_loop():
    """Persistent daemon: read JSON lines from stdin, write JSON lines to stdout."""
    # Initialize pygame once for daemon lifetime
    try:
        from play import player_instance
        player_instance.init()
    except Exception as e:
        _emit({"status": "error", "error": f"Init failed: {e}"})

    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            action = json.loads(line)
        except json.JSONDecodeError as e:
            _emit({"status": "error", "error": f"JSON parse: {e}"})
            continue
        try:
            result = route(action)
        except Exception as e:
            result = {"status": "error", "error": str(e)}
        _emit(result)


def oneshot():
    """One-shot: read single JSON from stdin, write JSON to stdout."""
    data = sys.stdin.read().strip()
    if not data:
        _emit({"status": "error", "error": "No input"})
        return
    try:
        action = json.loads(data)
    except json.JSONDecodeError as e:
        _emit({"status": "error", "error": f"JSON parse: {e}"})
        return
    try:
        result = route(action)
    except Exception as e:
        result = {"status": "error", "error": str(e)}
    _emit(result)


def _emit(obj: dict):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()


if __name__ == "__main__":
    # Add engine dir to path so sibling imports work
    sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

    if "--daemon" in sys.argv:
        daemon_loop()
    else:
        oneshot()
