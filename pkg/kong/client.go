package kong

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/sirupsen/logrus"
)

type request struct {
	method string
	url    string
	query  *url.Values
	body   io.Reader
	form   *url.Values
}

// Client Kong client API
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewKongClient Create new Kong API client
func NewKongClient(httpClient *http.Client, baseURL string) *Client {
	_, err := url.Parse(baseURL)
	if err != nil {
		panic(fmt.Errorf("Invalid Kong API endpoint %s: %s", baseURL, err))
	}
	kongClient := Client{
		httpClient: httpClient,
		baseURL:    baseURL,
	}
	logrus.WithFields(logrus.Fields{
		"endpoint": kongClient.baseURL,
	}).Debug("Kong client configuration")
	return &kongClient
}

// doRequest Do HTTP request
func (k *Client) doRequest(r *request) (*http.Response, error) {
	req, err := http.NewRequest(r.method, k.baseURL+r.url, r.body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if r.query != nil {
		req.URL.RawQuery = r.query.Encode()
	}
	return k.httpClient.Do(req)
}

// get HTTP GET
func (k *Client) get(url string, query *url.Values) (*http.Response, error) {
	req := &request{method: "GET", url: url, query: query}
	return k.doRequest(req)
}

// delete HTTP DELETE
func (k *Client) delete(url string, query *url.Values) (*http.Response, error) {
	req := &request{method: "DELETE", url: url, query: query}
	return k.doRequest(req)
}

// post HTTP POST
func (k *Client) post(url string, query *url.Values, body io.Reader) (*http.Response, error) {
	req := &request{method: "POST", url: url, query: query, body: body}
	return k.doRequest(req)
}

// put HTTP PUT
func (k *Client) put(url string, query *url.Values, body io.Reader) (*http.Response, error) {
	req := &request{method: "PUT", url: url, query: query, body: body}
	return k.doRequest(req)
}
