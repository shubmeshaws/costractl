#!/bin/bash
set -euo pipefail

# ============================================================
# Costra CLI installer — Linux
# Served from: https://get.costraai.com/linux
# ============================================================

REPO="shubmeshaws/costractl"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="costractl"

echo "Detecting architecture..."
ARCH=$(uname -m)

case "$ARCH" in
  x86_64)
    ASSET="costractl-linux-amd64"
    ;;
  aarch64|arm64)
    ASSET="costractl-linux-arm64"
    ;;
  *)
    echo "Error: unsupported architecture ${ARCH}."
    exit 1
    ;;
esac

echo "Fetching latest release info..."
LATEST_TAG=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name":' \
  | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_TAG" ]; then
  echo "Error: could not determine latest version. Check https://github.com/${REPO}/releases"
  exit 1
fi

DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST_TAG}/${ASSET}"
CHECKSUM_URL="https://github.com/${REPO}/releases/download/${LATEST_TAG}/checksums.txt"

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

echo "Downloading ${ASSET} (${LATEST_TAG})..."
curl -sL "$DOWNLOAD_URL" -o "${TMP_DIR}/${BINARY_NAME}"

echo "Downloading checksums for verification..."
curl -sL "$CHECKSUM_URL" -o "${TMP_DIR}/checksums.txt"

echo "Verifying checksum..."
EXPECTED_SHA=$(grep "$ASSET" "${TMP_DIR}/checksums.txt" | awk '{print $1}')
ACTUAL_SHA=$(sha256sum "${TMP_DIR}/${BINARY_NAME}" | awk '{print $1}')

if [ -z "$EXPECTED_SHA" ]; then
  echo "Warning: no checksum entry found for ${ASSET}. Proceeding without verification."
elif [ "$EXPECTED_SHA" != "$ACTUAL_SHA" ]; then
  echo "Checksum mismatch! Expected ${EXPECTED_SHA}, got ${ACTUAL_SHA}."
  echo "Aborting install for safety."
  exit 1
else
  echo "Checksum verified."
fi

chmod +x "${TMP_DIR}/${BINARY_NAME}"

echo "Installing to ${INSTALL_DIR}/${BINARY_NAME} (may prompt for sudo password)..."
if [ -w "$INSTALL_DIR" ]; then
  mv "${TMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
else
  sudo mv "${TMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
fi

echo ""
echo "costractl ${LATEST_TAG} installed successfully."
echo "Run 'costractl --version' to verify, or 'costractl cluster connect --help' to get started."
