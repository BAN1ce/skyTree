#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="skytree-local"
IMAGE="skytree:cluster-local"
MANIFEST_DIR="deploy/k8s/local"
PROMETHEUS_LOCAL_PORT="${PROMETHEUS_LOCAL_PORT:-19090}"
GRAFANA_LOCAL_PORT="${GRAFANA_LOCAL_PORT:-13000}"
ENVOY_GATEWAY_VERSION="${ENVOY_GATEWAY_VERSION:-v1.8.1}"
ENVOY_GATEWAY_NAMESPACE="${ENVOY_GATEWAY_NAMESPACE:-envoy-gateway-system}"
ENVOY_GATEWAY_RELEASE="${ENVOY_GATEWAY_RELEASE:-eg}"
GATEWAY_NAME="skytree-gateway"
GATEWAY_CLASS_NAME="eg"
MQTT_GATEWAY_PORT="1883"
SKYTREE_NODE_PORT_FORWARDS=(
  "node1:pod/skytree-0:11883:19526"
  "node2:pod/skytree-1:21883:29526"
  "node3:pod/skytree-2:31883:39526"
)
NO_CACHE=false
DOWN=false
LOGS=false
STATUS=false
SCYLLA_SINGLE=false
JSON_OUTPUT=false
WAIT_READY=false
LEADER=false
STOP_POD=""
RESTART_POD=""
RECOVER_POD=""

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
STATE_DIR="${REPO_ROOT}/../.skytree-full"
API_PORT_FORWARD_PID="${STATE_DIR}/skytree-api.port-forward.pid"
MQTT_PORT_FORWARD_PID="${STATE_DIR}/skytree-mqtt.port-forward.pid"
PROMETHEUS_PORT_FORWARD_PID="${STATE_DIR}/prometheus.port-forward.pid"
GRAFANA_PORT_FORWARD_PID="${STATE_DIR}/grafana.port-forward.pid"
API_PORT_FORWARD_LOG="${STATE_DIR}/skytree-api.port-forward.log"
MQTT_PORT_FORWARD_LOG="${STATE_DIR}/skytree-mqtt.port-forward.log"
PROMETHEUS_PORT_FORWARD_LOG="${STATE_DIR}/prometheus.port-forward.log"
GRAFANA_PORT_FORWARD_LOG="${STATE_DIR}/grafana.port-forward.log"
MQTT_GATEWAY_HOST_FILE="${STATE_DIR}/skytree-mqtt.gateway-host"
GRAFANA_DASHBOARD_CONFIGMAP="skytree-grafana-dashboard"
GRAFANA_DASHBOARD_FILE="${REPO_ROOT}/deploy/grafana/dashboards/skytree-overview.json"

