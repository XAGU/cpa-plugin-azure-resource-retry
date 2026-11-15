#!/bin/sh
set -eu

VERSION="${VERSION:-0.1.0}"
GOOS="${GOOS:-$(go env GOOS)}"
GOARCH="${GOARCH:-$(go env GOARCH)}"

case "$GOOS" in
  linux|freebsd) EXT=so ;;
  darwin) EXT=dylib ;;
  windows) EXT=dll ;;
  *) echo "不支持的平台：$GOOS" >&2; exit 1 ;;
esac

mkdir -p dist
CGO_ENABLED=1 GOOS="$GOOS" GOARCH="$GOARCH" go build -trimpath -buildvcs=false -buildmode=c-shared \
  -ldflags "-s -w -X github.com/XAGU/cpa-plugin-azure-resource-retry/internal/plugin.Version=${VERSION}" \
  -o "dist/azure-resource-retry.${EXT}" ./cmd/azure-resource-retry
