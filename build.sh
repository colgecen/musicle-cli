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
    CGO_ENABLED=1 $GO build -ldflags="-s -w -X main.version=$VERSION" -o build/musicle-cli .

    case "$fmt" in
      1)
        echo "==> Building AppImage..."
        APPDIR=build/MusicLe.AppDir
        mkdir -p $APPDIR/usr/bin $APPDIR/usr/lib $APPDIR/usr/share/applications $APPDIR/usr/share/icons/hicolor/1024x1024/apps

        echo "==> Bundling shared libraries..."
        for lib in $(ldd build/musicle-cli 2>/dev/null | grep '=> /' | awk '{print $3}' | sort -u); do
            case "$lib" in
                */libc.so.*|*/libpthread.so.*|*/libm.so.*|*/libdl.so.*|*/librt.so.*|*/libresolv.so.*|*/libnss_*|*/libutil.so.*|*/libgcc_s.so.*|*/libstdc++.so.*|*/ld-linux*|linux-vdso*)
                    ;;
                *)
                    cp -L "$lib" $APPDIR/usr/lib/
                    echo "  bundled: $lib"
                    ;;
            esac
        done

        echo "==> Downloading yt-dlp..."
        YTDLP_URL="https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux"
        if command -v curl &>/dev/null; then
            curl -sL "$YTDLP_URL" -o $APPDIR/usr/bin/yt-dlp
        elif command -v wget &>/dev/null; then
            wget -q "$YTDLP_URL" -O $APPDIR/usr/bin/yt-dlp
        fi
        if [ -f $APPDIR/usr/bin/yt-dlp ]; then
            chmod +x $APPDIR/usr/bin/yt-dlp
            # Verify it works
            if $APPDIR/usr/bin/yt-dlp --version >/dev/null 2>&1; then
                echo "  yt-dlp bundled: $($APPDIR/usr/bin/yt-dlp --version)"
            else
                echo "  WARNING: yt-dlp download appears broken, removing"
                rm -f $APPDIR/usr/bin/yt-dlp
            fi
        else
            echo "  WARNING: yt-dlp download failed, will try system yt-dlp at runtime"
        fi

        cp build/musicle-cli $APPDIR/usr/bin/
        cp assets/MusicLe.png $APPDIR/.DirIcon
        cp assets/MusicLe.png $APPDIR/musicle-cli.png
        cp assets/MusicLe.png $APPDIR/usr/share/icons/hicolor/1024x1024/apps/musicle-cli.png

        cat > $APPDIR/AppRun << 'APPRUN'
#!/bin/bash
HERE="$(dirname "$(readlink -f "$0")")"
export LD_LIBRARY_PATH="$HERE/usr/lib:$LD_LIBRARY_PATH"
export APPDIR="$HERE"
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
Terminal=true
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
        rm -rf squashfs-root appimagetool MusicLe.AppDir musicle-cli
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
cp %{_sourcedir}/musicle-cli %{buildroot}/usr/local/bin/
%files
/usr/local/bin/musicle-cli
SPEC
        cp build/musicle-cli build/rpm/SOURCES/
        rpmbuild --define "_topdir $(pwd)/build/rpm" -bb build/rpm/SPECS/musicle-cli.spec
        cp build/rpm/RPMS/x86_64/*.rpm build/
        echo "==> build/*.rpm"
        ;;
      3)
        echo "==> Packing tar.gz..."
        mkdir -p build/musicle-cli_Linux_x86_64
        cp build/musicle-cli build/musicle-cli_Linux_x86_64/
        cp README.md build/musicle-cli_Linux_x86_64/ 2>/dev/null || true
        cd build && tar czf musicle-cli_Linux_x86_64.tar.gz musicle-cli_Linux_x86_64 && cd ..
        echo "==> build/musicle-cli_Linux_x86_64.tar.gz"
        ;;
      4)
        echo "==> build/musicle-cli"
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
    GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $GO build -ldflags="-s -w -X main.version=$VERSION" -o build/musicle-cli.exe .
    if [ "$fmt" = "2" ]; then
      mkdir -p build/musicle-cli_Windows_x86_64
      cp build/musicle-cli.exe build/musicle-cli_Windows_x86_64/
      cd build && zip -r musicle-cli_Windows_x86_64.zip musicle-cli_Windows_x86_64 && cd ..
      echo "==> build/musicle-cli_Windows_x86_64.zip"
    else
      echo "==> build/musicle-cli.exe"
    fi
    ;;
  3)
    echo "Select format:"
    echo "1) tar.gz"
    echo "2) Binary only"
    read -p "Choice [1-2]: " fmt
    mkdir -p build
    echo "==> Building..."
    GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 $GO build -ldflags="-s -w -X main.version=$VERSION" -o build/musicle-cli-darwin .
    if [ "$fmt" = "1" ]; then
      mkdir -p build/musicle-cli_macOS_x86_64
      cp build/musicle-cli-darwin build/musicle-cli_macOS_x86_64/musicle-cli
      cp README.md build/musicle-cli_macOS_x86_64/ 2>/dev/null || true
      cd build && tar czf musicle-cli_macOS_x86_64.tar.gz musicle-cli_macOS_x86_64 && cd ..
      echo "==> build/musicle-cli_macOS_x86_64.tar.gz"
    else
      echo "==> build/musicle-cli-darwin"
    fi
    ;;
esac
