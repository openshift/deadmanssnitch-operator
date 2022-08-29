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
- Pays attention to [ClusterDeployments](https://github.com/openshift/hive/blob/master/config/crds/hive.openshift.io_clusterdeployments.yaml) that are:
  - Installed (`spec.installed=true`)
  - Managed (label `api.openshift.com/managed="true"`)
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
  - you can do that using `oc create -f https://github.com/openshift/deadmanssnitch-operator/raw/master/deploy/operator.yaml --dry-run=client -oyaml | oc set image --local -f - --dry-run=client -oyaml *=REPLACE_IMAGE`
- Deploy using `oc apply -f deploy/`

## Development

<details>
  <summary> how to develop this locally</summary>
    <p>

### Set up local OpenShift cluster

Methods include:
- [MiniShift](https://github.com/minishift/minishift)
- [Code Ready Containers](https://developers.redhat.com/products/codeready-containers/overview)
- [Integration OpenShift Cluster Manager](https://qaprodauth.cloud.redhat.com/openshift/?env=integration)

### Deploy dependencies

[Hive](https://github.com/openshift/hive/) CRDs need to be installed on the cluster.

Clone [hive repo](https://github.com/openshift/hive/) and run

```terminal
$ git clone https://github.com/openshift/hive.git
$ oc apply -f hive/config/crds
```

Install the `DeadMansSnitchIntegration` CRD, create the operator namespace and other operator dependencies:

```terminal
$ oc apply -f deploy/crds/deadmanssnitch.managed.openshift.io_deadmanssnitchintegrations_crd.yaml
$ oc new-project deadmanssnitch-operator
$ oc apply -f deploy/role.yaml
$ oc apply -f deploy/service_account.yaml
$ oc apply -f deploy/role_binding.yaml
```

Create a secret which will contain the DeadMansSnitch API Key and Hive Cluster Tag.

You will require an API Key signed up to a DeadMansSnitch plan that allows for enhanced snitch intervals (the "Private Eye" plan). You can alternatively test the `deadmanssnitch-oeprator` by signing up to the free tier DeadMansSnitch plan (limited to 1 snitch), but doing so will require you to customize the snitch interval from `15_minute` to `hourly`. This can be performed in [deadmanssnitchintegration_controller.go](pkg/controller/deadmanssnitchintegration/deadmanssnitchintegration_controller.go)

Adjust the example below and apply the file with `oc apply -f <file>`. Note that the values for `hive-cluster-tag` and `deadmanssnitch-api-key` need to be base64 encoded. This can be performed using `echo -n <text> | base64`.

```yaml
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: deadmanssnitch-api-key
  namespace: deadmanssnitch-operator
data:
  hive-cluster-tag: <value>
  deadmanssnitch-api-key: <value>
```

### Define a DeadMansSnitchIntegration

Create a `DeadMansSnitchIntegration` CR which will be used to identify clusters to apply DMS to.

The example below will target `clusterdeployment`s that have a `api.openshift.com/test` label set to `"true"`. Apply it using `oc apply -f <file>`.

```yaml
apiVersion: deadmanssnitch.managed.openshift.io/v1alpha1
kind: DeadmansSnitchIntegration
metadata:
  finalizers:
  - dms.managed.openshift.io/deadmanssnitch-osd
  name: test-dmsi
  namespace: deadmanssnitch-operator
spec:
  clusterDeploymentSelector:
    matchExpressions:
    - key: api.openshift.com/test
      operator: In
      values:
      - "true"
  dmsAPIKeySecretRef:
    name: deadmanssnitch-api-key
    namespace: deadmanssnitch-operator
  snitchNamePostFix: "test"
  tags:
  - test
  targetSecretRef:
    name: dms-secret-test
    namespace: openshift-monitoring
```

### Run the operator

```terminal
$ export OPERATOR_NAME=deadmanssnitch-operator
$ go run main.go
```

### Create Clusterdeployment

You can create a dummy ClusterDeployment by copying a real one from an active hive

```terminal
real-hive$ oc get cd -n <namespace> <cdname> -o yaml > /tmp/fake-clusterdeployment.yaml

...

$ oc create namespace fake-cluster-namespace
$ oc apply -f /tmp/fake-clusterdeployment.yaml
```

`deadmanssnitch-operator` doesn't start reconciling clusters until the `clusterdeployment`'s `spec.installed` is set to `true`. If present, set `spec.installed` to true.

```terminal
$ oc edit clusterdeployment fake-cluster -n fake-cluster-namespace
```

Ensure that the ClusterDeployment is labelled with the label from your `DMSI`'s `clusterDeploymentSelector` clause.

Using the example from earlier:
```terminal
$ oc label clusterdeployment -n <namespace> <cdname> api.openshift.com/test=true
```

### Delete ClusterDeployment

To trigger `deadmanssnitch-operator` to remove the service in DeadMansSnitch, you can either delete the `clusterdeployment` or remove the `clusterDeploymentSelector` label:

```terminal
$ oc delete clusterdeployment fake-cluster -n fake-cluster-namespace
```

If deleting the `clusterdeployment`, you may need to remove dangling finalizers from the `clusterdeployment` object.

```terminal
$ oc edit clusterdeployment fake-cluster -n fake-cluster-namespace
```

</p>
</details>