usage() {
  cat <<'EOF'
Usage: scripts/start-k8s-console.sh [options]

Options:
  --no-cache              Rebuild the skyTree image without Docker build cache.
  --down                  Delete local K8s resources and stop port-forward processes.
  --logs                  Follow skyTree pod logs after startup.
  --status                Print local K8s resource status.
  --scylla-single         Start Scylla as a single pod (replicas=1, RF=1) instead of a 3-node cluster.
  --json                  Print machine-readable JSON for supported status commands.
  --wait-ready            Wait for K8s resources to become ready, without rebuilding.
  --leader                Print /api/v1/cluster/overview from the local console API.
  --stop-pod POD          Stop a skyTree pod by deleting it and stopping its port-forward.
  --restart-pod POD       Stop then recover one skyTree pod.
  --recover-pod POD       Wait for one skyTree pod and restart its port-forward.
  --namespace NAME        Override the namespace.
  --image IMAGE           Override the skyTree image tag.
  -h, --help              Show this help.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --no-cache)
      NO_CACHE=true
      shift
      ;;
    --down)
      DOWN=true
      shift
      ;;
    --logs)
      LOGS=true
      shift
      ;;
    --status)
      STATUS=true
      shift
      ;;
    --scylla-single)
      SCYLLA_SINGLE=true
      shift
      ;;
    --json)
      JSON_OUTPUT=true
      shift
      ;;
    --wait-ready)
      WAIT_READY=true
      shift
      ;;
    --leader)
      LEADER=true
      shift
      ;;
    --stop-pod)
      if [[ $# -lt 2 || -z "$2" ]]; then
        echo "--stop-pod requires a value" >&2
        exit 2
      fi
      STOP_POD="$2"
      shift 2
      ;;
    --restart-pod)
      if [[ $# -lt 2 || -z "$2" ]]; then
        echo "--restart-pod requires a value" >&2
        exit 2
      fi
      RESTART_POD="$2"
      shift 2
      ;;
    --recover-pod)
      if [[ $# -lt 2 || -z "$2" ]]; then
        echo "--recover-pod requires a value" >&2
        exit 2
      fi
      RECOVER_POD="$2"
      shift 2
      ;;
    --namespace)
      if [[ $# -lt 2 || -z "$2" ]]; then
        echo "--namespace requires a value" >&2
        exit 2
      fi
      NAMESPACE="$2"
      shift 2
      ;;
    --image)
      if [[ $# -lt 2 || -z "$2" ]]; then
        echo "--image requires a value" >&2
        exit 2
      fi
      IMAGE="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

cd "${REPO_ROOT}"

require_command() {
  local name="$1"
  if ! command -v "${name}" >/dev/null 2>&1; then
    echo "required command not found: ${name}" >&2
    exit 1
  fi
}

docker_goarch() {
  local arch
  arch="$(docker version --format '{{.Server.Arch}}' 2>/dev/null || docker info --format '{{.Architecture}}')"
  case "${arch}" in
    amd64|x86_64)
      echo "amd64"
      ;;
    arm64|aarch64)
      echo "arm64"
      ;;
    *)
      echo "unsupported docker architecture: ${arch}" >&2
      exit 1
      ;;
  esac
}

build_local_skyTree_binary() {
  local out_dir="${REPO_ROOT}/.docker-build"
  local goarch
  goarch="$(docker_goarch)"
  mkdir -p "${out_dir}"
  echo "building local skyTree binary for linux/${goarch}"
  CGO_ENABLED=0 GOOS=linux GOARCH="${goarch}" go build \
    -ldflags='-w -s -extldflags "-static"' \
    -o "${out_dir}/skytree" \
    ./cmd/main.go
}

build_local_console_assets() {
  local console_dir="${REPO_ROOT}/web/console"
  if [[ ! -f "${console_dir}/package.json" ]]; then
    return 0
  fi
  require_command npm
  (
    cd "${console_dir}"
    if [[ ! -d node_modules ]]; then
      npm ci
    fi
    npm run build
  )
}

build_image() {
  local args=(-t "${IMAGE}" -f Dockerfile.k8s .)
  if [[ "${NO_CACHE}" == "true" ]]; then
    args=(--no-cache "${args[@]}")
  fi
  echo "building Docker image ${IMAGE}"
  docker build "${args[@]}"
}

load_image_to_cluster() {
  local context
  context="$(kubectl config current-context)"
  case "${context}" in
    kind-*|kind_*)
      if command -v kind >/dev/null 2>&1; then
        echo "loading image into kind cluster ${context}"
        kind load docker-image "${IMAGE}"
      fi
      ;;
    minikube)
      if command -v minikube >/dev/null 2>&1; then
        echo "loading image into minikube"
        minikube image load "${IMAGE}"
      fi
      ;;
    *)
      echo "using kubectl context ${context}; assuming it can access local Docker image ${IMAGE}"
      ;;
  esac
}

apply_manifests() {
  kubectl apply -f "${MANIFEST_DIR}/namespace.yaml"
  kubectl apply -f "${MANIFEST_DIR}/rbac.yaml"
  apply_scylla_manifest
  kubectl apply -f "${MANIFEST_DIR}/prometheus.yaml"
  apply_grafana_dashboard_configmap
  kubectl apply -f "${MANIFEST_DIR}/grafana.yaml"
  kubectl apply -f "${MANIFEST_DIR}/skytree-config.yaml"
  kubectl apply -f "${MANIFEST_DIR}/services.yaml"
  apply_skyTree_statefulset
  kubectl apply -f "${MANIFEST_DIR}/skytree-gateway.yaml"
  kubectl -n "${NAMESPACE}" set image statefulset/skytree skytree="${IMAGE}"
  kubectl -n "${NAMESPACE}" rollout restart statefulset/skytree
}

