#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-1.0.0}"

GO=$(command -v go 2>/dev/null || command -v /usr/local/go/bin/go 2>/dev/null || command -v /usr/lib/go/bin/go 2>/dev/null || true)
if [ -z "$GO" ]; then
  echo "Go not found. Install: sudo dnf install golang"
  exit 1
fi

echo "Select target OS:"
echo "1) Linux"
echo "2) Windows"
echo "3) macOS"
read -p "Choice [1-3]: " os_choice

case "$os_choice" in
  1)
    echo "Select format:"
    echo "1) AppImage"
    echo "2) RPM"
    echo "3) tar.gz"
    echo "4) Binary only"
    read -p "Choice [1-4]: " fmt

    mkdir -p build
    echo "==> Building binary..."
    CGO_ENABLED=1 $GO build -ldflags="-s -w -X main.version=$VERSION" -o build/muscle-cli .

    case "$fmt" in
      1)
        echo "==> Building AppImage..."
        APPDIR=build/MusicLe.AppDir
        mkdir -p $APPDIR/usr/bin $APPDIR/usr/share/applications $APPDIR/usr/share/icons/hicolor/1024x1024/apps
        cp build/muscle-cli $APPDIR/usr/bin/
        cp assets/MusicLe.png $APPDIR/.DirIcon
        cp assets/MusicLe.png $APPDIR/MusicLe.png
        cp assets/MusicLe.png $APPDIR/usr/share/icons/hicolor/1024x1024/apps/musicle-cli.png

        cat > $APPDIR/AppRun << 'APPRUN'
#!/bin/bash
HERE="$(dirname "$(readlink -f "$0")")"
if [ -t 0 ]; then
  exec "$HERE/usr/bin/musicle-cli" "$@"
else
  for term in x-terminal-emulator gnome-terminal konsole xterm alacritty kitty; do
    if command -v "$term" &>/dev/null; then
      exec "$term" -e "$HERE/usr/bin/musicle-cli" "$@"
    fi
  done
  exec "$HERE/usr/bin/musicle-cli" "$@"
fi
APPRUN
        chmod +x $APPDIR/AppRun

        cat > $APPDIR/musicle-cli.desktop << 'DESKTOP'
[Desktop Entry]
Name=MusicLe
Exec=musicle-cli
Icon=musicle-cli
Terminal=false
Type=Application
Categories=AudioVideo;Audio;Music;Player;
StartupNotify=false
DESKTOP
        cp $APPDIR/musicle-cli.desktop $APPDIR/usr/share/applications/
        wget -q https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage -O build/appimagetool
        chmod +x build/appimagetool
        cd build
        ./appimagetool --appimage-extract > /dev/null 2>&1 || true
        ./squashfs-root/AppRun -n MusicLe.AppDir MusicleCLI.AppImage
        rm -rf squashfs-root appimagetool
        cd ..
        echo "==> build/MusicleCLI.AppImage"
        ;;
      2)
        echo "==> Building RPM..."
        mkdir -p build/rpm/{BUILD,RPMS,SOURCES,SPECS,SRPMS}
        cat > build/rpm/SPECS/musicle-cli.spec << SPEC
Name: musicle-cli
Version: $VERSION
Release: 1
Summary: Terminal music player
License: MIT
BuildArch: x86_64
%description
MusicLe - terminal music player
%install
mkdir -p %{buildroot}/usr/local/bin
cp %{_sourcedir}/muscle-cli %{buildroot}/usr/local/bin/
%files
/usr/local/bin/muscle-cli
SPEC
        cp build/muscle-cli build/rpm/SOURCES/
        rpmbuild --define "_topdir $(pwd)/build/rpm" -bb build/rpm/SPECS/musicle-cli.spec
        cp build/rpm/RPMS/x86_64/*.rpm build/
        echo "==> build/*.rpm"
        ;;
      3)
        echo "==> Packing tar.gz..."
        mkdir -p build/muscle-cli_Linux_x86_64
        cp build/muscle-cli build/muscle-cli_Linux_x86_64/
        cp README.md build/muscle-cli_Linux_x86_64/ 2>/dev/null || true
        cd build && tar czf muscle-cli_Linux_x86_64.tar.gz muscle-cli_Linux_x86_64 && cd ..
        echo "==> build/muscle-cli_Linux_x86_64.tar.gz"
        ;;
      4)
        echo "==> build/muscle-cli"
        ;;
    esac
    ;;
  2)
    echo "Select format:"
    echo "1) .exe (portable)"
    echo "2) .exe + .zip"
    read -p "Choice [1-2]: " fmt
    mkdir -p build
    echo "==> Building..."
    GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $GO build -ldflags="-s -w -X main.version=$VERSION" -o build/muscle-cli.exe .
    if [ "$fmt" = "2" ]; then
      mkdir -p build/muscle-cli_Windows_x86_64
      cp build/muscle-cli.exe build/muscle-cli_Windows_x86_64/
      cd build && zip -r muscle-cli_Windows_x86_64.zip muscle-cli_Windows_x86_64 && cd ..
      echo "==> build/muscle-cli_Windows_x86_64.zip"
    else
      echo "==> build/muscle-cli.exe"
    fi
    ;;
  3)
    echo "Select format:"
    echo "1) tar.gz"
    echo "2) Binary only"
    read -p "Choice [1-2]: " fmt
    mkdir -p build
    echo "==> Building..."
    GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 $GO build -ldflags="-s -w -X main.version=$VERSION" -o build/muscle-cli-darwin .
    if [ "$fmt" = "1" ]; then
      mkdir -p build/muscle-cli_macOS_x86_64
      cp build/muscle-cli-darwin build/muscle-cli_macOS_x86_64/muscle-cli
      cp README.md build/muscle-cli_macOS_x86_64/ 2>/dev/null || true
      cd build && tar czf muscle-cli_macOS_x86_64.tar.gz muscle-cli_macOS_x86_64 && cd ..
      echo "==> build/muscle-cli_macOS_x86_64.tar.gz"
    else
      echo "==> build/muscle-cli-darwin"
    fi
    ;;
esac