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

package localmetrics

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	Collector *MetricsCollector
)

const (
	operatorName      = "deadmanssnitch-operator"
	snitchMethodLabel = "method"
)

type MetricsCollector struct {
	ReconcileDuration  prometheus.Histogram
	apiCallDuration    *prometheus.HistogramVec
	snitchCallErrors   prometheus.Counter
	snitchCallDuration *prometheus.HistogramVec
	collectors         []prometheus.Collector
}

func (m MetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	m.ReconcileDuration.Describe(ch)
	m.apiCallDuration.Describe(ch)
	m.snitchCallDuration.Describe(ch)
	m.snitchCallErrors.Describe(ch)
	for _, c := range m.collectors {
		c.Describe(ch)
	}
}

func (m MetricsCollector) Collect(ch chan<- prometheus.Metric) {
	m.ReconcileDuration.Collect(ch)
	m.apiCallDuration.Collect(ch)
	m.snitchCallErrors.Collect(ch)
	m.snitchCallDuration.Collect(ch)
	for _, c := range m.collectors {
		c.Collect(ch)
	}
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		ReconcileDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:        "dms_operator_reconcile_duration_seconds",
			Help:        "The duration it takes to reconcile a ClusterDeployment",
			ConstLabels: map[string]string{"name": operatorName},
		}),
		// apiCallDuration times API requests. Histogram also gives us a _count metric for free but also includes errors.
		apiCallDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:        "dms_operator_api_request_duration_seconds",
			Help:        "Distribution of the number of seconds an API request takes",
			ConstLabels: prometheus.Labels{"name": operatorName},
			// We really don't care about quantiles, but omitting Buckets results in defaults.
			// This minimizes the number of unused data points we store.
			Buckets: []float64{1},
		}, []string{"controller", "method", "resource", "status"}),
		snitchCallErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name:        "dms_operator_snitch_api_call_error",
			Help:        "Counter of the number of errors in calls to the DMS API",
			ConstLabels: prometheus.Labels{"name": operatorName},
		}),
		snitchCallDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:        "dms_operator_snitch_api_call_duration_seconds",
			Help:        "Distribution of the timings of API calls to DMS in seconds",
			ConstLabels: prometheus.Labels{"name": operatorName},
		}, []string{snitchMethodLabel}),
	}
}

// AddAPICall observes metrics for a call to an external API
// - param controller: The name of the controller making the API call
// - param req: The HTTP Request structure
// - param resp: The HTTP Response structure
// - param duration: The number of seconds the call took.
func (m *MetricsCollector) AddAPICall(controller string, req *http.Request, resp *http.Response, duration float64) {
	m.apiCallDuration.With(prometheus.Labels{
		"controller": controller,
		"method":     req.Method,
		"resource":   resourceFrom(req.URL),
		"status":     resp.Status,
	}).Observe(duration)
}

// ObserveReconcile records the duration of the reconcile loop of the operator
func (m *MetricsCollector) ObserveReconcile(seconds float64) {
	m.ReconcileDuration.Observe(seconds)
}

// AddCollector adds a collector to existing metrics
func (m *MetricsCollector) AddCollector(collector prometheus.Collector) {
	m.collectors = append(m.collectors, collector)
}

// RecordSnitchCallDuration records the time taken to make a call to the Dead Man Snitch API
func (m *MetricsCollector) RecordSnitchCallDuration(duration time.Duration, operation string) {
	m.snitchCallDuration.With(prometheus.Labels{snitchMethodLabel: operation}).Observe(duration.Seconds())
}

// RecordSnitchCallError increments the error counter while calling the Dead Man Snitch API
func (m *MetricsCollector) RecordSnitchCallError() {
	m.snitchCallErrors.Inc()
}

// resourceFrom normalizes an API request URL, including removing individual namespace and
// resource names, to yield a string of the form:
//     $group/$version/$kind[/{NAME}[/...]]
// or
//     $group/$version/namespaces/{NAMESPACE}/$kind[/{NAME}[/...]]
// ...where $foo is variable, {FOO} is actually {FOO}, and [foo] is optional.
// This is so we can use it as a dimension for the apiCallCount metric, without ending up
// with separate labels for each {namespace x name}.
func resourceFrom(url *url.URL) (resource string) {
	defer func() {
		// If we can't parse, return a general bucket. This includes paths that don't start with
		// /api or /apis.
		if r := recover(); r != nil {
			// TODO(efried): Should we be logging these? I guess if we start to see a lot of them...
			resource = "{OTHER}"
		}
	}()

	tokens := strings.Split(url.Path[1:], "/")

	// First normalize to $group/$version/...
	switch tokens[0] {
	case "api":
		// Core resources: /api/$version/...
		// => core/$version/...
		tokens[0] = "core"
	case "apis":
		// Extensions: /apis/$group/$version/...
		// => $group/$version/...
		tokens = tokens[1:]
	default:
		// Something else. Punt.
		panic(1)
	}

	// Single resource, non-namespaced (including a namespace itself): $group/$version/$kind/$name
	if len(tokens) == 4 {
		// Factor out the resource name
		tokens[3] = "{NAME}"
	}

	// Kind or single resource, namespaced: $group/$version/namespaces/$nsname/$kind[/$name[/...]]
	if len(tokens) > 4 && tokens[2] == "namespaces" {
		// Factor out the namespace name
		tokens[3] = "{NAMESPACE}"

		// Single resource, namespaced: $group/$version/namespaces/$nsname/$kind/$name[/...]
		if len(tokens) > 5 {
			// Factor out the resource name
			tokens[5] = "{NAME}"
		}
	}

	resource = strings.Join(tokens, "/")

	return
}