scylla_single_pod_manifest() {
  # Derive a single-pod variant of the 3-node Scylla manifest on the fly:
  #   - StatefulSet replicas 3 -> 1
  #   - seed list reduced to scylla-0 only
  #   - init keyspace replication factor 3 -> 1
  # The source manifest stays the canonical 3-node cluster definition.
  sed \
    -e 's/^\(  replicas:\) 3$/\1 1/' \
    -e 's/,scylla-1\.[^"]*//' \
    -e "s/'datacenter1': 3/'datacenter1': 1/" \
    "${MANIFEST_DIR}/scylla.yaml"
}

apply_scylla_manifest() {
  # The Makefile K8s profile now uses a Scylla StatefulSet. Clean up the old
  # local single-node Deployment and recreate the init Job because Job pod
  # templates are immutable.
  kubectl -n "${NAMESPACE}" delete deployment scylla --ignore-not-found=true
  kubectl -n "${NAMESPACE}" delete job scylla-init --ignore-not-found=true
  if [[ "${SCYLLA_SINGLE}" == "true" ]]; then
    echo "starting Scylla in single-pod mode (replicas=1, RF=1)"
    scylla_single_pod_manifest | kubectl apply -f -
  else
    kubectl apply -f "${MANIFEST_DIR}/scylla.yaml"
  fi
}

install_envoy_gateway() {
  echo "installing Envoy Gateway ${ENVOY_GATEWAY_VERSION}"
  if command -v helm >/dev/null 2>&1; then
    helm upgrade --install "${ENVOY_GATEWAY_RELEASE}" oci://docker.io/envoyproxy/gateway-helm \
      --version "${ENVOY_GATEWAY_VERSION}" \
      --namespace "${ENVOY_GATEWAY_NAMESPACE}" \
      --create-namespace
  else
    echo "helm is not installed; falling back to the pinned Envoy Gateway install manifest"
    kubectl apply --server-side --force-conflicts \
      -f "https://github.com/envoyproxy/gateway/releases/download/${ENVOY_GATEWAY_VERSION}/install.yaml"
  fi

  kubectl -n "${ENVOY_GATEWAY_NAMESPACE}" rollout status deployment/envoy-gateway --timeout=180s
  kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: ${GATEWAY_CLASS_NAME}
spec:
  controllerName: gateway.envoyproxy.io/gatewayclass-controller
EOF
  kubectl wait --for=condition=Accepted "gatewayclass/${GATEWAY_CLASS_NAME}" --timeout=120s
}

apply_grafana_dashboard_configmap() {
  if [[ ! -f "${GRAFANA_DASHBOARD_FILE}" ]]; then
    echo "Grafana dashboard file not found: ${GRAFANA_DASHBOARD_FILE}" >&2
    exit 1
  fi
  kubectl -n "${NAMESPACE}" create configmap "${GRAFANA_DASHBOARD_CONFIGMAP}" \
    --from-file=skytree-overview.json="${GRAFANA_DASHBOARD_FILE}" \
    --dry-run=client \
    -o yaml | kubectl apply -f -
}

apply_skyTree_statefulset() {
  local output
  if output="$(kubectl apply -f "${MANIFEST_DIR}/skytree-statefulset.yaml" 2>&1)"; then
    echo "${output}"
    return 0
  fi

  if [[ "${output}" == *"updates to statefulset spec"* ]]; then
    echo "${output}" >&2
    echo "recreating skyTree StatefulSet to apply immutable local K8s settings"
    kubectl -n "${NAMESPACE}" delete statefulset skytree --cascade=orphan --ignore-not-found=true
    kubectl apply -f "${MANIFEST_DIR}/skytree-statefulset.yaml"
    return 0
  fi

  echo "${output}" >&2
  return 1
}

