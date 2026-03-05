#!/usr/bin/env bash
set -euo pipefail

KIND_CLUSTER="${KIND_CLUSTER:-kattack}"
NAMESPACE="${NAMESPACE:-default}"
CR_NAME="${CR_NAME:-vegeta-log-check}"
IMG="${IMG:-controller:dev}"
TIMEOUT="${TIMEOUT:-300s}"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

require_cmd kind
require_cmd kubectl
require_cmd make
require_cmd docker

if ! kind get clusters | grep -qx "${KIND_CLUSTER}"; then
  echo "Creating kind cluster: ${KIND_CLUSTER}"
  kind create cluster --name "${KIND_CLUSTER}"
fi

# Use this kind cluster context explicitly.
kubectl config use-context "kind-${KIND_CLUSTER}" >/dev/null

echo "Building controller image: ${IMG}"
make docker-build IMG="${IMG}"

echo "Loading image into kind cluster: ${KIND_CLUSTER}"
kind load docker-image "${IMG}" --name "${KIND_CLUSTER}"

echo "Checking cluster service networking (kube-proxy + CoreDNS)"
if ! kubectl -n kube-system rollout status daemonset/kube-proxy --timeout=120s; then
  echo "kube-proxy is not healthy; service networking is broken." >&2
  kubectl -n kube-system get pods -o wide >&2 || true
  kubectl -n kube-system logs daemonset/kube-proxy --tail=100 >&2 || true
  exit 1
fi
if ! kubectl -n kube-system rollout status deployment/coredns --timeout=120s; then
  echo "CoreDNS is not healthy; DNS/service discovery is broken." >&2
  kubectl -n kube-system get pods -o wide >&2 || true
  kubectl -n kube-system logs deployment/coredns --tail=100 >&2 || true
  exit 1
fi

echo "Installing CRDs and deploying operator image: ${IMG}"
make install
make deploy IMG="${IMG}"

kubectl wait --for=condition=Available deployment/kattack-controller-manager \
  -n kattack-system --timeout="${TIMEOUT}"

echo "Applying VegetaLoadTest: ${CR_NAME}"
cat <<MANIFEST | kubectl apply -f -
apiVersion: loadtest.io/v1alpha1
kind: VegetaLoadTest
metadata:
  name: ${CR_NAME}
  namespace: ${NAMESPACE}
spec:
  report:
    type: text
  attack:
    rate: "5/1s"
    duration: 10s
    timeout: 5s
  targets:
    - name: homepage
      url: http://dummy-health-service:80/healthz
      method: GET
MANIFEST

echo "Waiting for job pod"
for _ in $(seq 1 60); do
  POD_NAME="$(kubectl get pods -n "${NAMESPACE}" -l "vegetaloadtest=${CR_NAME}" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
  if [[ -n "${POD_NAME}" ]]; then
    break
  fi
  sleep 2
done

if [[ -z "${POD_NAME:-}" ]]; then
  echo "No runner pod found for ${CR_NAME}" >&2
  exit 1
fi

echo "Waiting for pod completion: ${POD_NAME}"
kubectl wait --for=condition=Ready=false pod/"${POD_NAME}" -n "${NAMESPACE}" --timeout=1s >/dev/null 2>&1 || true
kubectl wait --for=jsonpath='{.status.phase}'=Succeeded pod/"${POD_NAME}" -n "${NAMESPACE}" --timeout="${TIMEOUT}" || true

echo
 echo "=== Runner Pod Logs (${POD_NAME}) ==="
kubectl logs -n "${NAMESPACE}" "${POD_NAME}"

echo
 echo "=== VegetaLoadTest Status (${CR_NAME}) ==="
kubectl get vegetaloadtest "${CR_NAME}" -n "${NAMESPACE}" -o yaml
