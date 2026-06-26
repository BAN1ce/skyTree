#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

require_file() {
  local path="$1"
  if [[ ! -f "${path}" ]]; then
    echo "missing required artifact: ${path}" >&2
    exit 1
  fi
}

require_dir() {
  local path="$1"
  if [[ ! -d "${path}" ]]; then
    echo "missing required directory: ${path}" >&2
    exit 1
  fi
}

echo "[beta] validating beta config profile"
go run ./cmd/configcheck -profile beta etc/examples/config.beta.example.yaml

echo "[beta] checking canonical deploy layout"
require_dir deploy/k8s/local
require_dir deploy/k8s/beta
require_dir deploy/k8s/stress
require_file deploy/k8s/README.md
require_file deploy/k8s/beta/skytree-config.yaml
require_file deploy/k8s/beta/skytree-statefulset.yaml
require_file deploy/k8s/stress/skytree-stress.yaml
require_file Dockerfile.k8s

if [[ -f web/console/package.json ]]; then
  echo "[beta] building console assets"
  (
    cd web/console
    if [[ ! -d node_modules ]]; then
      npm ci
    fi
    npm run build
  )
fi
