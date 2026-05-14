package dmsclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/openshift/deadmanssnitch-operator/pkg/localmetrics"
)

const (
	apiEndpoint = "https://api.deadmanssnitch.com/v1"
)

// Client is a wrapper interface for the dmsClient to allow for easier testing
type Client interface {
	ListAll() ([]Snitch, error)
	List(snitchToken string) (Snitch, error)
	Create(newSnitch Snitch) (Snitch, error)
	Delete(snitchToken string) (bool, error)
	FindSnitchesByName(snitchName string) ([]Snitch, error)
	Update(updateSnitch Snitch) (Snitch, error)
	CheckIn(s Snitch) error
}

// SnitchType Struct
type SnitchType struct {
	Interval string `json:"interval"`
}

// Snitch Struct
type Snitch struct {
	Name        string     `json:"name"`
	Token       string     `json:"token"`
	Href        string     `json:"href"`
	Tags        []string   `json:"tags"`
	Notes       string     `json:"notes"`
	Status      string     `json:"status"`
	CheckedInAt string     `json:"checked_in_at"`
	CheckInURL  string     `json:"check_in_url"`
	CreatedAt   string     `json:"created_at"`
	Interval    string     `json:"interval"`
	AlertType   string     `json:"alert_type"`
	AlertEmail  []string   `json:"alert_email"`
	Type        SnitchType `json:"type"`
}

// Client wraps http client
type dmsClient struct {
	authToken        string
	httpClient       *http.Client
	metricsCollector *localmetrics.MetricsCollector
}

// NewClient creates an API client
func NewClient(authToken string, collector *localmetrics.MetricsCollector) Client {
	return &dmsClient{
		authToken:        authToken,
		httpClient:       http.DefaultClient,
		metricsCollector: collector,
	}
}

// NewSnitch creates a new Snitch only requiring a few items
func NewSnitch(name string, tags []string, interval string, alertType string) Snitch {
	return Snitch{
		Name:      name,
		Tags:      tags,
		Interval:  interval,
		AlertType: alertType,
	}
}

func (c *dmsClient) newRequest(ctx context.Context, method, path string, body interface{}) (*http.Request, error) {
	base, _ := url.Parse(apiEndpoint)
	rel := &url.URL{Path: path}
	u := base.ResolveReference(rel)
	var buf io.ReadWriter

	if body != nil {
		buf = new(bytes.Buffer)
		err := json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), buf)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "golang httpClient")
	req.SetBasicAuth(c.authToken, "")
	return req, nil
}

func (c *dmsClient) do(req *http.Request, operation string) (*http.Response, error) {
	start := time.Now()
	defer func() {
		c.metricsCollector.ObserveSnitchCallDuration(time.Since(start).Seconds(), operation)
	}()
	resp, err := c.httpClient.Do(req) // #nosec G107 G704

	// raise an error if unable to authenticate to DMS service
	if resp.StatusCode == 401 {
		err = fmt.Errorf("unauthorized error: please check the deadmanssnitch credentials")
	}

	if err != nil {
		c.metricsCollector.ObserveSnitchCallError()
		return resp, fmt.Errorf("error calling the API endpoint: %w", err)
	}

	return resp, nil
}

// ListAll snitches
func (c *dmsClient) ListAll() ([]Snitch, error) {
	req, err := c.newRequest(context.Background(), "GET", "/v1/snitches", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(req, "list_all")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var snitches []Snitch
	decodeErr := json.NewDecoder(resp.Body).Decode(&snitches)
	if decodeErr != nil {
		err = fmt.Errorf("error listing all snitches: %w", decodeErr)
	}

	return snitches, err
}

// List a single snitch
func (c *dmsClient) List(snitchToken string) (Snitch, error) {
	var snitch Snitch

	req, err := c.newRequest(context.Background(), "GET", "/v1/snitches/"+snitchToken, nil)
	if err != nil {
		return snitch, err
	}

	resp, err := c.do(req, "describe")
	if err != nil {
		return snitch, err
	}
	defer func() { _ = resp.Body.Close() }()

	decodeErr := json.NewDecoder(resp.Body).Decode(&snitch)
	if decodeErr != nil {
		err = fmt.Errorf("error listing snitch: %w", decodeErr)
	}
	return snitch, err
}

// Create a snitch
func (c *dmsClient) Create(newSnitch Snitch) (Snitch, error) {
	var snitch Snitch
	req, err := c.newRequest(context.Background(), "POST", "/v1/snitches", newSnitch)
	if err != nil {
		return snitch, err
	}
	resp, err := c.do(req, "create")
	if err != nil {
		return snitch, err
	}

	defer func() { _ = resp.Body.Close() }()

	decodeErr := json.NewDecoder(resp.Body).Decode(&snitch)
	if decodeErr != nil {
		err = fmt.Errorf("error creating snitch: %w", decodeErr)
	}
	return snitch, err
}

// Delete a snitch
func (c *dmsClient) Delete(snitchToken string) (bool, error) {
	req, err := c.newRequest(context.Background(), "DELETE", "/v1/snitches/"+snitchToken, nil)
	if err != nil {
		return false, err
	}
	resp, err := c.do(req, "delete")
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 204 {
		return true, nil
	}

	return false, nil
}

// FindSnitchesByName This will search for snitches using a name. This
// could return multiple snitches, as the same name may be used multiple times
func (c *dmsClient) FindSnitchesByName(snitchName string) ([]Snitch, error) {
	var foundSnitches []Snitch
	listedSnitches, err := c.ListAll()
	if err != nil {
		return foundSnitches, err
	}

	for _, snitch := range listedSnitches {
		if snitch.Name == snitchName {
			foundSnitches = append(foundSnitches, snitch)
		}
	}

	return foundSnitches, err
}

// Update the snitch
func (c *dmsClient) Update(updateSnitch Snitch) (Snitch, error) {
	var snitch Snitch
	req, err := c.newRequest(context.Background(), "PATCH", "/v1/snitches/"+updateSnitch.Token, updateSnitch)
	if err != nil {
		return snitch, err
	}
	resp, err := c.do(req, "update")
	if err != nil {
		return snitch, err
	}

	defer func() { _ = resp.Body.Close() }()
	err = json.NewDecoder(resp.Body).Decode(&snitch)

	return snitch, err
}

// Initialize the snitch with a basic GET call to its url
func (c *dmsClient) CheckIn(s Snitch) error {
	req, err := http.NewRequestWithContext(context.Background(), "GET", s.CheckInURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "golang httpClient")

	resp, err := c.do(req, "check_in")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	return nil
}
