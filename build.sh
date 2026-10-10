#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

APP="fadownload"
OUT_DIR="${OUT_DIR:-dist}"
LDFLAGS="-s -w"

need() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "error: required command not found: $1" >&2
    exit 1
  }
}

need go
need upx

mkdir -p "$OUT_DIR"

build_one() {
  local goos="$1"
  local goarch="$2"
  local ext=""
  [[ "$goos" == "windows" ]] && ext=".exe"

  local out="${OUT_DIR}/${APP}-${goos}-${goarch}${ext}"

  echo "==> Building ${APP} for ${goos}/${goarch}"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags="$LDFLAGS" -o "$out" .

  echo "==> Compressing ${out}"
  upx --best --force-overwrite "$out" >/dev/null

  local size
  size="$(du -h "$out" | awk '{print $1}')"
  echo "    ok  ${out}  (${size})"
}

build_one linux amd64
build_one windows amd64

echo
echo "Done. Artifacts in ${OUT_DIR}/"
ls -lh "$OUT_DIR"/"${APP}"-*
