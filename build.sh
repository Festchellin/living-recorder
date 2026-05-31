#!/bin/sh
set -e

usage() {
    echo "Usage: $0 [native|embed] [linux|windows|all]"
    echo ""
    echo "  native          Build with system ffmpeg (default)"
    echo "  embed           Build with embedded ffmpeg"
    echo "  linux           Build for Linux amd64"
    echo "  windows         Build for Windows amd64"
    echo "  all             Build for Linux and Windows"
    echo ""
    echo "Examples:"
    echo "  $0              Build native Linux binary"
    echo "  $0 embed all    Build embedded ffmpeg for all platforms"
    echo "  $0 native all   Build system ffmpeg for all platforms"
    exit 1
}

MODE="${1:-native}"
PLATFORM="${2:-linux}"

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
    NAME="living-recorder-${GOOS}-${GOARCH}${SUFFIX}${EXT}"

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
    windows)
        build windows amd64 ".exe"
        ;;
    all)
        build linux amd64 ""
        build windows amd64 ".exe"
        ;;
    *)
        usage
        ;;
esac

echo "==> Done"
