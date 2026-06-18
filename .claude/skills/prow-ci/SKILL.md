---
name: prow-ci
description: Fetch and analyze OpenShift Prow CI job failures with automated artifact download and failure pattern detection
trigger: prow, prow-ci, /prow-ci, ci results, check ci, analyze ci failure
---

# Prow CI Analysis for deadmanssnitch-operator

This skill fetches Prow CI job artifacts from Google Cloud Storage and provides automated failure analysis.

## Prerequisites

Before using this skill, verify gcloud CLI is installed:
```bash
which gcloud
```

If not installed, see: https://cloud.google.com/sdk/docs/install

**Note**: The `test-platform-results` GCS bucket is publicly accessible — no authentication required.

## Quick Start

```bash
# Check PR status and get Prow job URLs
gh pr checks <PR_NUMBER>

# Analyze a failed job
/prow-ci <prow-job-url>

# Or ask naturally:
"Analyze the lint failure in PR <NUMBER>"
"Check why the validate job failed"
```

## Implementation

When invoked, this skill:

1. **Fetches artifacts** using `fetch_prow_artifacts.py`:
   - Downloads **prowjob.json** (job metadata)
   - Downloads **build-log.txt** (complete build output with all errors)
   - Saves to `.work/prow-artifacts/<build-id>/`

2. **Analyzes failures** using `analyze_failure.py`:
   - Parses build-log.txt for error patterns
   - Detects common failure patterns (lint, build, timeout, OOM)
   - Extracts error messages and stack traces

3. **Generates report**:
   - Markdown format with failure summary
   - Pattern detection results
   - Actionable failure details

## Usage Instructions

### Step 1: Get Prow Job URL

```bash
# View PR checks to find failed jobs
gh pr checks <PR_NUMBER>

# Or get detailed status
gh pr view <PR_NUMBER> --json statusCheckRollup --jq '.statusCheckRollup[] | select(.state == "FAILURE")'
```

Example Prow job URL:
```text
https://prow.ci.openshift.org/view/gs/test-platform-results/pr-logs/pull/openshift_deadmanssnitch_operator/<PR_NUMBER>/pull-ci-openshift-deadmanssnitch-operator-master-lint/<BUILD_ID>
```

### Step 2: Fetch and Analyze

```bash
# From repository root
python3 .claude/skills/prow-ci/fetch_prow_artifacts.py "<prow-job-url>" -o .work/prow-artifacts
```

### Step 3: Analyze Failures

```bash
python3 .claude/skills/prow-ci/analyze_failure.py .work/prow-artifacts/<build-id> -f markdown
```

## Common Job Names

**Prow CI Jobs** (configured in openshift/release):
- `pull-ci-openshift-deadmanssnitch-operator-master-coverage` - Code coverage
- `pull-ci-openshift-deadmanssnitch-operator-master-lint` - Linting checks
- `pull-ci-openshift-deadmanssnitch-operator-master-test` - Unit tests
- `pull-ci-openshift-deadmanssnitch-operator-master-validate` - Validation checks

**Tekton Pipelines** (configured in `.tekton/`):
- `deadmanssnitch-operator-pull-request` - Main PR pipeline
- `deadmanssnitch-operator-e2e-pull-request` - E2E testing pipeline
- `deadmanssnitch-operator-pko-pull-request` - PKO pipeline

## Reproducing Failures Locally

```bash
# For unit tests (matches: pull-ci-...-test)
make go-test

# For linting (matches: pull-ci-...-lint)
make go-check

# For validation (matches: pull-ci-...-validate)
make validate

# For coverage (matches: pull-ci-...-coverage)
make coverage

# For container builds (Tekton pipelines)
make docker-build
```

## Prow Resources

**Main Dashboard**: https://prow.ci.openshift.org/
**CI Search**: https://github.com/openshift/ci-search
**Job History**: https://prow.ci.openshift.org/?repo=openshift%2Fdeadmanssnitch-operator

## Troubleshooting

### Can't find job results?
- Check both Prow AND Tekton — this repo uses both systems
- Prow jobs: `pull-ci-openshift-deadmanssnitch-operator-master-*`
- Tekton jobs: Show as pipeline names in PR checks
- Verify repo name format in Prow: `openshift_deadmanssnitch_operator` (underscore, not dash)

### Tekton pipeline failures?
- Check the pipeline link in PR checks (links to Konflux/AppStudio UI)
- Common issues:
  - Image build failures → Check Dockerfile syntax
  - Pipeline timeout → Check for slow steps or network issues
- Local validation:
  ```bash
  kubectl apply --dry-run=client -f .tekton/
  podman build -f build/Dockerfile -t test:local .
  ```

## References

- [Prow Dashboard](https://prow.ci.openshift.org/)
- [CI Search Tool](https://github.com/openshift/ci-search)
- [OpenShift CI Documentation](https://docs.ci.openshift.org/)