wait_ready() {
  echo "waiting for Scylla"
  kubectl -n "${NAMESPACE}" rollout status statefulset/scylla --timeout=600s
  kubectl -n "${NAMESPACE}" wait pod -l app=scylla --for=condition=Ready --timeout=600s
  kubectl -n "${NAMESPACE}" wait --for=condition=complete job/scylla-init --timeout=300s
  echo "waiting for skyTree StatefulSet"
  kubectl -n "${NAMESPACE}" rollout status statefulset/skytree --timeout=300s
  echo "waiting for Prometheus"
  kubectl -n "${NAMESPACE}" rollout status deployment/prometheus --timeout=180s
  echo "waiting for Grafana"
  kubectl -n "${NAMESPACE}" rollout status deployment/grafana --timeout=180s
}

gateway_api_available() {
  kubectl api-resources --api-group=gateway.networking.k8s.io 2>/dev/null | awk '{print $1}' | grep -qx gatewayclasses
}

gateway_mqtt_host() {
  local address
  address="$(kubectl -n "${NAMESPACE}" get gateway "${GATEWAY_NAME}" \
    -o jsonpath='{.status.addresses[0].value}' 2>/dev/null || true)"
  if [[ -z "${address}" ]]; then
    address="$(kubectl get svc -A \
      -l "gateway.envoyproxy.io/owning-gateway-name=${GATEWAY_NAME}" \
      -o jsonpath='{range .items[*]}{.status.loadBalancer.ingress[0].ip}{.status.loadBalancer.ingress[0].hostname}{"\n"}{end}' 2>/dev/null | awk 'NF {print; exit}')"
  fi
  case "${address}" in
    ""|"<pending>")
      return 1
      ;;
    "0.0.0.0"|"::")
      echo "127.0.0.1"
      ;;
    *)
      echo "${address}"
      ;;
  esac
}

gateway_mqtt_candidates() {
  local host
  if host="$(gateway_mqtt_host)"; then
    echo "${host}"
  fi
  echo "127.0.0.1"
}

reachable_gateway_mqtt_host() {
  local host
  while read -r host; do
    if [[ -n "${host}" ]] && (echo >"/dev/tcp/${host}/${MQTT_GATEWAY_PORT}") >/dev/null 2>&1; then
      echo "${host}"
      return 0
    fi
  done < <(gateway_mqtt_candidates | awk '!seen[$0]++')
  return 1
}

wait_tcp_route_accepted() {
  local attempt
  local status

  for attempt in {1..120}; do
    status="$(kubectl -n "${NAMESPACE}" get tcproute skytree-mqtt \
      -o go-template='{{range .status.parents}}{{range .conditions}}{{if and (eq .type "Accepted") (eq .status "True")}}True{{end}}{{end}}{{end}}' 2>/dev/null || true)"
    if [[ "${status}" == *"True"* ]]; then
      return 0
    fi
    sleep 1
  done

  echo "timed out waiting for TCPRoute skytree-mqtt to be accepted" >&2
  kubectl -n "${NAMESPACE}" describe tcproute skytree-mqtt >&2 || true
  return 1
}

wait_gateway_ready() {
  echo "waiting for SkyTree MQTT Gateway"
  kubectl -n "${NAMESPACE}" wait "gateway/${GATEWAY_NAME}" --for=condition=Accepted --timeout=120s
  kubectl -n "${NAMESPACE}" wait "gateway/${GATEWAY_NAME}" --for=condition=Programmed --timeout=180s
  wait_tcp_route_accepted
}

wait_gateway_mqtt_port() {
  local attempt
  local host

  for attempt in {1..120}; do
    if host="$(reachable_gateway_mqtt_host)"; then
      echo "${host}" >"${MQTT_GATEWAY_HOST_FILE}"
      return 0
    fi
    sleep 1
  done

  echo "SkyTree MQTT Gateway did not open port ${MQTT_GATEWAY_PORT}" >&2
  kubectl -n "${NAMESPACE}" get gateway "${GATEWAY_NAME}" -o yaml >&2 || true
  kubectl get svc -A -l "gateway.envoyproxy.io/owning-gateway-name=${GATEWAY_NAME}" -o wide >&2 || true
  return 1
}

stop_port_forward() {
  local pid_file="$1"
  if [[ ! -f "${pid_file}" ]]; then
    return 0
  fi
  local pid
  pid="$(<"${pid_file}")"
  rm -f "${pid_file}"
  if [[ -n "${pid}" ]] && kill -0 "${pid}" >/dev/null 2>&1; then
    kill "${pid}" >/dev/null 2>&1 || true
  fi
}

