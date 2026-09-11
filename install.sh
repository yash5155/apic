#!/bin/sh
# apic installer — downloads the right prebuilt binary for your OS/arch from the
# latest GitHub release and drops it on your PATH.
#
#   curl -fsSL https://raw.githubusercontent.com/yash5155/apic/main/install.sh | bash
#
# Environment overrides:
#   APIC_INSTALL_DIR   where to install (default: $HOME/.local/bin)
#   APIC_VERSION       a specific tag to install, e.g. v0.1.0 (default: latest)
set -eu

REPO="yash5155/apic"
BIN="apic"

info() { printf '%s\n' "$*"; }
err() { printf 'error: %s\n' "$*" >&2; exit 1; }

# --- detect OS ---------------------------------------------------------------
ext=""      # asset/binary suffix (.exe on Windows)
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$os" in
  linux) os="linux" ;;
  darwin) os="darwin" ;;
  mingw* | msys* | cygwin* | windows*) os="windows"; ext=".exe" ;;
  *) err "unsupported OS '$os'. Download manually: https://github.com/$REPO/releases" ;;
esac

# --- detect architecture -----------------------------------------------------
arch="$(uname -m)"
case "$arch" in
  x86_64 | amd64) arch="amd64" ;;
  aarch64 | arm64) arch="arm64" ;;
  *) err "unsupported architecture '$arch'. Download manually: https://github.com/$REPO/releases" ;;
esac

asset="apic_${os}_${arch}${ext}"
binfile="${BIN}${ext}"

# --- resolve download URL (latest or a pinned version) -----------------------
version="${APIC_VERSION:-latest}"
if [ "$version" = "latest" ]; then
  url="https://github.com/$REPO/releases/latest/download/${asset}"
else
  url="https://github.com/$REPO/releases/download/${version}/${asset}"
fi

# --- choose an install directory (no sudo needed by default) ------------------
# Git Bash already has $HOME/bin on PATH, so prefer it on Windows.
default_dir="$HOME/.local/bin"
[ "$os" = "windows" ] && default_dir="$HOME/bin"
dir="${APIC_INSTALL_DIR:-$default_dir}"
mkdir -p "$dir" || err "could not create install dir: $dir"

# --- download ----------------------------------------------------------------
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT INT TERM

info "Downloading ${asset} (${version})..."
if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$url" -o "$tmp" || err "download failed: $url"
elif command -v wget >/dev/null 2>&1; then
  wget -qO "$tmp" "$url" || err "download failed: $url"
else
  err "need curl or wget installed"
fi

[ -s "$tmp" ] || err "downloaded file is empty; no release asset for ${os}/${arch} yet?"

chmod +x "$tmp"
mv "$tmp" "$dir/$binfile"
trap - EXIT INT TERM

info "Installed ${binfile} -> ${dir}/${binfile}"

# --- PATH hint ---------------------------------------------------------------
case ":$PATH:" in
  *":$dir:"*)
    info "Run: ${BIN} --help"
    ;;
  *)
    info ""
    info "${dir} is not on your PATH. Add it, then reopen your shell:"
    info "  echo 'export PATH=\"${dir}:\$PATH\"' >> ~/.bashrc && source ~/.bashrc"
    info ""
    info "Or run it directly: ${dir}/${binfile} --help"
    ;;
esac
