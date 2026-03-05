# Contributing

By participating in this project, you agree to follow [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

## Pull Request Process

1. Create a feature branch from `main`.
2. Keep commits focused and include tests for behavior changes.
3. Run `make manifests generate fmt vet` and `go test ./... -count=1`.
4. Open a PR with:
   - problem statement
   - implementation summary
   - test evidence
5. Address review comments and keep branch rebased.

## Issues and Discussions

- Use GitHub Issues for bugs and feature requests.
- Use GitHub Discussions for design and usage questions.
- Follow issue and PR templates to include reproducible context.

## Code Style

- Use `go fmt` formatting.
- Keep reconciler logic deterministic and idempotent.
- Prefer table-driven tests for parser/evaluator/scheduler units.
- Add concise comments only where intent is non-obvious.

## Local Validation

```bash
make manifests generate fmt vet
go build ./...
go test ./... -v
```

## Kind Image Pull Troubleshooting

If the controller pod fails with `ErrImagePull` for `controller:latest`, build and load the image into Kind:

```bash
make docker-build IMG=controller:latest
kind load docker-image controller:latest --name kind
make deploy IMG=controller:latest
kubectl rollout restart deployment/kattack-controller-manager -n kattack-system
kubectl rollout status deployment/kattack-controller-manager -n kattack-system
```
