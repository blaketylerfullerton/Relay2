#!/usr/bin/env sh
# install.sh — bootstrap Relay on a new machine.
#
# Clones the source to ~/.relay/src, builds, and installs the `relay` binary.
# After this, keep the machine current with:  relay update
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/blaketylerfullerton/Relay2/main/scripts/install.sh | sh
#   ./scripts/install.sh            # from a local checkout
set -eu

REPO="${RELAY_REPO:-https://github.com/blaketylerfullerton/Relay2.git}"
SRC="${RELAY_SRC:-$HOME/.relay/src}"
BIN_DIR="${RELAY_BIN_DIR:-$HOME/.local/bin}"

command -v git >/dev/null 2>&1 || { echo "install: git is required" >&2; exit 1; }
command -v go  >/dev/null 2>&1 || { echo "install: Go is required (https://go.dev/dl)" >&2; exit 1; }

if [ -d "$SRC/.git" ]; then
  echo "Updating source in $SRC ..."
  git -C "$SRC" pull --ff-only
else
  echo "Cloning $REPO -> $SRC ..."
  mkdir -p "$(dirname "$SRC")"
  git clone "$REPO" "$SRC"
fi

echo "Building ..."
mkdir -p "$BIN_DIR"
( cd "$SRC" && go build -o "$BIN_DIR/relay" . )

echo "Installed relay -> $BIN_DIR/relay"
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo "Note: add $BIN_DIR to your PATH to run 'relay' directly." ;;
esac
echo "Done. Keep this machine current with:  relay update"
