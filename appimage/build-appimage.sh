#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-dev}"
OUTPUT="musicle-cli_${VERSION}_x86_64.AppImage"
APPDIR="musicle-cli-x86_64.AppDir"

echo "==> Building binary..."
mkdir -p "$APPDIR/usr/bin"
CGO_ENABLED=1 go build \
  -ldflags="-s -w -X main.version=$VERSION" \
  -o "$APPDIR/usr/bin/musicle-cli" .

echo "==> Setting up AppDir..."
mkdir -p "$APPDIR/usr/share/applications"
mkdir -p "$APPDIR/usr/share/icons/hicolor/1024x1024/apps"
mkdir -p "$APPDIR/usr/share/metainfo"

cp appimage/io.anomalyco.musicle-cli.desktop "$APPDIR/usr/share/applications/"
cp appimage/io.anomalyco.musicle-cli.desktop "$APPDIR/"
cp assets/MusicLe.png "$APPDIR/usr/share/icons/hicolor/1024x1024/apps/io.anomalyco.musicle-cli.png"
cp assets/MusicLe.png "$APPDIR/io.anomalyco.musicle-cli.png"
cp appimage/io.anomalyco.musicle-cli.appdata.xml "$APPDIR/usr/share/metainfo/"

echo "==> Validating desktop file..."
desktop-file-validate "$APPDIR/io.anomalyco.musicle-cli.desktop"

echo "==> Fetching appimagetool..."
if [ ! -f appimagetool ]; then
  wget -q https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage -O appimagetool
  chmod +x appimagetool
fi

echo "==> Building AppImage..."
./appimagetool -n "$APPDIR" "$OUTPUT"

echo "==> Done: $OUTPUT"