# kattack
`kattack` is a **Kubernetes load testing operator** for **declarative HTTP performance testing** with Vegeta.
It runs **in-cluster load tests** as Kubernetes Jobs and writes structured results into Custom Resource status.

## Description

`kattack` lets you define HTTP load tests as Kubernetes custom resources and execute them inside the cluster.
It turns ephemeral load-test logs into structured Kubernetes status that can be used for automation and release gates.

## Overview

- Define a `VegetaLoadTest` custom resource.
- Apply CRD + controller manifests to the cluster.
- Run load using the provided scripts.
- Read pass/fail and per-target metrics from `status.results[]`.

**kattack helps Platform Engineering, SRE, and DevOps teams** automate:
- Kubernetes performance testing
- API load testing and stress testing
- release gating with threshold-based pass/fail
- GitOps/CI/CD validation before production rollout

## Problem We Solve

Most teams run load tests from laptops/CI and treat results as temporary logs. That creates three gaps:

- Not Kubernetes-native: traffic path and runtime conditions differ from in-cluster reality
- Not queryable: outputs disappear in pod/CI logs instead of living in Kubernetes status
- Not automatable: no reliable pass/fail signal for release gates

`kattack` solves this by running Vegeta in-cluster, persisting per-phase results in `status.results[]`, and evaluating thresholds into machine-readable pass/fail status.

## Why use it

- Kubernetes-native load testing (same network/DNS/runtime conditions as workloads)
- CRD-based load testing (`VegetaLoadTest`) for declarative operations
- Multi-phase sequential traffic profiles (`spec.targets[]`)
- Automated threshold evaluation for SLO/SLA-style checks
- Machine-readable pass/fail for deployment approval workflows
- Readable runner logs + structured `status.results[]` for observability
- Retry, warmup, wait controls, and cron scheduling for production-like test workflows
- Optional webhook/Slack/email notifications and storage integrations

## Prerequisites

- Kubernetes (`>= 1.28`)
- `kubectl`, `make`, `docker`
- `kind` (recommended for local dev)
- Go `>= 1.22` (for development)

## Quick Start (Kind)

```bash
# from repo root
./scripts/start_and_test.sh
```

What it does:
1. Creates/uses a Kind cluster
2. Builds controller image and loads it into Kind
3. Installs CRDs and deploys operator
4. Applies a sample `VegetaLoadTest`
5. Prints runner logs and CR status

## Apply CRD (manual)

If you want to install only the CRD directly:

```bash
kubectl apply -f config/crd/bases/loadtest.io_vegetaloadtests.yaml
```

Or install CRDs via Make:

```bash
make install
```

## Run Load Using Script (recommended)

Run end-to-end load flow with script:

```bash
./scripts/start_and_test.sh
```

Build/deploy and run a specific load manifest:

```bash
KIND_CLUSTER=kattack APPLY_RUN=true \
RUN_FILE=config/samples/loadtest_v1alpha1_dummy_healthz.yaml \
./scripts/build_and_apply.sh
```

## Run only build + deploy

```bash
KIND_CLUSTER=kattack ./scripts/build_and_apply.sh
```

Apply a test run as part of deploy:

```bash
KIND_CLUSTER=kattack APPLY_RUN=true \
RUN_FILE=config/samples/loadtest_v1alpha1_dummy_healthz.yaml \
./scripts/build_and_apply.sh
```

## Helm Chart Option

Chart location:
- `charts/kattack` (chart name is `kattack`)

Install:

```bash
helm upgrade --install kattack ./charts/kattack \
  --namespace kattack-system \
  --create-namespace
```

Preview rendered manifests before install:

```bash
helm template kattack ./charts/kattack \
  --namespace kattack-system
```

Install with custom image:

```bash
helm upgrade --install kattack ./charts/kattack \
  --namespace kattack-system \
  --create-namespace \
  --set image.repository=ghcr.io/loadtest-io/kattack \
  --set image.tag=latest \
  --set image.pullPolicy=IfNotPresent
```

Install using a values file:

```bash
helm upgrade --install kattack ./charts/kattack \
  --namespace kattack-system \
  --create-namespace \
  -f charts/kattack/values.yaml
```

Verify deployment:

```bash
kubectl get pods -n kattack-system
kubectl rollout status deployment/kattack-controller-manager -n kattack-system
kubectl logs -n kattack-system deployment/kattack-controller-manager -c manager
```

Upgrade after chart/value changes:

```bash
helm upgrade kattack ./charts/kattack \
  --namespace kattack-system \
  -f charts/kattack/values.yaml
```

Uninstall:

```bash
helm uninstall kattack -n kattack-system
```

## Options You Can Use

Deployment options:
- Kustomize/Make workflow (`make install`, `make deploy`)
- Helm chart deployment (`charts/kattack`)
- Local Kind loop with image build/load scripts

Execution options in CR:
- Single or multi-phase targets (`spec.targets[]`)
- Per-target overrides (`attackOverride`, thresholds, `waitAfter`, `continueOnFailure`)
- Warmup/retries (`spec.executionPolicy`)
- Cron-based runs (`spec.schedule`)
- Notifications (`slack`, `webhook`, `email`)
- Report/storage controls (`spec.report`, `spec.storage`)

Operational options:
- Full run helper: `./scripts/start_and_test.sh`
- Build/deploy helper: `./scripts/build_and_apply.sh`
- Cleanup helper: `./scripts/cleanup_start_and_test.sh`

## Useful commands

```bash
# list load tests
kubectl get vegetaloadtests -A

# inspect one CR
kubectl get vegetaloadtest <name> -n <ns> -o yaml

# list runner jobs/pods for a CR
kubectl get jobs -n <ns> -l vegetaloadtest=<name>
kubectl get pods -n <ns> -l vegetaloadtest=<name>

# runner logs
kubectl logs -n <ns> <pod-name>

# operator logs
kubectl logs -n kattack-system deployment/kattack-controller-manager -c manager
```

## Cleanup

```bash
./scripts/cleanup_start_and_test.sh
```

Keep cluster but cleanup resources:

```bash
DELETE_CLUSTER=false ./scripts/cleanup_start_and_test.sh
```

## Samples

- Rich sample (covers most CRD fields):
  - `config/samples/loadtest_v1alpha1_dummy_healthz.yaml`
- Basic sample:
  - `config/samples/loadtest_v1alpha1_simple.yaml`

## Documentation

- Deep technical details + flowcharts: [details.md](details.md)
- Jobs and Kubernetes workflow: [docs/jobs-and-kubernetes.md](docs/jobs-and-kubernetes.md)
- Comparison vs dastergon operator: [docs/what-we-built-vs-dastergon.md](docs/what-we-built-vs-dastergon.md)
- AI handoff prompt: [docs/ai-handoff-prompt.md](docs/ai-handoff-prompt.md)


## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Community and Security

- Code of Conduct: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- Security Policy: [SECURITY.md](SECURITY.md)
- Support: [SUPPORT.md](SUPPORT.md)
