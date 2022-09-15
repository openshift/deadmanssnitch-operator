# Development

- [Development](#development)
  - [Development Environment Setup](#development-environment-setup)
    - [golang](#golang)
    - [operator-sdk](#operator-sdk)
  - [Makefile](#makefile)
  - [Build using boilerplate container](#build-using-boilerplate-container)

This document should entail all you need to develop this operator locally.

## Development Environment Setup

### golang

A recent Go distribution (>=1.17) with enabled Go modules.

```shell
$ go version
go version go1.17.11 linux/amd64
```

### operator-sdk

The Operator is being developed based on the [Operators SDK](https://github.com/operator-framework/operator-sdk).
Ensure this is installed and available in your `$PATH`.

[v1.21.0](https://github.com/operator-framework/operator-sdk/releases/tag/v1.21.0) is being used for `deadmanssnitch-operator` development.

```shell
$ operator-sdk version
operator-sdk version: "v1.21.0", commit: "89d21a133750aee994476736fa9523656c793588", kubernetes version: "1.23", go version: "go1.17.10", GOOS: "linux", GOARCH: "amd64"
```

## Makefile

There are some `make` commands that can be run using the top level `Makefile` itself:

```shell
$ make help
Usage: make <OPTIONS> ... <TARGETS>

Available targets are:

go-build                         Build binary
go-check                         Golang linting and other static analysis
boilerplate-update               Make boilerplate update itself
help                             Show this help screen.
run                              Run deadmanssnitch-operator locally
```

---

## Build using boilerplate container

To run lint, test and build in `app-sre/boilerplate` container, call `boilerplate/_lib/container-make`. This will call `make` inside the `app-sre/boilerplate` container.

```shell
boilerplate/_lib/container-make TARGET
```

Example:

```shell
# To run unit tests
boilerplate/_lib/container-make test

# To run lint tests
boilerplate/_lib/container-make lint

# To run coverage
boilerplate/_lib/container-make coverage
```
