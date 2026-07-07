#!/usr/bin/env bash
set -euo pipefail

GO=$(command -v go 2>/dev/null || command -v /usr/local/go/bin/go 2>/dev/null || command -v /usr/lib/go/bin/go 2>/dev/null || true)
if [ -z "$GO" ]; then
  echo "Go not found. Install: sudo dnf install golang"
  exit 1
fi

VERSION="${1:-1.0.0}"

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
    echo "==> Building..."
    CGO_ENABLED=1 $GO build -ldflags="-s -w -X main.version=$VERSION" -o build/muscle-cli .
    case "$fmt" in
      1)
        echo "==> Building AppImage..."
        APPDIR=build/MusicLe.AppDir
        mkdir -p $APPDIR/usr/bin $APPDIR/usr/share/applications $APPDIR/usr/share/icons/hicolor/1024x1024/apps
        cp build/muscle-cli $APPDIR/usr/bin/
        cp assets/MusicLe.png $APPDIR/io.anomalyco.musicle-cli.png
        cp assets/MusicLe.png $APPDIR/usr/share/icons/hicolor/1024x1024/apps/
        cat > $APPDIR/io.anomalyco.musicle-cli.desktop << 'DESKTOP'
[Desktop Entry]
Name=MusicLe
Exec=musicle-cli
Icon=io.anomalyco.musicle-cli
Terminal=true
Type=Application
Categories=Audio;Music;Player;
DESKTOP
        cp $APPDIR/io.anomalyco.musicle-cli.desktop $APPDIR/usr/share/applications/
        wget -q https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage -O build/appimagetool
        chmod +x build/appimagetool
        ./build/appimagetool -n $APPDIR "build/MusicleCLI.AppImage"
        echo "==> build/MusicleCLI.AppImage"
        ;;
      2)
        echo "==> Building RPM..."
        sudo dnf install -y rpm-build 2>/dev/null || true
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
    echo "3) .msi"
    read -p "Choice [1-3]: " fmt
    mkdir -p build
    echo "==> Building..."
    GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $GO build -ldflags="-s -w -X main.version=$VERSION" -o build/muscle-cli.exe .
    case "$fmt" in
      1|2)
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
        echo "==> Building MSI..."
        if command -v wix 2>/dev/null || command -v candle 2>/dev/null; then
          echo "WiX not available, creating .exe only"
        fi
        echo "==> build/muscle-cli.exe (MSI requires WiX on Windows)"
        ;;
    esac
    ;;
  3)
    echo "Select format:"
    echo "1) tar.gz"
    echo "2) Binary only"
    read -p "Choice [1-2]: " fmt
    mkdir -p build
    echo "==> Building..."
    GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 $GO build -ldflags="-s -w -X main.version=$VERSION" -o build/muscle-cli-darwin .
    case "$fmt" in
      1)
        mkdir -p build/muscle-cli_macOS_x86_64
        cp build/muscle-cli-darwin build/muscle-cli_macOS_x86_64/muscle-cli
        cp README.md build/muscle-cli_macOS_x86_64/ 2>/dev/null || true
        cd build && tar czf muscle-cli_macOS_x86_64.tar.gz muscle-cli_macOS_x86_64 && cd ..
        echo "==> build/muscle-cli_macOS_x86_64.tar.gz"
        ;;
      2)
        echo "==> build/muscle-cli-darwin"
        ;;
    esac
    ;;
esac