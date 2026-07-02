#!/usr/bin/env sh
set -eu

repo="${REPO:-AliSayyah/hardcover-goodreads}"
version="${VERSION:-latest}"
bin="${BIN:-hardcover-goodreads}"
bin_dir="${BIN_DIR:-$HOME/.local/bin}"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m | tr '[:upper:]' '[:lower:]')"

case "$os" in
  linux|darwin) ;;
  *) echo "unsupported OS: $os" >&2; exit 1 ;;
esac

case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) echo "unsupported architecture: $arch" >&2; exit 1 ;;
esac

archive="${bin}_${os}_${arch}.tar.gz"
base="https://github.com/${repo}/releases"
if [ "$version" = "latest" ]; then
  url="${base}/latest/download/${archive}"
else
  url="${base}/download/${version}/${archive}"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

mkdir -p "$bin_dir"
curl -fsSL "$url" -o "$tmp/$archive"
tar -xzf "$tmp/$archive" -C "$tmp"
install -m 0755 "$tmp/$bin" "$bin_dir/$bin"

echo "installed $bin to $bin_dir/$bin"
