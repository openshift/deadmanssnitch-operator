# deadmanssnitch-operator

Operator to manage deadmanssnitch configs for Openshift Dedicated

## Metrics

metricDeadMansSnitchHeartbeat: Every 5 minutes, makes a request to the Dead Man's Switch API using the API key and updates the gauge to 1 when the response code is between 200-299.

## Alerts

Creates an alert when metricDeadMansSnitchHeartbeat is 0 for 15 minutes.