#!/bin/sh
# Builds tlua for the machine it runs on and packs it for a release, into
# dist/: tlua-VERSION-OS-ARCH.tar.gz (.zip on Windows), with the GUI. On
# Linux it also packs tlua-VERSION-linux-ARCH-static.tar.gz, built without
# cgo: one file that runs anywhere, everything but the gui module. On a
# musl Linux, Alpine's, it packs tlua-VERSION-linux-ARCH-musl.tar.gz, with
# the GUI, built with -tags musl (see internal/gui/muslcompat);
# scripts/build-alpine.sh runs it in an Alpine container.
#
#   scripts/build-release.sh 0.3.2
#
# The GUI needs cgo and the C++ toolchain go-fltk's libraries were built
# with: Xcode's command line tools on macOS, gcc and the X11, Wayland and
# OpenGL development packages on Linux (.github/workflows/release.yml lists
# them), MinGW-w64 on Windows.
set -eu

version=${1:?usage: scripts/build-release.sh VERSION}
os=$(go env GOOS)
arch=$(go env GOARCH)
mkdir -p dist

# pack DIR: the folder as an archive beside it, named after it.
pack() {
  (
    cd dist
    if [ "$os" = windows ]; then
      7z a -tzip -bso0 "$1.zip" "$1" >/dev/null
    else
      tar czf "$1.tar.gz" "$1"
    fi
    rm -rf "$1"
  )
}

# build NAME CGO [LDFLAGS [TAGS]]: one build into dist/NAME, packed.
build() {
  name=$1 cgo=$2 extra=${3:-} tags=${4:-}
  exe=tlua
  [ "$os" = windows ] && exe=tlua.exe
  rm -rf "dist/$name"
  mkdir -p "dist/$name"
  CGO_ENABLED=$cgo go build -trimpath -tags "$tags" -ldflags "-s -w $extra" -o "dist/$name/$exe" ./cmd/tlua
  cp README.md "dist/$name/"
  mkdir -p "dist/$name/docs"
  cp docs/*.md "dist/$name/docs/"
  cp -R examples "dist/$name/"
  pack "$name"
  echo "built dist/$name"
}

case "$os" in
windows)
  # MinGW's C++ runtime goes into the executable, so it needs no DLLs
  # beside it. go-fltk links with -mwindows, which would make tlua a
  # windowed program with no console: no REPL, no editor, no output in cmd.
  # The console subsystem, asked for after it, wins.
  build "tlua-$version-$os-$arch" 1 "-extldflags '-static -Wl,--subsystem,console'"
  ;;
linux)
  if ldd --version 2>&1 | grep -qi musl; then
    build "tlua-$version-$os-$arch-musl" 1 "" musl
  else
    build "tlua-$version-$os-$arch" 1
    build "tlua-$version-$os-$arch-static" 0
  fi
  ;;
*)
  build "tlua-$version-$os-$arch" 1
  ;;
esac