node_port_forward_pid_file() {
  local node="$1"
  echo "${STATE_DIR}/skytree-${node}.port-forward.pid"
}

node_port_forward_log_file() {
  local node="$1"
  echo "${STATE_DIR}/skytree-${node}.port-forward.log"
}

node_spec_by_pod() {
  local pod="$1"
  local spec node resource mqtt_port health_port
  for spec in "${SKYTREE_NODE_PORT_FORWARDS[@]}"; do
    IFS=: read -r node resource mqtt_port health_port <<<"${spec}"
    case "${pod}" in
      "${node}"|"${node/node/node-}"|"${resource#pod/}")
        echo "${node}:${resource}:${mqtt_port}:${health_port}"
        return 0
        ;;
    esac
  done
  echo "unknown skyTree pod: ${pod}" >&2
  return 1
}

stop_node_port_forward() {
  local spec
  for spec in "${SKYTREE_NODE_PORT_FORWARDS[@]}"; do
    local node
    IFS=: read -r node _ <<<"${spec}"
    stop_port_forward "$(node_port_forward_pid_file "${node}")"
  done
}

wait_local_port() {
  local name="$1"
  local port="$2"
  local pid_file="$3"
  local log_file="$4"
  local attempt

  for attempt in {1..40}; do
    if [[ -f "${pid_file}" ]]; then
      local pid
      pid="$(<"${pid_file}")"
      if [[ -n "${pid}" ]] && ! kill -0 "${pid}" >/dev/null 2>&1; then
        echo "${name} port-forward exited unexpectedly; see ${log_file}" >&2
        return 1
      fi
    fi
    if (echo >"/dev/tcp/127.0.0.1/${port}") >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.25
  done

  echo "${name} port-forward did not open 127.0.0.1:${port}; see ${log_file}" >&2
  return 1
}

start_named_port_forward() {
  local name="$1"
  local resource="$2"
  local pid_file="$3"
  local log_file="$4"
  shift 4
  nohup kubectl -n "${NAMESPACE}" port-forward --address 127.0.0.1 "${resource}" "$@" >"${log_file}" 2>&1 &
  local pid="$!"
  echo "${pid}" >"${pid_file}"
  disown "${pid}" >/dev/null 2>&1 || true
}

start_node_port_forward() {
  local spec
  for spec in "${SKYTREE_NODE_PORT_FORWARDS[@]}"; do
    local node resource mqtt_port health_port
    IFS=: read -r node resource mqtt_port health_port <<<"${spec}"
    local pid_file log_file
    pid_file="$(node_port_forward_pid_file "${node}")"
    log_file="$(node_port_forward_log_file "${node}")"
    start_named_port_forward "skyTree ${node}" "${resource}" "${pid_file}" "${log_file}" \
      "${mqtt_port}:1883" "${health_port}:9526"
    wait_local_port "skyTree ${node} MQTT" "${mqtt_port}" "${pid_file}" "${log_file}"
    wait_local_port "skyTree ${node} health" "${health_port}" "${pid_file}" "${log_file}"
  done
}

start_single_node_port_forward() {
  local pod="$1"
  local spec node resource mqtt_port health_port
  spec="$(node_spec_by_pod "${pod}")"
  IFS=: read -r node resource mqtt_port health_port <<<"${spec}"
  local pid_file log_file
  pid_file="$(node_port_forward_pid_file "${node}")"
  log_file="$(node_port_forward_log_file "${node}")"
  stop_port_forward "${pid_file}"
  start_named_port_forward "skyTree ${node}" "${resource}" "${pid_file}" "${log_file}" \
    "${mqtt_port}:1883" "${health_port}:9526"
  wait_local_port "skyTree ${node} MQTT" "${mqtt_port}" "${pid_file}" "${log_file}"
  wait_local_port "skyTree ${node} health" "${health_port}" "${pid_file}" "${log_file}"
}

stop_single_node_port_forward() {
  local pod="$1"
  local spec node
  spec="$(node_spec_by_pod "${pod}")"
  IFS=: read -r node _ <<<"${spec}"
  stop_port_forward "$(node_port_forward_pid_file "${node}")"
}

