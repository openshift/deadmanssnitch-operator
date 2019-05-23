// Copyright 2019 RedHat
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	// MetricsEndpoint is the port to export metrics on
	MetricsEndpoint = ":8080"
)

var (
	metricDeadMansSnitchHeartbeat = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "metricDeadMansSnitchHeartbeat",
		Help: "Metric for heartbeating the Dead Man's Snitch api",
	}, []string{"name"})

	metricsList = []prometheus.Collector{
		metricDeadMansSnitchHeartbeat,
	}
)

// StartMetrics register metrics and exposes them
func StartMetrics() {

	// Register metrics and start serving them on /metrics endpoint
	RegisterMetrics()
	http.Handle("/metrics", prometheus.Handler())
	go http.ListenAndServe(MetricsEndpoint, nil)
}

// RegisterMetrics for the operator
func RegisterMetrics() error {
	for _, metric := range metricsList {
		err := prometheus.Register(metric)
		if err != nil {
			return err
		}
	}
	return nil
}

// UpdateMetrics updates all the metrics ever 5 minutes
func UpdateMetrics() {

	d := time.Tick(5 * time.Minute)
	for range d {
		UpdateMetricDeadMansSnitchHeartbeatGauge()
	}
}

// UpdateMetricDeadMansSnitchHeartbeatGauge curls the DMS API, updates the gauge to 1 when successful.
func UpdateMetricDeadMansSnitchHeartbeatGauge() {

	req, err := http.NewRequest("GET", "https://api.deadmanssnitch.com/v1/snitches", nil)
	if err != nil {
		metricDeadMansSnitchHeartbeat.With(prometheus.Labels{"name": "deadmanssnitch-operator"}).Set(float64(0))
	}
	req.SetBasicAuth(DeadMansSnitchAPISecretName, "")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		metricDeadMansSnitchHeartbeat.With(prometheus.Labels{"name": "deadmanssnitch-operator"}).Set(float64(0))
	}
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		metricDeadMansSnitchHeartbeat.With(prometheus.Labels{"name": "deadmanssnitch-operator"}).Set(float64(1))
	} else {
		metricDeadMansSnitchHeartbeat.With(prometheus.Labels{"name": "deadmanssnitch-operator"}).Set(float64(0))
	}

	defer resp.Body.Close()
}