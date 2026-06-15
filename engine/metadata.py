"""
MusicLe Engine — metadata.py
Audio metadata extraction using mutagen.
Extracts: title, artist, album, duration, and album art.
"""
import os
import hashlib

try:
    from mutagen import File as MutagenFile
    from mutagen.mp3 import MP3
    from mutagen.id3 import ID3, APIC
    from mutagen.flac import FLAC
    from mutagen.mp4 import MP4
    MUTAGEN_AVAILABLE = True
except ImportError:
    MUTAGEN_AVAILABLE = False


def extract_metadata(file_path: str) -> dict:
    """Extract title, artist, album, duration, art_path from an audio file."""
    if not os.path.isfile(file_path):
        return {"status": "error", "error": f"File not found: {file_path}"}

    base = os.path.splitext(os.path.basename(file_path))[0]
    ext = os.path.splitext(file_path)[1].lower()
    fmt_name = _format_name(ext)
    result = {
        "status": "ok",
        "title": base,
        "artist": "Unknown",
        "album": "",
        "duration": 0.0,
        "art_path": "",
        "filename": os.path.basename(file_path),
        "format": fmt_name,
        "sample_rate": 0,
        "bitrate": 0,
    }

    if not MUTAGEN_AVAILABLE:
        return result

    try:
        audio = MutagenFile(file_path)
        if audio is None:
            return result

        # Duration
        if hasattr(audio, "info") and audio.info:
            result["duration"] = float(audio.info.length)
            if hasattr(audio.info, "sample_rate"):
                result["sample_rate"] = int(audio.info.sample_rate)
            if hasattr(audio.info, "bitrate"):
                result["bitrate"] = int(audio.info.bitrate) // 1000

        ext = os.path.splitext(file_path)[1].lower()

        if ext == ".mp3":
            _extract_id3(audio, result, file_path)
        elif ext == ".flac":
            _extract_flac(audio, result, file_path)
        elif ext in (".m4a", ".mp4", ".aac"):
            _extract_mp4(audio, result, file_path)
        else:
            # Generic tag reading
            tags = audio.tags
            if tags:
                result["title"] = str(tags.get("title", [base])[0]) if tags.get("title") else base
                result["artist"] = str(tags.get("artist", ["Unknown"])[0]) if tags.get("artist") else "Unknown"
                result["album"] = str(tags.get("album", [""])[0]) if tags.get("album") else ""

    except Exception as e:
        result["error"] = str(e)

    return result


def _extract_id3(audio, result: dict, file_path: str):
    tags = audio.tags
    if not tags:
        return
    if "TIT2" in tags:
        result["title"] = str(tags["TIT2"])
    if "TPE1" in tags:
        result["artist"] = str(tags["TPE1"])
    if "TALB" in tags:
        result["album"] = str(tags["TALB"])

    # Album art — APIC frame
    for key in tags:
        if key.startswith("APIC"):
            apic = tags[key]
            art_path = _save_art(apic.data, file_path)
            if art_path:
                result["art_path"] = art_path
            break


def _extract_flac(audio, result: dict, file_path: str):
    if audio.tags:
        result["title"] = audio.tags.get("title", [result["title"]])[0]
        result["artist"] = audio.tags.get("artist", ["Unknown"])[0]
        result["album"] = audio.tags.get("album", [""])[0]
    if audio.pictures:
        art_path = _save_art(audio.pictures[0].data, file_path)
        if art_path:
            result["art_path"] = art_path


def _extract_mp4(audio, result: dict, file_path: str):
    tags = audio.tags
    if not tags:
        return
    result["title"] = str(tags.get("\xa9nam", [result["title"]])[0])
    result["artist"] = str(tags.get("\xa9ART", ["Unknown"])[0])
    result["album"] = str(tags.get("\xa9alb", [""])[0])
    if "covr" in tags and tags["covr"]:
        art_path = _save_art(bytes(tags["covr"][0]), file_path)
        if art_path:
            result["art_path"] = art_path


def _save_art(data: bytes, audio_path: str) -> str:
    """Save album art bytes to a .jpg file alongside the audio. Returns path or ''."""
    try:
        h = hashlib.md5(data[:64]).hexdigest()[:8]
        art_dir = os.path.join(os.path.dirname(audio_path), "_art")
        os.makedirs(art_dir, exist_ok=True)
        art_path = os.path.join(art_dir, f"art_{h}.jpg")
        if not os.path.exists(art_path):
            with open(art_path, "wb") as f:
                f.write(data)
        return art_path
    except Exception:
        return ""


def _format_name(ext: str) -> str:
    return {
        ".mp3": "MP3", ".flac": "FLAC", ".m4a": "AAC",
        ".mp4": "AAC", ".aac": "AAC", ".ogg": "OGG",
        ".wav": "WAV", ".opus": "Opus",
    }.get(ext, ext.lstrip(".").upper() or "Unknown")


def get_duration(file_path: str) -> float:
    """Quickly return file duration in seconds."""
    if not MUTAGEN_AVAILABLE:
        return 0.0
    try:
        audio = MutagenFile(file_path)
        if audio and audio.info:
            return float(audio.info.length)
    except Exception:
        pass
    return 0.0
