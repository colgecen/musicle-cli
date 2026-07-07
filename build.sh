#!/usr/bin/env bash
set -euo pipefail

echo "Select build format:"
echo "1) AppImage (portable)"
echo "2) RPM (Fedora package)"
echo "3) Binary only (./muscle-cli)"
read -p "Choice [1-3]: " choice

VERSION="${1:-1.0.0}"

echo "==> Building binary..."
mkdir -p build
CGO_ENABLED=1 go build -ldflags="-s -w -X main.version=$VERSION" -o build/muscle-cli .

case "$choice" in
  1)
    echo "==> Building AppImage..."
    mkdir -p build/MusicLe.AppDir/usr/bin
    mkdir -p build/MusicLe.AppDir/usr/share/applications
    mkdir -p build/MusicLe.AppDir/usr/share/icons/hicolor/1024x1024/apps
    cp build/muscle-cli build/MusicLe.AppDir/usr/bin/
    cp assets/MusicLe.png build/MusicLe.AppDir/io.anomalyco.musicle-cli.png
    cp assets/MusicLe.png build/MusicLe.AppDir/usr/share/icons/hicolor/1024x1024/apps/

    cat > build/MusicLe.AppDir/io.anomalyco.musicle-cli.desktop << 'DESKTOP'
[Desktop Entry]
Name=MusicLe
Exec=musicle-cli
Icon=io.anomalyco.musicle-cli
Terminal=true
Type=Application
Categories=Audio;Music;Player;
DESKTOP
    cp build/MusicLe.AppDir/io.anomalyco.musicle-cli.desktop build/MusicLe.AppDir/usr/share/applications/

    wget -q https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage -O build/appimagetool
    chmod +x build/appimagetool
    ./build/appimagetool -n build/MusicLe.AppDir "build/MusicleCLI.AppImage"
    echo "==> Done: build/MusicleCLI.AppImage"
    ;;
  2)
    echo "==> Building RPM..."
    sudo dnf install -y rpm-build
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
    echo "==> Done: build/*.rpm"
    ;;
  3)
    echo "==> Done: build/muscle-cli"
    ;;
esac