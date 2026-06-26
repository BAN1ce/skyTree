#!/usr/bin/env bash
set -euo pipefail

# Local dev helper for OrbStack K8s.
# Builds the latest SkyTree code into a local image, deploys manifests, and restarts pods
# so the cluster always runs the newest binary.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

NAMESPACE="${NAMESPACE:-skytree-test}"
SKYTREE_IMAGE="${SKYTREE_IMAGE:-skytree:dev}"
DOCKERFILE="${DOCKERFILE:-${ROOT_DIR}/Dockerfile}"

NO_CACHE="${NO_CACHE:-false}"
BUILD_ARGS="${BUILD_ARGS:-}"

echo "[dev] building image ${SKYTREE_IMAGE}"
if [[ "${NO_CACHE}" == "true" ]]; then
  docker build --no-cache -f "${DOCKERFILE}" -t "${SKYTREE_IMAGE}" ${BUILD_ARGS} "${ROOT_DIR}"
else
  docker build -f "${DOCKERFILE}" -t "${SKYTREE_IMAGE}" ${BUILD_ARGS} "${ROOT_DIR}"
fi

echo "[dev] deploying to namespace ${NAMESPACE}"
NAMESPACE="${NAMESPACE}" SKYTREE_IMAGE="${SKYTREE_IMAGE}" "${ROOT_DIR}/scripts/k8s/deploy.sh"

# If tag stays the same (skytree:dev), Kubernetes won't redeploy automatically.
# Restarting pods guarantees the new local image is used.
if kubectl -n "${NAMESPACE}" get statefulset/skytree >/dev/null 2>&1; then
  echo "[dev] restarting skytree statefulset to pick up latest image"
  kubectl -n "${NAMESPACE}" rollout restart statefulset/skytree
  kubectl -n "${NAMESPACE}" rollout status statefulset/skytree --timeout=600s
fi

echo "[dev] done"





