# deadmanssnitch-operator

Operator to manage deadmanssnitch configs for Openshift Dedicated

## Metrics

metricDeadMansSnitchHeartbeat: Every 5 minutes, makes a request to the Dead Man's Switch API using the API key and updates the gauge to 1 when the response code is between 200-299.

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
