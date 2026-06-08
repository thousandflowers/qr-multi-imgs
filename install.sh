#!/usr/bin/env bash
set -euo pipefail

NAME="qr-multi-imgs"
# Switch to the script's own directory so relative paths work
cd "$(dirname "$0")"

printf "📦 Installing %s...\n\n" "$NAME"

# ---- Check prerequisites ----
if ! command -v go &>/dev/null; then
	echo "❌ Go is required but not found."
	echo "   Install it: https://go.dev/dl/"
	exit 1
fi

GO_VERSION=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | head -1)
echo "   Go $GO_VERSION — OK"

# ---- Build ----
echo "   Downloading dependencies..."
go mod download

echo "   Building..."
go build -o "$NAME" .

# ---- Install ----
# Try $HOME/.local/bin first (common on macOS/Linux), fallback /usr/local/bin
if [ -d "$HOME/.local/bin" ] || mkdir -p "$HOME/.local/bin" 2>/dev/null; then
	INSTALL_DIR="$HOME/.local/bin"
	NEED_SUDO=false
else
	INSTALL_DIR="/usr/local/bin"
	NEED_SUDO=true
fi

if [ "$NEED_SUDO" = true ]; then
	echo "   Installing to $INSTALL_DIR (sudo)..."
	sudo mv "$NAME" "$INSTALL_DIR/$NAME"
else
	echo "   Installing to $INSTALL_DIR..."
	mv "$NAME" "$INSTALL_DIR/$NAME"
fi

echo ""
echo "✅ Done! Installed at $INSTALL_DIR/$NAME"

# ---- Verify PATH ----
if command -v "$NAME" &>/dev/null; then
	echo "🎉 Ready. Use it:  $NAME /path/to/images"
else
	echo ""
	echo "⚠️  $INSTALL_DIR is not in your PATH."
	echo "   Add this to your shell config:"
	echo ""
	echo "    export PATH=\"\$HOME/.local/bin:\$PATH\""
	echo ""
	echo "   Then restart your shell and run:  $NAME /path/to/images"
fi
