# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

The deadmanssnitch-operator manages Dead Man's Snitch (DMS) heartbeat monitoring for OpenShift Dedicated (OSD) clusters. It runs on Hive, watches `ClusterDeployment` resources, and for each installed managed cluster: creates a snitch via the DMS API, stores the check-in URL in a Secret, and syncs it into the target cluster's `openshift-monitoring` namespace via a `SyncSet`.

## Build & Test Commands

The build system uses OpenShift boilerplate (`.mk` files in `boilerplate/`). FIPS is enabled by default, which requires `GOEXPERIMENT=boringcrypto` and generally only builds inside the provided Dockerfile.

```bash
# Build, lint, and test (default target)
make

# Individual targets
make go-check          # golangci-lint
make go-test           # Unit tests (uses setup-envtest for kubebuilder assets)
make go-build          # Build binary (FIPS-enabled, may fail outside container)
make lint              # YAML validation + go-check
make validate          # Ensures generated code is committed and boilerplate is frozen
make generate          # Run all code generation (CRDs, deepcopy, openapi, mocks)

# Run a single test
KUBEBUILDER_ASSETS=$(setup-envtest use 1.28.0 --arch amd64 --os linux --bin-dir /tmp/envtest-binaries -p path) \
  go test -run TestReconcileClusterDeployment ./controllers/deadmanssnitchintegration/

# Run locally (requires kubeconfig with Hive CRDs)
make run

# Container-based targets (run inside boilerplate container)
make container-test
make container-lint
make container-validate
make container-all          # All container validations in sequence

# PKO template validation (requires kubectl-package CLI)
kubectl-package validate deploy_pko/
```

## Architecture

**Single controller** (`DeadmansSnitchIntegrationReconciler`) managing one CRD:

- **CRD**: `DeadmansSnitchIntegration` (shortName: `dmsi`, group: `deadmanssnitch.managed.openshift.io/v1alpha1`)
- **Entry point**: `main.go` -- sets up manager, leader election, custom metrics server (port 8081)
- **Controller**: `controllers/deadmanssnitchintegration/` -- reconciles DMSI CRs
- **Event handlers**: `event_handlers.go` -- maps ClusterDeployment/Secret/SyncSet changes back to DMSI reconcile requests

### Reconciliation flow

1. Fetch DMSI CR and load DMS API key from referenced Secret
2. List ClusterDeployments matching `spec.clusterDeploymentSelector`
3. For each matching CD that is installed and not hibernating:
   - Add finalizer (`dms.managed.openshift.io/deadmanssnitch-{postfix}`)
   - Create/find snitch via DMS API
   - Create Secret with snitch check-in URL
   - Create SyncSet to sync Secret into cluster's `openshift-monitoring` namespace
4. On deletion/hibernation: delete snitch, remove Secret, SyncSet, and finalizer

### Key packages

- `pkg/dmsclient/` -- DMS API client with `Client` interface; mock at `pkg/dmsclient/mock/` (generated via `go.uber.org/mock`)
- `config/` -- Operator constants (namespace, label names, SyncSet postfixes) and FedRAMP env var handling
- `pkg/localmetrics/` -- Prometheus metrics for DMS API call duration, errors, and heartbeat
- `pkg/utils/` -- Secret and utility helpers

## Testing

Tests use `controller-runtime`'s fake client and GoMock for the DMS API client. The `setupDefaultMocks()` helper in the controller test creates a standard set of test objects (DMSI, ClusterDeployment, Secrets).

**PKO template tests** (`pkg/pko/template_test.go`) have two layers:
1. **Snapshot tests** -- golden files in `deploy_pko/.test-fixtures/`, validated by `kubectl-package validate`
2. **Structural tests** -- Go assertions on rendered template output (kind, annotations, conditional fields)

To regenerate PKO fixtures after template changes:
```bash
rm -rf deploy_pko/.test-fixtures/
kubectl-package validate deploy_pko/
```

## Code Generation

`make generate` runs four generators in sequence: `op-generate` (controller-gen CRDs + deepcopy), `go-generate` (mocks), `openapi-generate`, `manifests`. CI enforces that generated files are committed via `make generate-check`.

Generated files (do not edit manually):
- `api/v1alpha1/zz_generated.deepcopy.go`
- `api/v1alpha1/zz_generated.openapi.go`
- `pkg/dmsclient/mock/mock_dmsclient.go`
- `deploy/crds/*.yaml`

## Deployment

Two deployment models exist side by side:
- **`deploy/`** -- Traditional OLM-style Kubernetes manifests
- **`deploy_pko/`** -- Package Operator format with Go templates (`.gotmpl`), used for current deployments

PKO config values (`deploy_pko/manifest.yaml`): `image`, `silentAlertLegalEntityIds`, `deadmanssnitchOsdTags`, `fedramp`.

## Boilerplate

This repo consumes `openshift/boilerplate` v8.3.4 via `boilerplate/update`. Run `make boilerplate-update` to pull the latest. Boilerplate provides standard Make targets, golangci-lint config, Tekton pipelines, and OWNERS_ALIASES. Do not edit files under `boilerplate/` directly.
