#!/bin/sh
set -e

usage() {
    echo "Usage: $0 [native|embed] [linux|windows|darwin|linux-arm64|all]"
    echo ""
    echo "  native          Build with system ffmpeg (default)"
    echo "  embed           Build with embedded ffmpeg"
    echo "  linux           Build for Linux amd64"
    echo "  linux-arm64     Build for Linux arm64"
    echo "  windows         Build for Windows amd64"
    echo "  darwin          Build for macOS amd64"
    echo "  darwin-arm64    Build for macOS arm64"
    echo "  all             Build for all platforms"
    echo ""
    echo "Examples:"
    echo "  $0              Build native Linux binary"
    echo "  $0 embed all    Build embedded ffmpeg for all platforms"
    echo "  $0 native all   Build system ffmpeg for all platforms"
    exit 1
}

MODE="${1:-native}"
PLATFORM="${2:-linux}"

# Auto-detect version from git tag, fall back to VERSION env var
VERSION="${VERSION:-$(git describe --tags --abbrev=0 2>/dev/null || true)}"

if [ "$MODE" != "native" ] && [ "$MODE" != "embed" ]; then
    usage
fi

if [ "$MODE" = "embed" ]; then
    TAG="-tags embedffmpeg"
    SUFFIX="-embedded-ffmpeg"
else
    TAG=""
    SUFFIX=""
fi

echo "==> Building frontend..."
cd frontend
npm ci
npm run build
cd ..

echo "==> Copying frontend dist..."
rm -rf backend/embed/dist
cp -r frontend/dist backend/embed/

build() {
    GOOS=$1
    GOARCH=$2
    EXT=$3
    OUTPUT_DIR="${OUTPUT_DIR:-.}"
    mkdir -p "$OUTPUT_DIR"
    if [ -n "$VERSION" ]; then
        NAME="${OUTPUT_DIR}/living-recorder-${VERSION}-${GOOS}-${GOARCH}${SUFFIX}${EXT}"
    else
        NAME="${OUTPUT_DIR}/living-recorder-${GOOS}-${GOARCH}${SUFFIX}${EXT}"
    fi

    echo "==> Building $NAME..."
    cd backend
    GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=0 go build $TAG -o "../$NAME" .
    cd ..
    echo "    -> $NAME"
}

case "$PLATFORM" in
    linux)
        build linux amd64 ""
        ;;
    linux-arm64)
        build linux arm64 ""
        ;;
    windows)
        build windows amd64 ".exe"
        ;;
    windows-arm64)
        build windows arm64 ".exe"
        ;;
    darwin)
        build darwin amd64 ""
        ;;
    darwin-arm64)
        build darwin arm64 ""
        ;;
    all)
        build darwin amd64 ""
        build darwin arm64 ""
        build linux amd64 ""
        build linux arm64 ""
        build windows amd64 ".exe"
        build windows arm64 ".exe"
        ;;
    *)
        usage
        ;;
esac

echo "==> Done"
