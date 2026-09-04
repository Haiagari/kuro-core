#!/usr/bin/env sh
# Kuro — install script
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/Haiagari/kuro/main/scripts/install.sh | sh
#
# To pin a specific version:
#   curl -sSL https://raw.githubusercontent.com/Haiagari/kuro/main/scripts/install.sh | sh -s -- v0.1.0
#
# Environment variables:
#   KURO_INSTALL_DIR  — install destination (default: /usr/local/bin)
#   KURO_VERSION      — version to install (default: latest release)
#
# This script:
#   1. Detects OS and architecture
#   2. Downloads the correct binary archive from GitHub Releases
#   3. Verifies the SHA-256 checksum
#   4. Installs to /usr/local/bin/kuro

set -eu

# ── Config ──────────────────────────────────────────────────
REPO="Haiagari/kuro"
INSTALL_DIR="${KURO_INSTALL_DIR:-/usr/local/bin}"
VERSION="${KURO_VERSION:-${1:-}}"

# ── Colors ──────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info()  { printf "${GREEN}%s${NC}\n" "$*"; }
warn()  { printf "${YELLOW}%s${NC}\n" "$*"; }
error() { printf "${RED}%s${NC}\n" "$*"; exit 1; }

# ── Detect OS/arch ──────────────────────────────────────────
detect_os() {
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  case "$os" in
    linux)  echo "linux" ;;
    darwin) echo "darwin" ;;
    *)      error "Unsupported OS: $os (only linux and darwin are supported)" ;;
  esac
}

detect_arch() {
  arch=$(uname -m)
  case "$arch" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *)            error "Unsupported architecture: $arch (only amd64 and arm64 are supported)" ;;
  esac
}

# ── Resolve version ─────────────────────────────────────────
resolve_version() {
  if [ -n "$VERSION" ]; then
    # Strip leading v if present
    VERSION=$(echo "$VERSION" | sed 's/^v//')
    echo "v${VERSION}"
    return
  fi

  # Fetch latest release from GitHub API
  url="https://api.github.com/repos/${REPO}/releases/latest"
  if command -v curl >/dev/null 2>&1; then
    tag=$(curl -sL "$url" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": "//' | sed 's/".*//')
  elif command -v wget >/dev/null 2>&1; then
    tag=$(wget -qO- "$url" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": "//' | sed 's/".*//')
  else
    error "Neither curl nor wget found. Install one of them and try again."
  fi

  if [ -z "$tag" ] || [ "$tag" = "null" ]; then
    error "Could not determine latest release from GitHub API.
  Try specifying a version: curl ... | sh -s -- v0.1.0"
  fi

  echo "$tag"
}

# ── Download ────────────────────────────────────────────────
download() {
  url="$1"
  dest="$2"

  if command -v curl >/dev/null 2>&1; then
    curl -sLfo "$dest" "$url"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$dest" "$url"
  else
    error "Neither curl nor wget found."
  fi
}

# ── Main ────────────────────────────────────────────────────
main() {
  os=$(detect_os)
  arch=$(detect_arch)
  tag=$(resolve_version)

  archive="kuro_${tag}_${os}_${arch}.tar.gz"
  checksum_file="kuro_${tag}_checksums.txt"
  download_url="https://github.com/${REPO}/releases/download/${tag}"

  info "Kuro ${tag} — ${os}/${arch}"
  echo ""

  # ── Create temp directory ──────────────────────────────────
  tmpdir=$(mktemp -d /tmp/kuro-install-XXXXXX)
  trap 'rm -rf "$tmpdir"' EXIT

  # ── Download archive ───────────────────────────────────────
  printf "  Downloading %s ... " "$archive"
  download "${download_url}/${archive}" "${tmpdir}/${archive}"
  info "done"

  # ── Download checksums ─────────────────────────────────────
  printf "  Downloading checksums ... "
  download "${download_url}/${checksum_file}" "${tmpdir}/${checksum_file}"
  info "done"

  # ── Verify checksum ────────────────────────────────────────
  printf "  Verifying checksum ... "
  expected=$(grep "${archive}" "${tmpdir}/${checksum_file}" | awk '{print $1}')
  if [ -z "$expected" ]; then
    error "checksum for ${archive} not found in ${checksum_file}"
  fi

  if command -v sha256sum >/dev/null 2>&1; then
    actual=$(sha256sum "${tmpdir}/${archive}" | awk '{print $1}')
  elif command -v shasum >/dev/null 2>&1; then
    actual=$(shasum -a 256 "${tmpdir}/${archive}" | awk '{print $1}')
  else
    error "sha256sum or shasum not found — install one of them and try again"
  fi

  if [ "$actual" != "$expected" ]; then
    error "Checksum mismatch!
  Expected: $expected
  Actual:   $actual"
  fi
  info "done"

  # ── Extract ────────────────────────────────────────────────
  printf "  Extracting ... "
  tar -xzf "${tmpdir}/${archive}" -C "$tmpdir"
  info "done"

  # ── Install ────────────────────────────────────────────────
  if [ ! -d "$INSTALL_DIR" ]; then
    mkdir -p "$INSTALL_DIR"
  fi

  if [ -f "${INSTALL_DIR}/kuro" ]; then
    warn "  Overwriting existing kuro binary at ${INSTALL_DIR}/kuro"
  fi

  cp "${tmpdir}/kuro" "${INSTALL_DIR}/kuro"
  chmod 755 "${INSTALL_DIR}/kuro"
  info "  Installed to ${INSTALL_DIR}/kuro"

  # ── Verify ─────────────────────────────────────────────────
  if echo "$PATH" | grep -q "$INSTALL_DIR"; then
    info ""
    info "  ✅ Kuro ${tag} installed!"
    info ""
    info "  Run: kuro scan ./your-repo"
  else
    warn ""
    warn "  ✅ Kuro ${tag} installed to ${INSTALL_DIR}/kuro"
    warn "  ${INSTALL_DIR} is not in your PATH."
    warn "  Add it: export PATH=\"\$PATH:${INSTALL_DIR}\""
    warn "  Or run: ${INSTALL_DIR}/kuro scan ./your-repo"
  fi
}

main
