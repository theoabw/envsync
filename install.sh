#!/bin/sh

set -eu

repo="theoabw/envsync"
install_dir="${ENVSYNC_INSTALL_DIR:-${HOME}/.local/bin}"
latest_url="https://github.com/${repo}/releases/latest"

fail() {
  printf 'envsync installer: %s\n' "$1" >&2
  exit 1
}

command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v tar >/dev/null 2>&1 || fail "tar is required"

case "$(uname -s)" in
  Darwin) os="darwin" ;;
  Linux) os="linux" ;;
  FreeBSD) os="freebsd" ;;
  *) fail "unsupported operating system: $(uname -s)" ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) fail "unsupported architecture: $(uname -m)" ;;
esac

release_url=$(curl -fsSL -o /dev/null -w '%{url_effective}' "$latest_url")
version=${release_url##*/}
[ -n "$version" ] || fail "could not determine the latest release"

archive="envsync_${version}_${os}_${arch}.tar.gz"
download_base="https://github.com/${repo}/releases/download/${version}"
tmp_dir=$(mktemp -d 2>/dev/null || mktemp -d -t envsync)
trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM

printf 'Downloading envsync %s for %s/%s...\n' "$version" "$os" "$arch"
curl -fsSL "$download_base/$archive" -o "$tmp_dir/$archive"
curl -fsSL "$download_base/checksums.txt" -o "$tmp_dir/checksums.txt"

expected=$(awk -v file="$archive" '$2 == file || $2 == "*" file { print $1; exit }' "$tmp_dir/checksums.txt")
[ -n "$expected" ] || fail "checksum not found for $archive"

if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmp_dir/$archive" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  actual=$(shasum -a 256 "$tmp_dir/$archive" | awk '{print $1}')
else
  fail "sha256sum or shasum is required to verify the download"
fi

[ "$actual" = "$expected" ] || fail "checksum verification failed"

tar -xzf "$tmp_dir/$archive" -C "$tmp_dir"
extracted_dir=${archive%.tar.gz}
binary="$tmp_dir/$extracted_dir/envsync"
[ -f "$binary" ] || fail "envsync binary was not found in the archive"

mkdir -p "$install_dir"
install -m 0755 "$binary" "$install_dir/envsync"

printf 'Installed envsync to %s/envsync\n' "$install_dir"
case ":${PATH}:" in
  *":${install_dir}:"*) ;;
  *)
    printf '\nAdd this directory to your PATH, then restart your shell:\n'
    printf '  export PATH="%s:$PATH"\n' "$install_dir"
    ;;
esac
