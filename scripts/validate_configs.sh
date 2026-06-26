#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

go run ./cmd/configcheck etc/config.yaml etc/config.example.yaml
go run ./cmd/configcheck -pattern 'etc/examples/*.yaml'
go run ./cmd/configcheck -pattern 'etc/fixtures/*.yaml'
go run ./cmd/configcheck -profile beta etc/examples/config.beta.example.yaml