start_port_forward() {
  mkdir -p "${STATE_DIR}"
  stop_port_forward "${API_PORT_FORWARD_PID}"
  stop_port_forward "${MQTT_PORT_FORWARD_PID}"
  stop_port_forward "${PROMETHEUS_PORT_FORWARD_PID}"
  stop_port_forward "${GRAFANA_PORT_FORWARD_PID}"
  stop_node_port_forward
  echo "starting port-forward for console, skyTree nodes, Prometheus, and Grafana"
  nohup kubectl -n "${NAMESPACE}" port-forward --address 127.0.0.1 svc/skytree-api 9526:9526 >"${API_PORT_FORWARD_LOG}" 2>&1 &
  local api_pid="$!"
  echo "${api_pid}" >"${API_PORT_FORWARD_PID}"
  disown "${api_pid}" >/dev/null 2>&1 || true
  start_node_port_forward
  nohup kubectl -n "${NAMESPACE}" port-forward --address 127.0.0.1 svc/prometheus "${PROMETHEUS_LOCAL_PORT}:9090" >"${PROMETHEUS_PORT_FORWARD_LOG}" 2>&1 &
  local prometheus_pid="$!"
  echo "${prometheus_pid}" >"${PROMETHEUS_PORT_FORWARD_PID}"
  disown "${prometheus_pid}" >/dev/null 2>&1 || true
  nohup kubectl -n "${NAMESPACE}" port-forward --address 127.0.0.1 svc/grafana "${GRAFANA_LOCAL_PORT}:3000" >"${GRAFANA_PORT_FORWARD_LOG}" 2>&1 &
  local grafana_pid="$!"
  echo "${grafana_pid}" >"${GRAFANA_PORT_FORWARD_PID}"
  disown "${grafana_pid}" >/dev/null 2>&1 || true
  wait_local_port "console" 9526 "${API_PORT_FORWARD_PID}" "${API_PORT_FORWARD_LOG}"
  wait_local_port "Prometheus" "${PROMETHEUS_LOCAL_PORT}" "${PROMETHEUS_PORT_FORWARD_PID}" "${PROMETHEUS_PORT_FORWARD_LOG}"
  wait_local_port "Grafana" "${GRAFANA_LOCAL_PORT}" "${GRAFANA_PORT_FORWARD_PID}" "${GRAFANA_PORT_FORWARD_LOG}"
}

print_status() {
  if [[ "${JSON_OUTPUT}" == "true" ]]; then
    kubectl -n "${NAMESPACE}" get pods,svc,statefulset,deploy,job -o json
    return 0
  fi
  kubectl get pods,svc,statefulset,deploy,job -n "${NAMESPACE}"
  if gateway_api_available; then
    kubectl get gateway,tcproute -n "${NAMESPACE}" || true
  fi
  echo
  echo "Console: http://127.0.0.1:9526/console"
  if [[ -f "${MQTT_GATEWAY_HOST_FILE}" ]]; then
    host="$(<"${MQTT_GATEWAY_HOST_FILE}")"
    echo "MQTT:    mqtt://${host}:${MQTT_GATEWAY_PORT}"
  elif host="$(reachable_gateway_mqtt_host)"; then
    echo "MQTT:    mqtt://${host}:${MQTT_GATEWAY_PORT}"
  else
    echo "MQTT:    Gateway address is pending"
  fi
  echo "Gardener node1: mqtt://127.0.0.1:11883  http://127.0.0.1:19526/health"
  echo "Gardener node2: mqtt://127.0.0.1:21883  http://127.0.0.1:29526/health"
  echo "Gardener node3: mqtt://127.0.0.1:31883  http://127.0.0.1:39526/health"
  echo "Metrics: http://127.0.0.1:${PROMETHEUS_LOCAL_PORT}"
  echo "Grafana: http://127.0.0.1:${GRAFANA_LOCAL_PORT}/d/skytree-overview/skytree-overview"
}

print_leader() {
  require_command curl
  curl -fsS -u admin:secret "http://127.0.0.1:9526/api/v1/cluster/overview"
  echo
}

pod_resource_name() {
  local pod="$1"
  local spec resource
  spec="$(node_spec_by_pod "${pod}")"
  IFS=: read -r _ resource _ _ <<<"${spec}"
  echo "${resource#pod/}"
}

