# Parallel hardened build (ROSAENG-75339)

The production `build/Dockerfile` is unchanged. The new
`build/Dockerfile.hummingbird` keeps the existing boilerplate Go compiler stage
and copies only the application binary into digest-pinned
`hi/core-runtime:2.43-openssl-fips`. The base supplies UID 65532, glibc, and
trusted CA certificates. No Hummingbird builder stage or package installation
is needed: the application invokes no external runtime commands and performs
no runtime package installation.

The application is dynamically linked by the existing FIPS build rules
(`CGO_ENABLED=1`, `GOFIPS140=v1.0.0`, external linking). The hardened variant
retains those rules; its runtime must be checked by executing the binary in
the built image. No UBI-installed libraries or filesystem are copied.

The hardened image uses `/` instead of `/root` and invokes
`/usr/local/bin/deadmanssnitch-operator` directly. Existing deployment command
overrides continue to resolve the binary through PATH.

The additional push PipelineRun uses its own application/component labels and
`-hummingbird` output tag. It does not change production pipelines or release
configuration. Renovate manages both Dockerfiles and requires manual review
of Hummingbird digest updates.

## Local verification

JSON and YAML parse successfully. The parallel PipelineRun was checked to
differ only in its name, application/component labels, output tag, and
Dockerfile path. `git diff --check` passes, and the production Dockerfile is
identical to the original checkout.

Verified locally on 2026-11-09 using Linux/amd64 images under QEMU on an
ARM64 Podman VM:

- Both Dockerfiles built successfully with `--pull=never` after pulling their
  bases. Local tags: `deadmanssnitch-operator-hummingbird-test` and
  `deadmanssnitch-operator-production-test`.
- Both images ran `/usr/local/bin/deadmanssnitch-operator --help` successfully
  (exit 0). This executes the compiled FIPS startup assertion and verifies
  that the dynamic loader and libraries resolve on each base.
- `readelf` confirms an ELF interpreter and dependencies on `libresolv.so.2`
  and `libc.so.6`; only the compiled application binary is copied into the
  hardened image. Its default runtime user is 65532.
- Build command: `podman build --jobs=1 --cpu-period=100000
  --cpu-quota=200000 --platform linux/amd64 --pull=never -f
  build/Dockerfile.hummingbird -t deadmanssnitch-operator-hummingbird-test .`;
  production used `build/Dockerfile` and its corresponding tag.

Cluster reconciliation, CVE scanning, Enterprise Contract, and release gates
remain CI/cluster validation work; this parallel build does not cut over
production.
