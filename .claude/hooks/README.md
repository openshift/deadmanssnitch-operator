# Claude Code Hooks

Security and validation hooks for this operator development.

## Overview

This repository uses **prek** (git hook manager) for quality checks and validation. Claude Code hooks integrate with prek to provide immediate feedback during development.

## Architecture

```text
┌─────────────────────────────────────┐
│   Developer / Claude Code Agent     │
└──────────────┬──────────────────────┘
               │
               ▼
┌─────────────────────────────────────┐
│   Stop Hook (conditional)           │
│   - Default: runs only with changes │
│   - Strict: runs every turn         │
│   - Blocks if issues found          │
│   - Claude fixes automatically      │
└──────────────┬──────────────────────┘
               │
               ▼
┌─────────────────────────────────────┐
│   Prek Hooks (CI config)            │
│   - golangci-lint (static analysis) │
│   - RBAC wildcard check             │
│   - go build validation             │
│   - go mod tidy check               │
│   - file hygiene (trailing space)   │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│   Prek Hooks (full config)          │
│   + rh-pre-commit (InfoSec)         │
│   + gitleaks (secret scanning)      │
└──────────────┬──────────────────────┘
               │
               ▼
┌─────────────────────────────────────┐
│   Git Commit                        │
└──────────────┬──────────────────────┘
               │
               ▼
┌─────────────────────────────────────┐
│   CI/CD (Tekton Pipelines)          │
└─────────────────────────────────────┘
```

## Available Hooks

### [stop-prek-validation.sh](./stop-prek-validation.sh)
**Purpose**: Run prek validation when Claude makes changes (or always, if configured)

**Triggers**: On Claude Code session stop (Stop hook)

**Default mode** (recommended):
- Only runs if there are uncommitted changes
- Skips validation for read-only queries (fast iteration)
- Validates when Claude modifies code (before commit)

**Strict mode** (opt-in):
- Set environment variable: `export CLAUDE_LINT_ON_STOP=true`
- Always runs validation on every stop
- Slower but catches issues immediately

**Common behavior**:
- Runs `prek run --config hack/prek.ci.toml` on changed files
- Uses CI-compatible config (skips network-dependent hooks like rh-pre-commit, gitleaks)
- Blocks Claude from stopping if issues found
- Feeds errors back to Claude for automatic fixes

**Enable strict mode**:
```bash
# In your shell profile (~/.zshrc, ~/.bashrc)
export CLAUDE_LINT_ON_STOP=true
```

---

### [pre-edit.sh](./pre-edit.sh)
**Purpose**: Prevent editing generated files and warn about high-risk changes

**Status**: Available for standalone use (not wired as a Claude Code hook)

**Checks**:
- Generated files (`zz_generated.*.go`)
- Generated mocks (`pkg/dmsclient/mock/mock_dmsclient.go`)
- Vendored code (`vendor/`)
- Boilerplate files (managed upstream)
- High-risk security files (RBAC, auth, NetworkPolicy)
- CI/CD pipelines (`.tekton/*.yaml`)
- Dockerfiles

**Manual Usage**:
```bash
.claude/hooks/pre-edit.sh path/to/file.go
```

---

## Prek Configuration

This repository maintains **two prek configurations**:

### 1. **prek.toml** (Full validation)
Used for local development with internal network access.

**Hooks**:
- File hygiene (trailing whitespace, EOF, syntax checks)
- **rh-pre-commit**: Red Hat InfoSec security checks (requires `gitlab.cee.redhat.com` access)
- **gitleaks**: Secret detection (configured via `.gitleaks.toml`)
- **golangci-lint**: Static analysis
- **go-build**: Compile check
- **go-mod-tidy**: Dependency drift detection
- **rbac-wildcard-check**: RBAC validation

**Usage**:
```bash
prek run  # Uses prek.toml by default
```

### 2. **hack/prek.ci.toml** (CI-compatible)
Used by Claude Code stop hook and CI environments without internal network access.

**Excludes**:
- `rh-pre-commit` (requires Red Hat internal network)
- `gitleaks` (may not be available in all CI environments)

**Usage**:
```bash
hack/ci.sh
# or
prek run --config hack/prek.ci.toml --all-files
```

## Setup

### Prerequisites
```bash
# Install prek (choose one)
uv tool install prek      # recommended
pipx install prek         # alternative
pip install --user prek   # fallback
```

### Install Git Hooks
```bash
prek install
```

## Security Guardrails

### Secret Prevention
**Implementation**: gitleaks via prek
**Configuration**: `.gitleaks.toml`
**Action**: BLOCK commit

### InfoSec Scanning
**Implementation**: rh-pre-commit via prek
**Action**: BLOCK commit on violations

### RBAC Validation
**Implementation**: rbac-wildcard-check via prek
**Detects**: Wildcard resources/verbs in `deploy/*.yaml`
**Action**: BLOCK commit

### File Edit Protection
**Implementation**: pre-edit.sh (standalone)
**Action**: BLOCK edit on generated/vendored files, WARN on high-risk files

## Hook Performance

**Targets:**
- Stop hook: <30s for full validation
- Pre-commit hook: <30s on typical changeset

**Never bypass hooks:**
```bash
# FORBIDDEN
git commit --no-verify
SKIP=hook-id git commit
```

Security hooks (gitleaks, rh-pre-commit) must NEVER be bypassed.

## Version Management

Prek version pinned in `.prek-version`:
```bash
cat .prek-version  # v0.4.1
```

Hook dependencies pinned in `prek.toml`:
- `rh-pre-commit-2.3.0`
- `v8.18.0` (gitleaks)
- `v2.0.2` (golangci-lint)

## References

- [Prek Documentation](https://prek.j178.dev/)
- [Gitleaks](https://github.com/gitleaks/gitleaks)
- [RH InfoSec Tools](https://gitlab.cee.redhat.com/infosec-public/developer-workbench/tools)
- [golangci-lint](https://golangci-lint.run/)
- [AGENTS.md](../../AGENTS.md) - Development guidelines
