#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${SKYTREE_K8S_NAMESPACE:-skytree-local}"
SCYLLA_HOST="${SKYTREE_SCYLLA_HOST:-127.0.0.1}"
SCYLLA_PORT="${SKYTREE_SCYLLA_PORT:-9042}"
STATE_DIR="${SKYTREE_STATE_DIR:-../.skytree-full}"
PID_FILE="${STATE_DIR}/scylla.port-forward.pid"
LOG_FILE="${STATE_DIR}/scylla.port-forward.log"
STARTED_PORT_FORWARD=false

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

port_open() {
  (echo >"/dev/tcp/${SCYLLA_HOST}/${SCYLLA_PORT}") >/dev/null 2>&1
}

stop_port_forward() {
  if [[ "${STARTED_PORT_FORWARD}" != "true" ]]; then
    return 0
  fi
  if [[ ! -f "${PID_FILE}" ]]; then
    return 0
  fi
  local pid
  pid="$(<"${PID_FILE}")"
  rm -f "${PID_FILE}"
  if [[ -n "${pid}" ]] && kill -0 "${pid}" >/dev/null 2>&1; then
    kill "${pid}" >/dev/null 2>&1 || true
  fi
}

start_port_forward_if_needed() {
  if port_open; then
    return 0
  fi
  if ! command -v kubectl >/dev/null 2>&1; then
    echo "Scylla is not reachable at ${SCYLLA_HOST}:${SCYLLA_PORT}, and kubectl is not available" >&2
    exit 1
  fi
  kubectl -n "${NAMESPACE}" rollout status statefulset/scylla --timeout=600s
  kubectl -n "${NAMESPACE}" wait pod -l app=scylla --for=condition=Ready --timeout=600s
  mkdir -p "${STATE_DIR}"
  nohup kubectl -n "${NAMESPACE}" port-forward --address "${SCYLLA_HOST}" svc/scylla "${SCYLLA_PORT}:9042" >"${LOG_FILE}" 2>&1 &
  local pid="$!"
  echo "${pid}" >"${PID_FILE}"
  STARTED_PORT_FORWARD=true

  local attempt
  for attempt in {1..80}; do
    if ! kill -0 "${pid}" >/dev/null 2>&1; then
      echo "Scylla port-forward exited unexpectedly; see ${LOG_FILE}" >&2
      exit 1
    fi
    if port_open; then
      return 0
    fi
    sleep 0.25
  done

  echo "Timed out waiting for Scylla port-forward ${SCYLLA_HOST}:${SCYLLA_PORT}; see ${LOG_FILE}" >&2
  exit 1
}

main() {
  cd "${REPO_ROOT}"
  trap stop_port_forward EXIT
  start_port_forward_if_needed
  SKYTREE_SCYLLA_INTEGRATION=1 \
  SKYTREE_SCYLLA_HOST="${SCYLLA_HOST}" \
  SKYTREE_SCYLLA_PORT="${SCYLLA_PORT}" \
    go test -tags="integration scylla" ./pkg/storage/delivery
}

main "$@"
