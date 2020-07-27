package dmsclient

import (
	operatorconfig "github.com/openshift/deadmanssnitch-operator/config"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	methodNameLabel = "method"
)

type InstrumentedClient struct {
	Client
	apiCall      *prometheus.HistogramVec
	apiCallError *prometheus.CounterVec
}

func NewInstrumentedClient(authToken string) *InstrumentedClient {
	return &InstrumentedClient{
		Client: NewClient(authToken),
		apiCall: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:        "dms_operator_snitch_client_request_duration_seconds",
			Help:        "Distribution of API call times to list snitches",
			ConstLabels: prometheus.Labels{"name": operatorconfig.OperatorName},
		}, []string{methodNameLabel}),
		apiCallError: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:        "dms_operator_snitch_client_request_errors",
			Help:        "Count of error in Dead Man's Snitch API calls",
			ConstLabels: prometheus.Labels{"name": operatorconfig.OperatorName},
		}, []string{methodNameLabel}),
	}
}

func (i InstrumentedClient) Describe(descs chan<- *prometheus.Desc) {
	i.apiCall.Describe(descs)
	i.apiCallError.Describe(descs)
}

func (i InstrumentedClient) Collect(metrics chan<- prometheus.Metric) {
	i.apiCall.Collect(metrics)
	i.apiCallError.Collect(metrics)
}

func methodLabel(methodName string) prometheus.Labels {
	return prometheus.Labels{methodNameLabel: methodName}
}

func (i InstrumentedClient) ListAll() ([]Snitch, error) {
	start := time.Now()
	snitches, err := i.Client.ListAll()
	if err != nil {
		i.apiCallError.With(methodLabel("list_all")).Inc()
		return nil, err
	}
	i.apiCall.With(methodLabel("list_all")).Observe(time.Since(start).Seconds())
	return snitches, nil
}

func (i InstrumentedClient) List(snitchToken string) (Snitch, error) {
	start := time.Now()
	snitch, err := i.Client.List(snitchToken)
	if err != nil {
		i.apiCallError.With(methodLabel("list")).Inc()
		return snitch, err
	}
	i.apiCall.With(methodLabel("list")).Observe(time.Since(start).Seconds())
	return snitch, nil
}

func (i InstrumentedClient) Create(newSnitch Snitch) (Snitch, error) {
	start := time.Now()
	snitch, err := i.Client.Create(newSnitch)
	if err != nil {
		i.apiCallError.With(methodLabel("create")).Inc()
		return snitch, err
	}
	i.apiCall.With(methodLabel("create")).Observe(time.Since(start).Seconds())
	return snitch, nil
}

func (i InstrumentedClient) Delete(snitchToken string) (bool, error) {
	start := time.Now()
	deleted, err := i.Client.Delete(snitchToken)
	if err != nil {
		i.apiCallError.With(methodLabel("delete")).Inc()
		return deleted, err
	}
	i.apiCall.With(methodLabel("delete")).Observe(time.Since(start).Seconds())
	return deleted, nil
}

func (i InstrumentedClient) FindSnitchesByName(snitchName string) ([]Snitch, error) {
	start := time.Now()
	snitches, err := i.Client.FindSnitchesByName(snitchName)
	if err != nil {
		i.apiCallError.With(methodLabel("find_by_name")).Inc()
		return nil, err
	}
	i.apiCall.With(methodLabel("find_by_name")).Observe(time.Since(start).Seconds())
	return snitches, nil
}

func (i InstrumentedClient) Update(updateSnitch Snitch) (Snitch, error) {
	start := time.Now()
	snitch, err := i.Client.Update(updateSnitch)
	if err != nil {
		i.apiCallError.With(methodLabel("update")).Inc()
		return snitch, err
	}
	i.apiCall.With(methodLabel("update")).Observe(time.Since(start).Seconds())
	return snitch, nil
}

func (i InstrumentedClient) CheckIn(s Snitch) error {
	start := time.Now()
	err := i.Client.CheckIn(s)
	if err != nil {
		i.apiCallError.With(methodLabel("check_in")).Inc()
		return err
	}
	i.apiCall.With(methodLabel("check_in")).Observe(time.Since(start).Seconds())
	return nil
}