stop_pod() {
  local pod="$1"
  local pod_name
  pod_name="$(pod_resource_name "${pod}")"
  stop_single_node_port_forward "${pod_name}"
  kubectl -n "${NAMESPACE}" delete pod "${pod_name}" --ignore-not-found=true --wait=false
  if [[ "${JSON_OUTPUT}" == "true" ]]; then
    printf '{"namespace":"%s","pod":"%s","action":"stop-pod"}\n' "${NAMESPACE}" "${pod_name}"
  fi
}

recover_pod() {
  local pod="$1"
  local pod_name
  pod_name="$(pod_resource_name "${pod}")"
  kubectl -n "${NAMESPACE}" wait "pod/${pod_name}" --for=condition=Ready --timeout=300s
  start_single_node_port_forward "${pod_name}"
  if [[ "${JSON_OUTPUT}" == "true" ]]; then
    printf '{"namespace":"%s","pod":"%s","action":"recover-pod"}\n' "${NAMESPACE}" "${pod_name}"
  fi
}

restart_pod() {
  local pod="$1"
  stop_pod "${pod}"
  recover_pod "${pod}"
}

delete_resources() {
  stop_port_forward "${API_PORT_FORWARD_PID}"
  stop_port_forward "${MQTT_PORT_FORWARD_PID}"
  stop_port_forward "${PROMETHEUS_PORT_FORWARD_PID}"
  stop_port_forward "${GRAFANA_PORT_FORWARD_PID}"
  stop_node_port_forward
  rm -f "${MQTT_GATEWAY_HOST_FILE}"
  kubectl delete -f "${MANIFEST_DIR}/skytree-gateway.yaml" --ignore-not-found=true || true
  kubectl delete -f "${MANIFEST_DIR}/skytree-statefulset.yaml" --ignore-not-found=true
  kubectl delete -f "${MANIFEST_DIR}/services.yaml" --ignore-not-found=true
  kubectl delete -f "${MANIFEST_DIR}/skytree-config.yaml" --ignore-not-found=true
  kubectl delete -f "${MANIFEST_DIR}/grafana.yaml" --ignore-not-found=true
  kubectl -n "${NAMESPACE}" delete configmap "${GRAFANA_DASHBOARD_CONFIGMAP}" --ignore-not-found=true
  kubectl delete -f "${MANIFEST_DIR}/prometheus.yaml" --ignore-not-found=true
  kubectl delete -f "${MANIFEST_DIR}/scylla.yaml" --ignore-not-found=true
  kubectl delete -f "${MANIFEST_DIR}/rbac.yaml" --ignore-not-found=true
  kubectl delete -f "${MANIFEST_DIR}/namespace.yaml" --ignore-not-found=true
}

follow_logs() {
  kubectl logs -n "${NAMESPACE}" -l app=skytree -f --max-log-requests=10
}

require_command kubectl

if [[ "${STATUS}" == "true" ]]; then
  print_status
  exit 0
fi

if [[ "${WAIT_READY}" == "true" ]]; then
  wait_ready
  wait_gateway_ready
  wait_gateway_mqtt_port
  exit 0
fi

if [[ "${LEADER}" == "true" ]]; then
  print_leader
  exit 0
fi

if [[ -n "${STOP_POD}" ]]; then
  stop_pod "${STOP_POD}"
  exit 0
fi

if [[ -n "${RESTART_POD}" ]]; then
  restart_pod "${RESTART_POD}"
  exit 0
fi

if [[ -n "${RECOVER_POD}" ]]; then
  recover_pod "${RECOVER_POD}"
  exit 0
fi

if [[ "${DOWN}" == "true" ]]; then
  delete_resources
  exit 0
fi

require_command docker
require_command go
build_local_skyTree_binary
build_local_console_assets
build_image
load_image_to_cluster
install_envoy_gateway
apply_manifests
wait_ready
wait_gateway_ready
stop_port_forward "${MQTT_PORT_FORWARD_PID}"
wait_gateway_mqtt_port
start_port_forward
print_status

if [[ "${LOGS}" == "true" ]]; then
  follow_logs
fi
