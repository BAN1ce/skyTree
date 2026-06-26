#!/usr/bin/env bash
set -euo pipefail

# One-click reset for SkyTree K8s test clusters.
# Deletes the namespace to wipe all workload state and PVCs, then optionally re-deploys.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

NAMESPACE="${NAMESPACE:-skytree-test}"
REDEPLOY="${REDEPLOY:-true}"

echo "[reset] deleting namespace ${NAMESPACE} (this wipes PVC data)"
kubectl delete namespace "${NAMESPACE}" --ignore-not-found=true

echo "[reset] waiting for namespace to terminate..."
for _ in {1..120}; do
  if ! kubectl get namespace "${NAMESPACE}" >/dev/null 2>&1; then
    echo "[reset] namespace deleted"
    break
  fi
  sleep 1
done

if kubectl get namespace "${NAMESPACE}" >/dev/null 2>&1; then
  echo "[reset] namespace still exists after timeout"
  exit 1
fi

if [[ "${REDEPLOY}" == "true" ]]; then
  echo "[reset] redeploying..."
  "${ROOT_DIR}/scripts/k8s/deploy.sh"
fi

echo "[reset] done"





