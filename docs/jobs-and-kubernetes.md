# Jobs and Kubernetes Workflow

This document explains how `kattack` creates and manages Kubernetes Jobs from a `VegetaLoadTest` custom resource, and how to operate/debug that flow in a cluster.

## Scope

- Controller deployment lifecycle in Kubernetes
- `VegetaLoadTest` to Job/Pod orchestration
- Job naming, labels, execution behavior
- Status updates, retries, and cleanup
- Common kubectl operations for troubleshooting

## Control Plane Resources

When deployed with `make deploy`, the operator installs these core resources:

- Namespace: `kattack-system`
- Deployment: `kattack-controller-manager`
- ServiceAccount and RBAC for CRD, Jobs, Pods, Pod logs, Events
- CRD: `vegetaloadtests.loadtest.io`

Runner Jobs and Pods are created in the namespace where the `VegetaLoadTest` resource exists.

## Kind + Image Flow

1. Ensure Kind cluster exists (`KIND_CLUSTER`).
2. Switch kube context to `kind-$KIND_CLUSTER`.
3. Build controller image (`make docker-build IMG=...`).
4. Load that image into Kind (`kind load docker-image ...`).
5. Validate cluster networking (kube-proxy/CoreDNS rollout).
6. Install CRDs (`make install`).
7. Deploy controller using the same image tag (`make deploy IMG=...`).

This guarantees the deployed controller uses your local code changes.

## CR to Job Lifecycle

Each `VegetaLoadTest` runs through these phases in reconcile:

1. Finalizer is added for cleanup safety.
2. Status enters `Running` and target counters are initialized.
3. For each target in order:
4. Controller checks if a matching Job exists.
5. If not, it creates one runner Job.
6. It waits while the Job is active.
7. When finished, it reads Pod logs, parses Vegeta report output, and writes `status.results`.
8. Thresholds are evaluated and pass/fail is recorded.
9. Retries may occur based on `spec.executionPolicy.retries`.
10. After all targets complete, CR phase becomes `Completed` or `Failed`.

## Job Naming and Labels

For target index `i`, Job name is:

- `<cr-name>-<target-name>-<i>`

Default labels:

- `app=kattack`
- `vegetaloadtest=<cr-name>`
- `target=<target-name>`

These labels are the primary selectors for fetching Jobs and Pods.

## Runner Pod Command Behavior

The runner container executes a generated shell command that:

1. Prints target metadata line in pod logs.
2. Runs `vegeta attack ... > report.bin`.
3. Prints report stage marker.
4. Runs `vegeta report -type=<...> report.bin`.
5. Optionally mirrors report output to `spec.report.output` via `tee`.
6. Optionally uploads `report.bin` to remote storage if `spec.storage` is configured.

If warmup is enabled, a warmup attack is executed before the main run.

## Status and Result Fields

The controller updates:

- `status.phase` (`Pending`, `Running`, `Completed`, `Failed`, `Scheduled`, `Suspended`)
- `status.currentTarget`
- `status.targetsTotal`, `status.targetsCompleted`, `status.targetsPassed`, `status.targetsFailed`
- `status.results[]` with per-target latency, throughput, success rate, thresholds
- `status.completionTime`, `status.overallSuccessRate`

## Retry and Failure Semantics

- Job failure can trigger retry if retries remain.
- Retry count is tracked on the CR using annotations.
- If retries are exhausted, target phase is set to `Failed`.
- `continueOnFailure` and threshold failures can skip later targets.

## Cleanup and Retention

- Finalizer: `loadtest.io/cleanup`
- On CR deletion, owned Jobs are cleaned up before finalizer removal.
- Job TTL can be controlled via `spec.ttlSecondsAfterFinished`.
- History cleanup is applied based on configured history limit policy.

## Kubernetes Operations Cheat Sheet

Apply a test CR:

```bash
kubectl apply -f config/samples/loadtest_v1alpha1_simple.yaml
```

Watch CR status:

```bash
kubectl get vegetaloadtest -n default -w
```

Inspect a CR in detail:

```bash
kubectl get vegetaloadtest <name> -n <namespace> -o yaml
kubectl describe vegetaloadtest <name> -n <namespace>
```

List Jobs for a load test:

```bash
kubectl get jobs -n <namespace> -l vegetaloadtest=<name>
```

List Pods for a load test:

```bash
kubectl get pods -n <namespace> -l vegetaloadtest=<name>
```

Read runner logs:

```bash
kubectl logs -n <namespace> <pod-name>
```

Operator logs:

```bash
kubectl logs -n kattack-system deployment/kattack-controller-manager -c manager
```

Events for troubleshooting:

```bash
kubectl get events -n <namespace> --sort-by=.metadata.creationTimestamp
```

## Troubleshooting Patterns

- No Job created:
  - Check operator logs and CR events.
  - Verify CRD exists and CR spec is valid.
- Job created but no Pod:
  - Check namespace quotas, scheduling constraints, image pull issues.
- Pod runs but result not parsed:
  - Check pod logs for final JSON/text report output.
  - Ensure report output is present in logs.
- Image update not reflected in Kind:
  - Re-run docker build + `kind load docker-image`.
  - Confirm deployment image value with `kubectl get deploy -n kattack-system -o yaml`.
