SHELL := /usr/bin/env bash
OPERATOR_DOCKERFILE = build/Dockerfile

# Include shared Makefiles
include project.mk
include standard.mk

default: gobuild

# Extend Makefile after here

BINDIR = bin
SRC_DIRS = pkg
GOFILES = $(shell find $(SRC_DIRS) -name '*.go' | grep -v bindata)

# Look up distro name (e.g. Fedora)
DISTRO ?= $(shell if which lsb_release &> /dev/null; then lsb_release -si; else echo "Unknown"; fi)

# Image URL to use all building/pushing image targets
IMG ?= deadmanssnitch-operator:latest

BUILD_CMD ?= docker build
DOCKER_CMD ?= docker

# Build the docker image
.PHONY: docker-build
docker-build:
	$(BUILD_CMD) -t ${IMG} -f ./build/Dockerfile .

# Push the docker image
.PHONY: docker-push
docker-push:
	$(DOCKER_CMD) push ${IMG}
