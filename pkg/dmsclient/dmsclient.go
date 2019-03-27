package dmsclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const (
	apiEndpoint = "https://api.deadmanssnitch.com/v1"
)

// Snitch Struct
type Snitch struct {
	Name        string   `json:"name"`
	Token       string   `json:"token"`
	Href        string   `json:"href"`
	Tags        []string `json:"tags"`
	Notes       string   `json:"notes"`
	Status      string   `json:"status"`
	CheckedInAt string   `json:"checked_in_at"`
	CheckInURL  string   `json:"check_in_url"`
	CreatedAt   string   `json:"created_at"`
	Interval    string   `json:"interval"`
	AlertType   string   `json:"alert_type"`
}

func defaultURL() *url.URL {
	url, _ := url.Parse(apiEndpoint)
	return url
}

// Client wraps http client
type Client struct {
	authToken  string
	BaseURL    *url.URL
	httpClient *http.Client
}

// NewClient creates an API client
func NewClient(authToken string) *Client {
	return &Client{
		authToken:  authToken,
		BaseURL:    defaultURL(),
		httpClient: http.DefaultClient,
		//httpClient: newDefaultHTTPClient(),
	}
}

func (c *Client) newRequest(method, path string, body interface{}) (*http.Request, error) {
	rel := &url.URL{Path: path}
	u := c.BaseURL.ResolveReference(rel)
	var buf io.ReadWriter

	if body != nil {
		buf = new(bytes.Buffer)
		err := json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequest(method, u.String(), buf)
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

func (c *Client) do(req *http.Request) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return resp, fmt.Errorf("Error calling the API endpoint: %v", err)
	}

	return resp, nil
}

// ListAll snitches
func (c *Client) ListAll() ([]Snitch, error) {
	req, err := c.newRequest("GET", "/v1/snitches", nil)
	if err != nil {
		return nil, err
	}

	resp, _ := c.do(req)
	if err != nil {
		return nil, err
	}

	var snitches []Snitch
	err = json.NewDecoder(resp.Body).Decode(&snitches)
	return snitches, err
}

//List a single snitch
func (c *Client) List(snitchToken string) (Snitch, error) {
	var snitch Snitch

	req, err := c.newRequest("GET", "/v1/snitches/"+snitchToken, nil)
	if err != nil {
		return snitch, err
	}

	resp, _ := c.do(req)
	if err != nil {
		return snitch, err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&snitch)

	return snitch, err
}

// Create a snitch
func (c *Client) Create(newSnitch Snitch) (Snitch, error) {
	var snitch Snitch
	req, err := c.newRequest("POST", "/v1/snitches", newSnitch)
	if err != nil {
		return snitch, err
	}
	resp, _ := c.do(req)
	if err != nil {
		return snitch, err
	}

	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&snitch)

	return snitch, err
}

// Delete a snitch
func (c *Client) Delete(snitchToken string) (bool, error) {
	req, err := c.newRequest("DELETE", "/v1/snitches/"+snitchToken, nil)
	if err != nil {
		return false, err
	}
	resp, _ := c.do(req)
	if err != nil {
		return false, err
	}

	if resp.StatusCode == 204 {
		return true, nil
	}

	return false, nil
}

// FindSnitchesByName This will search for snitches using a name. This
// could return multiple snitches, as the same name may be used multiple times
func (c *Client) FindSnitchesByName(snitchName string) ([]Snitch, error) {
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

// Update
func (c *Client) Update(updateSnitch Snitch) (Snitch, error) {
	var snitch Snitch
	req, err := c.newRequest("PATCH", "/v1/snitches/"+updateSnitch.Token, updateSnitch)
	if err != nil {
		return snitch, err
	}
	resp, _ := c.do(req)
	if err != nil {
		return snitch, err
	}

	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&snitch)

	return snitch, err
}
