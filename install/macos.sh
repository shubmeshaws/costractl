#!/bin/bash
# install.sh — Install costractl on Linux or macOS.
#
# Author: Shubham Meshram
#
# Usage:
#   curl -fsSL https://get.costraai.com/install.sh | bash
#   curl -fsSL https://get.costraai.com/install.sh | bash -s -- --version v0.2.0
#   curl -fsSL https://get.costraai.com/install.sh | bash -s -- --install-dir /usr/local/bin
#
set -euo pipefail

# ── Defaults ──────────────────────────────────────────────────────────
REPO="shubmeshaws/costractl"
INSTALL_DIR="$HOME/.local/bin"
VERSION="latest"
BINARY_NAME="costractl"

# ── Parse flags ───────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
	case "$1" in
	--version)
		VERSION="$2"
		shift 2
		;;
	--install-dir)
		INSTALL_DIR="$2"
		shift 2
		;;
	-h | --help)
		cat <<EOF
costractl installer

Author: Shubham Meshram

Usage: install.sh [--version VERSION] [--install-dir DIR]

Options:
  --version VERSION      Install a specific version (default: latest)
  --install-dir DIR      Install location (default: \$HOME/.local/bin)
  -h, --help             Show this help message

Examples:
  install.sh
  install.sh --version v0.2.0
  install.sh --install-dir /usr/local/bin
EOF
		exit 0
		;;
	*)
		echo "Unknown option: $1"
		echo "Run with --help for usage."
		exit 1
		;;
	esac
done

# ── Helpers ───────────────────────────────────────────────────────────
info() { echo "[costractl] $*"; }
error() {
	echo "[costractl] ERROR: $*" >&2
	exit 1
}

need_cmd() {
	if ! command -v "$1" &>/dev/null; then
		error "'$1' is required but not found. Please install it and try again."
	fi
}

# Download with retries; fall back to wget or gh when curl fails (common on flaky networks).
download_file() {
	local url="$1"
	local dest="$2"
	local attempt

	for attempt in 1 2 3; do
		if curl -fsSL \
			--http1.1 \
			--connect-timeout 30 \
			--max-time 600 \
			--retry 3 \
			--retry-delay 2 \
			--retry-all-errors \
			-o "$dest" \
			"$url"; then
			return 0
		fi
		info "Download attempt ${attempt}/3 failed, retrying..."
		sleep 2
	done

	if command -v wget &>/dev/null; then
		info "curl failed — trying wget..."
		wget -q --timeout=60 --tries=3 -O "$dest" "$url" && return 0
	fi

	if command -v gh &>/dev/null; then
		info "curl failed — trying gh release download..."
		local asset_name
		asset_name="$(basename "$url")"
		gh release download "$VERSION" \
			--repo "$REPO" \
			--pattern "$asset_name" \
			--dir "$(dirname "$dest")" \
			--clobber && mv "$(dirname "$dest")/${asset_name}" "$dest" && return 0
	fi

	return 1
}

# ── Detect OS and architecture ────────────────────────────────────────
detect_platform() {
	local os arch

	os="$(uname -s)"
	case "$os" in
	Linux*) os="linux" ;;
	Darwin*) os="darwin" ;;
	*) error "Unsupported OS: $os. Use the Windows installer (install.ps1) on Windows." ;;
	esac

	arch="$(uname -m)"
	case "$arch" in
	x86_64 | amd64) arch="amd64" ;;
	aarch64 | arm64) arch="arm64" ;;
	*) error "Unsupported architecture: $arch" ;;
	esac

	echo "${os}_${arch}"
}

# ── Resolve latest version from GitHub Releases API ───────────────────
resolve_version() {
	if [[ "$VERSION" == "latest" ]]; then
		info "Resolving latest version..."
		VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
			| grep '"tag_name":' \
			| sed -E 's/.*"([^"]+)".*/\1/')" ||
			error "Could not resolve latest version. Specify one with --version."
		[[ -z "$VERSION" ]] && error "Could not resolve latest version. Specify one with --version."
	fi
	info "Version: ${VERSION}"
}

# ── Download, verify, install ─────────────────────────────────────────
main() {
	need_cmd curl
	need_cmd uname

	local platform os arch asset_name download_url checksum_url tmp_dir

	resolve_version
	platform="$(detect_platform)"
	os="${platform%_*}"
	arch="${platform#*_}"
	asset_name="costractl-${os}-${arch}"

	download_url="https://github.com/${REPO}/releases/download/${VERSION}/${asset_name}"
	checksum_url="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"

	tmp_dir="$(mktemp -d)"
	trap 'rm -rf "$tmp_dir"' EXIT

	info "Downloading ${asset_name} (${VERSION})..."
	download_file "$download_url" "${tmp_dir}/${BINARY_NAME}" ||
		error "Download failed after retries. Try manually:
  curl -L --http1.1 --retry 5 -o costractl ${download_url}
  chmod +x costractl && sudo mv costractl /usr/local/bin/costractl"

	if command -v sha256sum &>/dev/null || command -v shasum &>/dev/null; then
		info "Verifying checksum..."
		download_file "$checksum_url" "${tmp_dir}/checksums.txt" ||
			error "Could not download checksums file."

		local expected actual
		expected="$(grep "${asset_name}" "${tmp_dir}/checksums.txt" | awk '{print $1}')"
		if [[ -z "$expected" ]]; then
			error "Checksum for ${asset_name} not found in checksums.txt"
		fi

		if command -v sha256sum &>/dev/null; then
			actual="$(sha256sum "${tmp_dir}/${BINARY_NAME}" | awk '{print $1}')"
		else
			actual="$(shasum -a 256 "${tmp_dir}/${BINARY_NAME}" | awk '{print $1}')"
		fi

		if [[ "$expected" != "$actual" ]]; then
			error "Checksum mismatch! Expected: ${expected}, got: ${actual}"
		fi
		info "Checksum verified."
	else
		info "Warning: sha256sum/shasum not found, skipping checksum verification."
	fi

	chmod +x "${tmp_dir}/${BINARY_NAME}"

	mkdir -p "$INSTALL_DIR"
	if [[ -w "$INSTALL_DIR" ]]; then
		mv "${tmp_dir}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
	else
		info "Elevated permissions required to install to ${INSTALL_DIR}."
		sudo mv "${tmp_dir}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
	fi

	info "costractl ${VERSION} installed to ${INSTALL_DIR}/${BINARY_NAME}"

	case ":$PATH:" in
	*":${INSTALL_DIR}:"*) ;;
	*) info "Note: ${INSTALL_DIR} is not on your PATH. Add it with:
	  export PATH=\"${INSTALL_DIR}:\$PATH\"" ;;
	esac

	info "Run 'costractl --help' to get started."
}

main
