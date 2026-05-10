#!/bin/bash
# Build Linux Bridge Binaries
# Cross-compiles bridge for multiple architectures

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${BLUE}Building ArmorClaw Bridge Binaries${NC}"
echo ""

# Ensure we're in the project root
cd "$(dirname "$0")"

# Version
VERSION="${VERSION:-v1.0.0}"
BUILD_DIR="build/bridge"
mkdir -p "$BUILD_DIR"

# Platforms to build
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "linux/arm/v7"
)

echo "Building for: ${PLATFORMS[*]}"
echo ""

# Build for each platform
for PLATFORM in "${PLATFORMS[@]}"; do
    IFS='/' read -r OS ARCH <<< "$PLATFORM"

    # Convert platform to filename format
    case "$ARCH" in
        amd64)  FILENAME="linux-amd64" ;;
        arm64)  FILENAME="linux-arm64" ;;
        arm/v7) FILENAME="linux-armv7" ;;
        *)      FILENAME="linux-$ARCH" ;;
    esac

    BINARY="$BUILD_DIR/armorclaw-bridge-$FILENAME"

    # Skip Windows extension for Linux
    if [[ "$OS" == "linux" ]]; then
        BINARY_EXT=""
    else
        BINARY_EXT=".exe"
    fi

    echo -e "${BLUE}Building $OS $ARCH...${NC}"

    # Set Go env for cross-compilation
    export GOOS="$OS"
    export GOARCH="$ARCH"
    export CGO_ENABLED=1

    # Build
    if go build -o "$BINARY$BINARY_EXT" -ldflags="-s -w" ./bridge/cmd/bridge; then
        SIZE=$(du -h "$BINARY$BINARY_EXT" | cut -f1)
        echo -e "${GREEN}✓${NC} Created: $BINARY$BINARY_EXT ($SIZE)"

        # Generate checksum
        if command -v sha256sum &>/dev/null; then
            sha256sum "$BINARY$BINARY_EXT" | tee -a "$BUILD_DIR/SHA256SUMS"
        fi
    else
        echo -e "${YELLOW}✗${NC} Failed to build $OS $ARCH"
    fi
done

echo ""
echo -e "${GREEN}Build complete!${NC}"
echo "Binaries in: $BUILD_DIR"
echo ""

# Also build for current platform for local use
CURRENT_GOOS=$(go env GOOS)
CURRENT_GOARCH=$(go env GOARCH)
echo "Building for current platform: $CURRENT_GOOS/$CURRENT_GOARCH"

go build -o "$BUILD_DIR/armorclaw-bridge" ./bridge/cmd/bridge
echo -e "${GREEN}✓${NC} Local binary: $BUILD_DIR/armorclaw-bridge"
