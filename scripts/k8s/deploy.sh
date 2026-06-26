#!/usr/bin/env bash
set -euo pipefail

# One-click deploy for SkyTree 3-node cluster and QuestDB on Kubernetes.
# This script renders template YAMLs via envsubst and applies them in a deterministic order.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
K8S_DIR="${ROOT_DIR}/deploy/k8s/cluster3"

NAMESPACE="${NAMESPACE:-skytree-test}"
SKYTREE_IMAGE="${SKYTREE_IMAGE:-skytree:latest}"

# Storage
SKYTREE_STORAGE_CLASS="${SKYTREE_STORAGE_CLASS:-standard}"
SKYTREE_STORAGE_SIZE="${SKYTREE_STORAGE_SIZE:-10Gi}"
QUESTDB_STORAGE_CLASS="${QUESTDB_STORAGE_CLASS:-standard}"
QUESTDB_STORAGE_SIZE="${QUESTDB_STORAGE_SIZE:-10Gi}"

# Optional exposure
EXPOSE_NODEPORT="${EXPOSE_NODEPORT:-false}"
NODEPORT_MQTT="${NODEPORT_MQTT:-30183}"
NODEPORT_HTTP="${NODEPORT_HTTP:-30526}"

render_apply() {
  local tmpl="$1"
  envsubst < "${tmpl}" | kubectl apply -f -
}

echo "[deploy] namespace=${NAMESPACE} image=${SKYTREE_IMAGE}"

render_apply "${K8S_DIR}/namespace.yaml.tmpl"
render_apply "${K8S_DIR}/questdb.yaml.tmpl"
render_apply "${K8S_DIR}/skytree-config.yaml.tmpl"
render_apply "${K8S_DIR}/skytree.yaml.tmpl"

if [[ "${EXPOSE_NODEPORT}" == "true" ]]; then
  render_apply "${K8S_DIR}/skytree-nodeport.yaml.tmpl"
fi

echo "[deploy] waiting for QuestDB..."
kubectl -n "${NAMESPACE}" rollout status deploy/questdb --timeout=300s

echo "[deploy] waiting for SkyTree StatefulSet..."
kubectl -n "${NAMESPACE}" rollout status statefulset/skytree --timeout=600s

echo "[deploy] done"
echo "[deploy] tip: kubectl -n ${NAMESPACE} get pods,svc"





