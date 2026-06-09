#!/usr/bin/env sh
# install.sh — install Relay on a machine.
#
# Default: downloads the prebuilt `relay` binary for this OS/arch from the
# rolling "latest" GitHub Release (rebuilt on every push to main). No Go, no
# source checkout, no build.
#
#   curl -fsSL https://raw.githubusercontent.com/blaketylerfullerton/Relay2/main/scripts/install.sh | sh
#
# After installing, keep the machine current with:  relay update
#
# Set RELAY_FROM_SOURCE=1 to clone + build instead (needs git and Go) — useful
# on a machine you also develop on.
set -eu

REPO="${RELAY_REPO:-blaketylerfullerton/Relay2}"
BIN_DIR="${RELAY_BIN_DIR:-$HOME/.local/bin}"

# Map uname to the GOOS/GOARCH names used in release asset filenames.
os="$(uname -s)"
case "$os" in
  Darwin) os="darwin" ;;
  Linux)  os="linux" ;;
  *) echo "install: unsupported OS '$os'" >&2; exit 1 ;;
esac
arch="$(uname -m)"
case "$arch" in
  arm64|aarch64) arch="arm64" ;;
  x86_64|amd64)  arch="amd64" ;;
  *) echo "install: unsupported arch '$arch'" >&2; exit 1 ;;
esac

mkdir -p "$BIN_DIR"

install_from_source() {
  SRC="${RELAY_SRC:-$HOME/.relay/src}"
  command -v git >/dev/null 2>&1 || { echo "install: git is required" >&2; exit 1; }
  command -v go  >/dev/null 2>&1 || { echo "install: Go is required (https://go.dev/dl)" >&2; exit 1; }
  if [ -d "$SRC/.git" ]; then
    echo "Updating source in $SRC ..."
    git -C "$SRC" pull --ff-only
  else
    echo "Cloning https://github.com/$REPO -> $SRC ..."
    mkdir -p "$(dirname "$SRC")"
    git clone "https://github.com/$REPO" "$SRC"
  fi
  echo "Building ..."
  ( cd "$SRC" && go build -o "$BIN_DIR/relay" . )
}

install_from_release() {
  asset="relay_${os}_${arch}"
  url="https://github.com/$REPO/releases/latest/download/$asset"
  echo "Downloading $asset from latest release ..."
  if command -v curl >/dev/null 2>&1; then
    curl -fSL "$url" -o "$BIN_DIR/relay"
  elif command -v wget >/dev/null 2>&1; then
    wget -O "$BIN_DIR/relay" "$url"
  else
    echo "install: need curl or wget" >&2; exit 1
  fi
  chmod +x "$BIN_DIR/relay"
}

if [ "${RELAY_FROM_SOURCE:-0}" = "1" ]; then
  install_from_source
else
  install_from_release
fi

echo "Installed relay -> $BIN_DIR/relay"
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo "Note: add $BIN_DIR to your PATH to run 'relay' directly." ;;
esac
echo "Done. Keep this machine current with:  relay update"
