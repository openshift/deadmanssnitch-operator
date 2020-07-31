# deadmanssnitch-operator

Operator to manage deadmanssnitch configs for Openshift Dedicated

- [deadmanssnitch-operator](#deadmanssnitch-operator)
  - [Overview](#overview)
  - [Metrics](#metrics)
  - [Alerts](#alerts)
  - [Usage](#usage)

## Overview

The operator runs on hive. It has a single controller. It:
- Requires a master Secret to talk to the Dead Man's Snitch API.
  This secret is expected to be named `deadmanssnitch-api-key` and live in the `deadmanssnitch-operator` namespace.
- Pays attention to ClusterDeployments that are:
  - Installed (`spec.installed=true`)
  - Managed (label `api.openshift.com/managed="true"`)
  - Not silenced (label `api.openshift.com/noalerts!="true"`)
- For each such ClusterDeployment:
  - Adds a finalizer to the ClusterDeployment to ensure we get a chance to clean up when it is deleted.
  - Creates a Snitch
  - Creates a Secret in the ClusterDeployment's namespace named `{clusterdeploymentname}-dms-secret`.
    The Secret contains the Snitch URL.
  - Creates a SyncSet in the ClusterDeployment's namespace named `{clusterdeploymentname}-dms}`.
    The SyncSet creates a SecretMapping that makes the above Secret appear inside the cluster as `dms-secret` in the `openshift-monitoring` namespace.

## Metrics

metricDeadMansSnitchHeartbeat: Every 5 minutes, makes a request to the Dead Man's Snitch API using the API key and updates the gauge to 1 when the response code is between 200-299.

## Alerts

- DeadMansSnitchAPIUnavailable - Unable to communicate with Dead Man's Snitch API for 15 minutes.

## Usage

- Create an account on https://deadmanssnitch.com/ 
- Choose a plan that allows enhanced snitch intervals(Private eye or above)
- Create an API key
- Create the following secret which is required for deadmanssnitch-operator to create snitches

```yaml
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: deadmanssnitch-api-key
  namespace: deadmanssnitch-operator
data:
  hive-cluster-tag: <Tag for snitches>
  deadmanssnitch-api-key: <deadmanssnitch API key here>
```

- Build a docker image and replace `REPLACE_IMAGE` [operator.yaml](deploy/operator.yaml) field with that image
- Deploy using `oc apply -f deploy/`
