#!/bin/sh
# Download the latest eerox release binary for this OS/arch into a bin dir on PATH.
# Usage: curl -fsSL https://raw.githubusercontent.com/laurenschristian/eerox/main/scripts/install.sh | sh
set -e

REPO="laurenschristian/eerox"
BIN="eerox"
os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo "unsupported arch: $arch" >&2; exit 1 ;;
esac
case "$os" in
  darwin) asset="${BIN}_Darwin_all.tar.gz" ;;
  linux)  asset="${BIN}_Linux_${arch}.tar.gz" ;;
  *) echo "unsupported OS: $os (use go install or a release archive)" >&2; exit 1 ;;
esac

tag=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep -o '"tag_name": *"[^"]*"' | head -1 | cut -d'"' -f4)
[ -n "$tag" ] || { echo "could not find latest release" >&2; exit 1; }
url="https://github.com/${REPO}/releases/download/${tag}/${asset}"

dir="${HOME}/.local/bin"
[ -w /usr/local/bin ] && dir=/usr/local/bin
mkdir -p "$dir"
tmp=$(mktemp -d)
echo "downloading ${BIN} ${tag} (${os}/${arch})..."
curl -fsSL "$url" | tar -xz -C "$tmp"
install -m 0755 "$tmp/${BIN}" "$dir/${BIN}"
rm -rf "$tmp"
echo "installed to $dir/${BIN}"
command -v "$BIN" >/dev/null 2>&1 || echo "note: add $dir to your PATH"
