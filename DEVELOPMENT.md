# Development

- [Development](#development)
  - [Development Environment Setup](#development-environment-setup)
    - [golang](#golang)
    - [prek (pre-commit hooks)](#prek-pre-commit-hooks)
  - [Makefile](#makefile)
  - [Code Generation](#code-generation)
  - [Build using boilerplate container](#build-using-boilerplate-container)
  - [Mocks](#mocks)

This document covers everything you need to develop this operator locally.

## Development Environment Setup

### golang

Go 1.25.4 or newer is required (see `go.mod`).

```bash
$ go version
go version go1.25.4 linux/amd64
```

**Note**: `FIPS_ENABLED=true` is set by the Makefile, which requires `GOEXPERIMENT=boringcrypto` and may fail outside the CI container. For local Go builds, use `go build .` directly or `make container-test` to build inside the boilerplate container.

### prek (pre-commit hooks)

This project uses [prek](https://prek.j178.dev/) for git hook management.

```bash
# Install prek
uv tool install prek      # recommended
# or: pipx install prek

# Wire up git hooks
prek install
```

## Makefile

Common make targets:

```bash
# Build, lint, and test (default)
make

# Individual targets
make go-check          # golangci-lint + other static analysis
make go-test           # Unit tests (requires envtest)
make go-build          # Build binary (FIPS-enabled, may fail outside container)
make lint              # YAML validation + go-check
make validate          # Ensure generated code is committed
make generate          # CRDs, deepcopy, openapi-gen, mocks
make coverage          # Code coverage report
make run               # Run operator locally (requires kubeconfig with Hive CRDs)

# Container-based targets (run inside boilerplate container, matches CI)
make container-test
make container-lint
make container-validate
make container-all
```

## Code Generation

After modifying CRD types (`api/v1alpha1/`) or the DMS client interface (`pkg/dmsclient/dmsclient.go`):

```bash
# Regenerate CRDs, deepcopy, openapi, mocks (use container for CI parity)
boilerplate/_lib/container-make generate

# Verify generated files are committed (what CI runs)
make generate-check
```

Generated files that must be committed:
- `api/v1alpha1/zz_generated.deepcopy.go`
- `api/v1alpha1/zz_generated.openapi.go`
- `pkg/dmsclient/mock/mock_dmsclient.go`
- `deploy/crds/*.yaml`

## Build using boilerplate container

To run lint, test and build in the boilerplate container (matches CI environment):

```bash
boilerplate/_lib/container-make TARGET
```

Examples:

```bash
# Run unit tests
boilerplate/_lib/container-make go-test

# Run lint
boilerplate/_lib/container-make go-check

# Run coverage
boilerplate/_lib/container-make coverage

# Run all validation
boilerplate/_lib/container-make container-all
```

## Mocks

The DMS API client mock lives at `pkg/dmsclient/mock/mock_dmsclient.go`, generated from the
`Client` interface in `pkg/dmsclient/dmsclient.go` using `go.uber.org/mock/mockgen`.

**Do not edit the mock directly.** Regenerate it with:

```bash
boilerplate/_lib/container-make generate
```
