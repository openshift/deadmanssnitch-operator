# Testing Guide

Testing guidelines for the deadmanssnitch-operator.

## Framework

- **testify/assert**: Assertions and test helpers (`github.com/stretchr/testify`)
- **GoMock**: Interface mocking (`go.uber.org/mock/gomock`)
- **controller-runtime fake client**: Kubernetes API simulation for controller tests
- **envtest**: Kubernetes API server for integration-style tests

## Quick Commands

```bash
# Run all tests
make go-test

# Run specific package
go test -v ./controllers/deadmanssnitchintegration/

# Run a single test by name
go test -v -run TestReconcileClusterDeployment ./controllers/deadmanssnitchintegration/

# Run all packages
go test ./...

# Container-based (CI parity)
boilerplate/_lib/container-make go-test
```

## Writing Tests

### Test Structure

Tests use standard Go `testing.T` with testify assertions:

```go
package deadmanssnitchintegration_test

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestMyFeature(t *testing.T) {
    result, err := MyFunction()
    require.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

### Mocking Interfaces

The DMS API client is mocked with GoMock. The mock is pre-generated at
`pkg/dmsclient/mock/mock_dmsclient.go`.

```go
import (
    "testing"
    "go.uber.org/mock/gomock"
    "github.com/openshift/deadmanssnitch-operator/pkg/dmsclient/mock"
)

func TestReconcileCreate(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockClient := mock.NewMockClient(ctrl)
    mockClient.EXPECT().
        CreateSnitch(gomock.Any()).
        Return(&dmsclient.Snitch{CheckInURL: "https://nosnch.in/abc"}, nil)

    // inject mock into reconciler and test...
}
```

The `setupDefaultMocks()` helper in the controller test creates a standard set of
test objects (DMSI CR, ClusterDeployment, Secrets) for use across tests.

**Regenerate all mocks:**
```bash
boilerplate/_lib/container-make generate
```

## Test Organization

### Unit Tests
- Test individual functions and methods
- Mock external dependencies (DMS API client)
- Fast execution (<1s per package)
- Located alongside source code

### Controller Tests
- Test reconciliation logic end-to-end
- Use controller-runtime's fake client
- Test custom resource lifecycle (create, update, delete, finalizers)
- Located in `controllers/deadmanssnitchintegration/`

### PKO Template Tests
- Located in `pkg/pko/template_test.go`
- **Snapshot tests**: golden files in `deploy_pko/.test-fixtures/`, validated by `kubectl-package validate`
- **Structural tests**: Go assertions on rendered template output (kind, annotations, conditional fields)

## Agent-Driven Validation

When AI agents modify code:

**Minimal validation:**
```bash
# After changing controllers/deadmanssnitchintegration/
go test ./controllers/deadmanssnitchintegration/
```

**Full validation before commit:**
```bash
make go-test
```

**If tests fail:**
1. Read test output carefully
2. Fix the underlying issue (don't skip tests)
3. Rerun to confirm fix
4. Regenerate mocks if interface changed: `boilerplate/_lib/container-make generate`

## Common Patterns

### Testing Controllers

```go
func TestReconcileClusterDeployment(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    // Create mock client and set expectations
    mockClient := mock.NewMockClient(ctrl)
    mockClient.EXPECT().FindSnitch(gomock.Any()).Return(nil, nil)
    mockClient.EXPECT().CreateSnitch(gomock.Any()).Return(&dmsclient.Snitch{}, nil)

    // Create fake k8s client with test objects
    scheme := setupScheme()
    fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(testObjects...).Build()

    // Run reconciler
    r := &DeadmansSnitchIntegrationReconciler{
        Client:    fakeClient,
        DmsClient: mockClient,
    }
    result, err := r.Reconcile(context.TODO(), req)
    assert.NoError(t, err)
    assert.False(t, result.Requeue)
}
```

### Testing Error Conditions

```go
func TestReconcileCreateError(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockClient := mock.NewMockClient(ctrl)
    mockClient.EXPECT().CreateSnitch(gomock.Any()).Return(nil, fmt.Errorf("API error"))

    // ...verify error is returned and handled correctly
}
```

### Using testify Matchers

```go
// Equality
assert.Equal(t, expected, actual)

// Nil checks
require.NoError(t, err)
assert.Nil(t, obj)

// Collections
assert.Contains(t, slice, "item")
assert.Len(t, slice, 3)
assert.Empty(t, slice)

// Booleans
assert.True(t, condition)
assert.False(t, condition)
```

## Coverage

Generate coverage report:
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

**Note**: Aim for meaningful coverage, not arbitrary percentages.
- Test critical paths and error handling
- Don't test generated code or trivial getters/setters

## PKO Template Tests

```bash
# Validate against existing fixtures
kubectl-package validate deploy_pko/

# Regenerate fixtures after template changes
rm -rf deploy_pko/.test-fixtures/
kubectl-package validate deploy_pko/

# Run Go-level template assertions
go test ./pkg/pko/...
```

## Debugging Tests

```bash
# Verbose output
go test -v ./controllers/deadmanssnitchintegration/

# Run single test
go test -v -run TestReconcileCreate ./controllers/deadmanssnitchintegration/

# Race detector
go test -race ./...
```

## CI Expectations

Tests run in Tekton pipeline with:
- Fresh environment
- No cached dependencies
- Strict timeout limits

**Local CI parity:**
```bash
boilerplate/_lib/container-make go-test
```

## Common Issues

**Mock not found or outdated:**
```bash
# Regenerate mocks
boilerplate/_lib/container-make generate
```

**envtest not installed:**
```bash
make setup-envtest
```

**Test passes locally, fails in CI:**
```bash
# Run in container environment
boilerplate/_lib/container-make go-test

# Check for:
# - Time-dependent tests
# - Environment-specific assumptions
# - File path dependencies
```

## Further Reading

- [testify Documentation](https://github.com/stretchr/testify)
- [GoMock Guide](https://pkg.go.dev/go.uber.org/mock/gomock)
- [controller-runtime Testing](https://book.kubebuilder.io/reference/testing.html)
