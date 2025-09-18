# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This is a Kubernetes operator built with the Operator SDK that manages Dead Man's Snitch integrations for OpenShift Dedicated clusters. The operator runs on Hive and watches for ClusterDeployments to automatically create monitoring snitches.

## Core Architecture

### Key Components

- **Custom Resource**: `DeadmansSnitchIntegration` (DMSI) defines which clusters get DMS integration
- **Controller**: `DeadmansSnitchIntegrationReconciler` in `controllers/deadmanssnitchintegration/`
- **API Types**: Located in `api/v1alpha1/` with the main CRD definition
- **DMS Client**: `pkg/dmsclient/` handles Dead Man's Snitch API interactions
- **Metrics**: `pkg/localmetrics/` provides custom Prometheus metrics
- **Utils**: `pkg/utils/` contains helper functions

### Controller Logic

The operator watches ClusterDeployments and:
1. Adds finalizers to ensure cleanup
2. Creates snitches via Dead Man's Snitch API
3. Creates secrets containing snitch URLs
4. Creates SyncSets to deploy secrets to target clusters
5. Manages the lifecycle when clusters are deleted

## Development Commands

### Basic Commands
- `make run` - Run the operator locally (requires OPERATOR_NAME env var)
- `make go-build` - Build the binary
- `make go-check` - Run linting and static analysis
- `make help` - Show all available make targets

### Testing Commands
- `boilerplate/_lib/container-make test` - Run unit tests in container
- `boilerplate/_lib/container-make lint` - Run lint checks in container
- `boilerplate/_lib/container-make coverage` - Run coverage analysis in container

### Development Dependencies

- Go 1.23+ with modules enabled
- Operator SDK v1.21.0
- Hive CRDs must be installed in target cluster
- Dead Man's Snitch API key for testing

## Boilerplate Integration

This project uses OpenShift's golang-osd-operator boilerplate which provides:
- Standardized Makefile targets
- Container build support via podman/docker
- CI/CD integration with app-sre
- FIPS compliance support
- CSV generation for OLM

The boilerplate is included via `boilerplate/generated-includes.mk` and should not be modified directly.

## Configuration

### Required Secrets
- `deadmanssnitch-api-key` secret in `deadmanssnitch-operator` namespace containing:
  - `deadmanssnitch-api-key`: Base64 encoded API key
  - `tags`: Base64 encoded tags for snitches

### Environment Variables
- `OPERATOR_NAME`: Required when running locally (set to "deadmanssnitch-operator")

## Key Files

- `main.go`: Entry point with manager setup and metrics configuration
- `api/v1alpha1/deadmanssnitchintegration_types.go`: CRD definition
- `controllers/deadmanssnitchintegration/deadmanssnitchintegration_controller.go`: Main reconciliation logic
- `pkg/dmsclient/`: Dead Man's Snitch API client implementation
- `config/`: Kubernetes manifests and templates
- `deploy/`: Deployment manifests for the operator

## Local Development Setup

1. Install Hive CRDs: `oc apply -f hive/config/crds`
2. Create operator namespace: `oc new-project deadmanssnitch-operator`
3. Apply RBAC: `oc apply -f deploy/role.yaml deploy/service_account.yaml deploy/role_binding.yaml`
4. Create API key secret (see README for format)
5. Run locally: `OPERATOR_NAME=deadmanssnitch-operator make run`

## Testing Strategy

- Unit tests use golang/mock for API client mocking
- Tests focus on controller reconciliation logic
- Integration tests require actual ClusterDeployment resources
- Use fake ClusterDeployments for local testing (see development.md)