#!/usr/bin/env bash
#
# Cross-compile release archives into dist/ and write checksums.txt.
#
# Usage: scripts/build-release.sh
# Env:   VERSION, COMMIT, BUILD_DATE, PLATFORMS ("os/arch os/arch ...")
set -euo pipefail

PKG="github.com/ValentinoTriadi/ci-cd-example"
BINARY="server"
DIST="dist"

VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo none)}"
BUILD_DATE="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

read -r -a PLATFORM_LIST <<<"${PLATFORMS:-linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64}"

LDFLAGS="-s -w \
  -X ${PKG}/internal/version.Version=${VERSION} \
  -X ${PKG}/internal/version.Commit=${COMMIT} \
  -X ${PKG}/internal/version.BuildDate=${BUILD_DATE}"

rm -rf "$DIST"
mkdir -p "$DIST"

echo "Building ${BINARY} ${VERSION} (${COMMIT}) for ${#PLATFORM_LIST[@]} platform(s)"

for platform in "${PLATFORM_LIST[@]}"; do
  os="${platform%%/*}"
  arch="${platform##*/}"

  name="${BINARY}-${VERSION}-${os}-${arch}"
  stage="${DIST}/${name}"
  mkdir -p "$stage"

  binary="${stage}/${BINARY}"
  [[ "$os" == "windows" ]] && binary="${binary}.exe"

  echo "  → ${platform}"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go build -trimpath -ldflags="$LDFLAGS" -o "$binary" ./cmd/server

  cp README.md "$stage/" 2>/dev/null || true

  # Windows users get a zip; everyone else gets a tar.gz.
  if [[ "$os" == "windows" ]]; then
    (cd "$DIST" && zip -qr "${name}.zip" "$name")
  else
    tar -czf "${DIST}/${name}.tar.gz" -C "$DIST" "$name"
  fi
  rm -rf "$stage"
done

# One checksum file covering every archive, so releases are verifiable.
(
  cd "$DIST"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum ./*.tar.gz ./*.zip 2>/dev/null | sed 's|\./||' >checksums.txt
  else
    shasum -a 256 ./*.tar.gz ./*.zip 2>/dev/null | sed 's|\./||' >checksums.txt
  fi
)

echo
echo "Artifacts in ${DIST}/:"
ls -lh "$DIST"
echo
cat "${DIST}/checksums.txt"
