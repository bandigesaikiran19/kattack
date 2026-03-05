#!/usr/bin/env bash
set -euo pipefail

KIND_CLUSTER="${KIND_CLUSTER:-kattack}"
NAMESPACE="${NAMESPACE:-default}"
CR_NAME="${CR_NAME:-vegeta-log-check}"
IMG="${IMG:-controller:dev}"
DELETE_CLUSTER="${DELETE_CLUSTER:-true}"
REMOVE_LOCAL_IMAGE="${REMOVE_LOCAL_IMAGE:-false}"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

require_cmd kind
require_cmd kubectl
require_cmd make

if [[ "${REMOVE_LOCAL_IMAGE}" == "true" ]]; then
  require_cmd docker
fi

cluster_exists=false
if kind get clusters | grep -qx "${KIND_CLUSTER}"; then
  cluster_exists=true
fi

if [[ "${cluster_exists}" == "true" ]]; then
  echo "Using kind context: kind-${KIND_CLUSTER}"
  kubectl config use-context "kind-${KIND_CLUSTER}" >/dev/null

  echo "Deleting VegetaLoadTest and runner resources"
  kubectl delete vegetaloadtest "${CR_NAME}" -n "${NAMESPACE}" --ignore-not-found=true || true
  kubectl delete jobs -n "${NAMESPACE}" -l "vegetaloadtest=${CR_NAME}" --ignore-not-found=true || true
  kubectl delete pods -n "${NAMESPACE}" -l "vegetaloadtest=${CR_NAME}" --ignore-not-found=true || true

  echo "Undeploying operator and uninstalling CRDs"
  make undeploy ignore-not-found=true || true
  make uninstall ignore-not-found=true || true
fi

if [[ "${DELETE_CLUSTER}" == "true" ]]; then
  if [[ "${cluster_exists}" == "true" ]]; then
    echo "Deleting kind cluster: ${KIND_CLUSTER}"
    kind delete cluster --name "${KIND_CLUSTER}"
  else
    echo "Kind cluster not found: ${KIND_CLUSTER}"
  fi
else
  echo "Skipping cluster deletion (DELETE_CLUSTER=${DELETE_CLUSTER})"
fi

if [[ "${REMOVE_LOCAL_IMAGE}" == "true" ]]; then
  echo "Removing local image: ${IMG}"
  docker rmi "${IMG}" >/dev/null 2>&1 || true
fi

echo "Cleanup complete."
