#!/bin/sh
# Envapor installer for Linux (and macOS as a fallback to Homebrew).
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/automazeio/envapor/main/installers/install.sh | sh
#
# Environment overrides:
#   ENVAPOR_VERSION      Release tag to install (e.g. v1.2.3). Defaults to latest.
#   ENVAPOR_INSTALL_DIR  Target directory. Defaults to /usr/local/bin when
#                        writable, otherwise ~/.local/bin.
#   ENVAPOR_REPO         GitHub owner/repo to download from. Defaults to
#                        automazeio/envapor (override for testing forks).
set -eu

REPO="${ENVAPOR_REPO:-automazeio/envapor}"
BINARY="envapor"

info() { printf 'envapor: %s\n' "$1"; }
err() { printf 'envapor: error: %s\n' "$1" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || err "required command not found: $1"; }

need curl
need tar

os="$(uname -s)"
case "$os" in
  Linux) os="linux" ;;
  Darwin) os="darwin" ;;
  *) err "unsupported operating system: $os (download a release from https://github.com/${REPO}/releases)" ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64 | amd64) arch="amd64" ;;
  aarch64 | arm64) arch="arm64" ;;
  *) err "unsupported architecture: $arch" ;;
esac

version="${ENVAPOR_VERSION:-}"
if [ -z "$version" ]; then
  info "resolving latest release"
  # Resolve the latest tag via the releases/latest redirect rather than the
  # GitHub API, which rate-limits unauthenticated requests (shared CI egress
  # IPs hit the limit constantly).
  effective="$(curl -fsSLI -o /dev/null -w '%{url_effective}' \
    "https://github.com/${REPO}/releases/latest")" || err "could not reach GitHub to resolve latest release"
  version="${effective##*/tag/}"
fi
case "$version" in
  "" | */*) err "could not determine release version" ;;
esac
number="${version#v}"

asset="${BINARY}_${number}_${os}_${arch}.tar.gz"
base="https://github.com/${REPO}/releases/download/${version}"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM

info "downloading ${asset}"
curl -fsSL "${base}/${asset}" -o "${tmp}/${asset}" || err "failed to download ${asset}"
curl -fsSL "${base}/checksums.txt" -o "${tmp}/checksums.txt" || err "failed to download checksums.txt"

info "verifying checksum"
expected="$(awk -v f="$asset" '$2 == f { print $1 }' "${tmp}/checksums.txt" | head -n1)"
[ -n "$expected" ] || err "checksum for ${asset} not found in checksums.txt"
if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "${tmp}/${asset}" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
  actual="$(shasum -a 256 "${tmp}/${asset}" | awk '{print $1}')"
else
  err "no SHA-256 tool found (need sha256sum or shasum)"
fi
[ "$expected" = "$actual" ] || err "checksum mismatch for ${asset}"

tar -xzf "${tmp}/${asset}" -C "${tmp}"
[ -f "${tmp}/${BINARY}" ] || err "binary '${BINARY}' not found in archive"

dir="${ENVAPOR_INSTALL_DIR:-}"
if [ -z "$dir" ]; then
  if [ -d "/usr/local/bin" ] && [ -w "/usr/local/bin" ]; then
    dir="/usr/local/bin"
  else
    dir="${HOME}/.local/bin"
  fi
fi
mkdir -p "$dir"

if ! install -m 0755 "${tmp}/${BINARY}" "${dir}/${BINARY}" 2>/dev/null; then
  cp "${tmp}/${BINARY}" "${dir}/${BINARY}"
  chmod 0755 "${dir}/${BINARY}"
fi

# Install the git-envapor shim so `git envapor <command>` works. A relative
# symlink keeps it valid if the directory is later moved.
if ln -sf "${BINARY}" "${dir}/git-${BINARY}" 2>/dev/null; then
  info "enabled 'git envapor'"
fi

info "installed ${BINARY} ${version} to ${dir}/${BINARY}"

case ":${PATH}:" in
  *":${dir}:"*) ;;
  *) info "add ${dir} to your PATH, e.g. export PATH=\"${dir}:\$PATH\"" ;;
esac
