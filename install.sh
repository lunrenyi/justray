#!/bin/sh
# curl -fsSL https://raw.githubusercontent.com/luynrs/justray/main/install.sh | sh
# re-run it to update

set -eu

dir="${JUSTRAY_INSTALL_DIR:-$HOME/.local/bin}"
base="https://github.com/luynrs/justray/releases/latest/download"

say() { echo "install.sh: $*"; }
die() { echo "install.sh: $*" >&2; exit 1; }

case $(uname -s) in
	Linux) os=linux ;;
	Darwin) os=darwin ;;
	*) die "unsupported OS: $(uname -s) (use install.ps1 on Windows)" ;;
esac

case $(uname -m) in
	x86_64 | amd64) arch=amd64 ;;
	arm64 | aarch64) arch=arm64 ;;
	*) die "unsupported arch: $(uname -m)" ;;
esac

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

say "fetching latest release"
line=$(curl -fsSL "$base/checksums.txt" | grep -E "justray_.*_${os}_${arch}\.tar\.gz$" | head -1)
[ -n "$line" ] || die "no release archive for ${os}_${arch}"
archive=${line##* }

say "downloading $archive"
curl -fsSL "$base/$archive" -o "$tmp/$archive"

sum=$(sha256sum "$tmp/$archive" 2>/dev/null || shasum -a 256 "$tmp/$archive")
case "$sum" in
	"${line%% *}"*) ;;
	*) die "checksum mismatch for $archive" ;;
esac

tar -xzf "$tmp/$archive" -C "$tmp"
mkdir -p "$dir"
install -m 755 "$tmp/justray" "$tmp/justrayd" "$dir/"
ln -sf justray "$dir/jray"

say "installed justray, jray, justrayd to $dir"
case ":$PATH:" in
	*":$dir:"*) ;;
	*) say "$dir is not on your PATH, add it to your shell profile" ;;
esac
if pgrep -x justrayd >/dev/null 2>&1; then
	say "justrayd is still running the old build, restart it: pkill justrayd"
fi
