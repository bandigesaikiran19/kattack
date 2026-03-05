# kattack vs dastergon/vegeta-operator

This document summarizes practical differences between `kattack` and the
original `dastergon/vegeta-operator` line.

## Positioning

- `kattack` focuses on platform-grade, declarative load testing workflows for
  CI/CD and Kubernetes-native operations.
- It keeps the CRD-driven model and Vegeta core while extending execution and
  status capabilities.

## Notable capabilities in kattack

- Multi-phase sequential targets in a single CR (`spec.targets[]`)
- Threshold-based pass/fail recorded in status
- Retry and warmup policies for realistic runs
- Cron scheduling support (`spec.schedule`)
- Optional report storage and notification integrations
- Structured per-target results under `status.results[]`

## Migration notes

- API group remains `loadtest.io/v1alpha1`.
- Existing manifests should be validated against current sample specs under
  `config/samples/`.
- Image, release process, and chart defaults are project-specific to `kattack`.

## Choosing between projects

Use `kattack` when you need Kubernetes-native automation, stronger status-driven
release gating, and operational workflows around retries/scheduling/notifications.
