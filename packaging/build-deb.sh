#!/bin/bash
# Build script for docker-volume-netshare .deb packages
# Usage: ./build-deb.sh [version]
#
# Requirements:
#   - Go 1.23 or later
#   - nfpm (https://nfpm.goreleaser.com/install/)
#
# Install nfpm:
#   go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest
# or:
#   curl -sfL https://install.goreleaser.com/github.com/goreleaser/nfpm.sh | sh -s -- -b /usr/local/bin

set -e

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Default version from Makefile or fallback
VERSION="${1:-0.36}"
BUILD_DIR="${PROJECT_ROOT}/build/deb"

echo "=== Building docker-volume-netshare v${VERSION} ==="
echo "Project root: ${PROJECT_ROOT}"
echo "Build dir: ${BUILD_DIR}"

# Create build directory
mkdir -p "${BUILD_DIR}"

# Build Go binary for both architectures
echo ""
echo "=== Building binaries ==="

for GOARCH in amd64 arm64; do
    echo "Building for linux/${GOARCH}..."
    GOOS=linux GOARCH=${GOARCH} go build \
        -ldflags="-s -w -X main.VERSION=${VERSION} -X main.BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
        -o "${BUILD_DIR}/docker-volume-netshare-${GOARCH}" \
        "${PROJECT_ROOT}"
done

# Check if nfpm is available
if ! command -v nfpm &> /dev/null; then
    echo ""
    echo "ERROR: nfpm not found. Install it with:"
    echo "  go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest"
    echo ""
    echo "Binaries built successfully in ${BUILD_DIR}/"
    echo "To build .deb packages, install nfpm and run this script again."
    exit 0
fi

# Build .deb packages
echo ""
echo "=== Building .deb packages ==="

cd "${SCRIPT_DIR}"

for GOARCH in amd64 arm64; do
    # Set architecture name for deb packages
    case ${GOARCH} in
        amd64) DEB_ARCH="amd64" ;;
        arm64) DEB_ARCH="arm64" ;;
    esac
    
    echo "Building .deb for ${DEB_ARCH}..."
    
    # Copy binary to project root temporarily
    cp "${BUILD_DIR}/docker-volume-netshare-${GOARCH}" "${PROJECT_ROOT}/docker-volume-netshare"
    
    # Build .deb using nfpm
    VERSION="${VERSION}" ARCH="${DEB_ARCH}" nfpm package \
        --config nfpm.yaml \
        --packager deb \
        --target "${BUILD_DIR}/"
    
    # Clean up temporary binary
    rm -f "${PROJECT_ROOT}/docker-volume-netshare"
done

echo ""
echo "=== Build complete ==="
echo "Packages available in: ${BUILD_DIR}/"
ls -la "${BUILD_DIR}/"*.deb 2>/dev/null || echo "No .deb files found (nfpm may not be installed)"
