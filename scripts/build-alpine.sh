#!/bin/sh
# Builds the musl release of tlua inside an Alpine container, for the
# machine's own architecture, into dist/:
#
#   docker run --rm -v "$PWD":/src -w /src alpine:3.22 scripts/build-alpine.sh 0.4.1
#
# It installs the packages the GUI links against and the Go that go.mod
# names, builds with scripts/build-release.sh, and runs what it built.
set -eu

version=${1:?usage: scripts/build-alpine.sh VERSION}

apk add --no-cache -q curl git build-base pkgconf \
  libx11-dev libxext-dev libxft-dev libxinerama-dev libxcursor-dev \
  libxrender-dev libxfixes-dev libxrandr-dev libxi-dev libxxf86vm-dev \
  pango-dev cairo-dev mesa-dev glu-dev wayland-dev libxkbcommon-dev \
  dbus-dev fontconfig-dev freetype-dev harfbuzz-dev alsa-lib-dev >/dev/null

case $(uname -m) in
x86_64) arch=amd64 ;;
aarch64) arch=arm64 ;;
*) echo "no Go for $(uname -m)" >&2; exit 1 ;;
esac
go=$(sed -n 's/^go \([0-9.]*\)$/\1/p' go.mod)
curl -sSL "https://go.dev/dl/go$go.linux-$arch.tar.gz" | tar -C /usr/local -xz
export PATH=/usr/local/go/bin:$PATH
# The checkout belongs to someone else; git would refuse to stamp it.
export GOFLAGS=-buildvcs=false

scripts/build-release.sh "$version"

# A musl build can only be tried where musl is: here.
name="tlua-$version-linux-$arch-musl"
tmp=$(mktemp -d)
tar xzf "dist/$name.tar.gz" -C "$tmp"
"$tmp/$name/tlua" -v
"$tmp/$name/tlua" -e 'assert(require("gui") and (1 + 1 == 2))'
rm -rf "$tmp"
