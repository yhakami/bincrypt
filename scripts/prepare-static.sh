#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC_DIR="${ROOT_DIR}/src/static"
DIST_DIR="${1:-${ROOT_DIR}/dist}"

if [[ ! -d "${SRC_DIR}" ]]; then
    echo "missing source directory: ${SRC_DIR}" >&2
    exit 1
fi

rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}/static"

# Keep canonical routes at the root for static hosts.
cp "${SRC_DIR}/index.html" "${DIST_DIR}/index.html"
cp "${SRC_DIR}/viewer.html" "${DIST_DIR}/viewer.html"
cp "${SRC_DIR}/about.html" "${DIST_DIR}/about.html"
cp "${SRC_DIR}/news.html" "${DIST_DIR}/news.html"

# Keep JS under /static so existing URLs continue to work.
cp "${SRC_DIR}/app.js" "${DIST_DIR}/static/app.js"
cp "${SRC_DIR}/viewer.js" "${DIST_DIR}/static/viewer.js"

echo "exported static site to ${DIST_DIR}"
