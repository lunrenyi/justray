#!/bin/sh
# curl -fsSL https://raw.githubusercontent.com/luynrs/justray/main/install.sh | sh
set -eu

repo="luynrs/justray"
dir="${JUSTRAY_INSTALL_DIR:-$HOME/.local/bin}"

os=$(uname -s)
case "$os" in
	Linux) os=linux ;;
	Darwin) os=darwin ;;
	*) echo "install.sh: unsupported OS: $os (use install.ps1 on Windows)" >&2; exit 1 ;;
esac

arch=$(uname -m)
case "$arch" in
	x86_64|amd64) arch=amd64 ;;
	arm64|aarch64) arch=arm64 ;;
	*) echo "install.sh: unsupported arch: $arch" >&2; exit 1 ;;
esac

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "install.sh: fetching latest release..."
curl -fsSL "https://github.com/$repo/releases/latest/download/checksums.txt" -o "$tmp/checksums.txt"

archive=$(grep -o "justray_[^ ]*_${os}_${arch}\.tar\.gz" "$tmp/checksums.txt" | head -1)
if [ -z "$archive" ]; then
	echo "install.sh: no release archive for ${os}_${arch}" >&2
	exit 1
fi

echo "install.sh: downloading $archive..."
curl -fsSL "https://github.com/$repo/releases/latest/download/$archive" -o "$tmp/$archive"

checksum=$(grep "$archive\$" "$tmp/checksums.txt" | cut -d' ' -f1)
got=$(sha256sum "$tmp/$archive" 2>/dev/null | cut -d' ' -f1 || shasum -a 256 "$tmp/$archive" | cut -d' ' -f1)
if [ "$checksum" != "$got" ]; then
	echo "install.sh: checksum mismatch for $archive" >&2
	exit 1
fi

tar -xzf "$tmp/$archive" -C "$tmp"
mkdir -p "$dir"
install -m 755 "$tmp/justray" "$tmp/justrayd" "$dir/"
ln -sf justray "$dir/jray"

echo "install.sh: installed justray, jray, justrayd to $dir"
case ":$PATH:" in
	*":$dir:"*) ;;
	*) echo "install.sh: $dir is not on your PATH, add it to your shell profile" ;;
esac
