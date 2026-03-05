#!/usr/bin/env bash
set -euo pipefail

IMG="${IMG:-controller:dev}"
KIND_CLUSTER="${KIND_CLUSTER:-}"
TIMEOUT="${TIMEOUT:-300s}"
APPLY_RUN="${APPLY_RUN:-false}"
RUN_FILE="${RUN_FILE:-config/samples/loadtest_v1alpha1_dummy_healthz.yaml}"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

require_cmd make
require_cmd kubectl
require_cmd docker

echo "Building controller image: ${IMG}"
make docker-build IMG="${IMG}"

if [[ -n "${KIND_CLUSTER}" ]]; then
  require_cmd kind
  if ! kind get clusters | grep -qx "${KIND_CLUSTER}"; then
    echo "Kind cluster not found: ${KIND_CLUSTER}" >&2
    exit 1
  fi
  echo "Using kind context: kind-${KIND_CLUSTER}"
  kubectl config use-context "kind-${KIND_CLUSTER}" >/dev/null
  echo "Loading image into kind cluster: ${KIND_CLUSTER}"
  kind load docker-image "${IMG}" --name "${KIND_CLUSTER}"
fi

echo "Installing CRDs"
make install

echo "Deploying controller image: ${IMG}"
make deploy IMG="${IMG}"

kubectl wait --for=condition=Available deployment/kattack-controller-manager \
  -n kattack-system --timeout="${TIMEOUT}"

if [[ "${APPLY_RUN}" == "true" ]]; then
  echo "Applying run manifest: ${RUN_FILE}"
  kubectl apply -f "${RUN_FILE}"
fi

echo "Build and apply completed."
