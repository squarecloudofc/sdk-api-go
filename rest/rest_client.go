package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/squarecloudofc/sdk-api-go/v2/squarecloud"
)

type Client interface {
	HTTPClient() *http.Client
	// Request performs a call and unmarshals the JSON response into rsBody.
	Request(method, url string, rqBody []byte, rsBody any, options ...RequestOpt) error
	// Stream performs a call and returns the raw response body without
	// reading it — used for server-sent events. The caller must Close it.
	Stream(method, url string, options ...RequestOpt) (io.ReadCloser, error)

	Close()
}

type clientImpl struct {
	config Config
	token  string
}

func NewClient(token string, opts ...ConfigOpt) Client {
	config := DefaultConfig()
	config.Apply(opts)

	return &clientImpl{
		config: *config,
		token:  token,
	}
}

func (c *clientImpl) HTTPClient() *http.Client {
	return c.config.HTTPClient
}

func (c *clientImpl) Close() {
	c.config.HTTPClient.CloseIdleConnections()
}

func (c *clientImpl) newRequest(method, url string, rqBody []byte, options []RequestOpt) (*http.Request, error) {
	req, err := http.NewRequest(method, c.config.URL+url, bytes.NewReader(rqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", c.config.UserAgent)

	if c.token != "" {
		req.Header.Set("Authorization", c.token)
	}

	cfg := DefaultRequestConfig(req)
	cfg.Apply(options)

	return cfg.Request, nil
}

func (c *clientImpl) Request(method, url string, rqBody []byte, rsBody any, options ...RequestOpt) error {
	req, err := c.newRequest(method, url, rqBody, options)
	if err != nil {
		return err
	}

	response, err := c.HTTPClient().Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	rawResponseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}

	c.config.Logger.Debug("response", slog.String("code", response.Status), slog.String("body", string(rawResponseBody)))

	switch response.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		if rsBody != nil && len(rawResponseBody) > 0 {
			if err := json.Unmarshal(rawResponseBody, rsBody); err != nil {
				return fmt.Errorf("error unmarshalling response body: %w", err)
			}
		}

		return nil
	default:
		var r squarecloud.APIResponse[any]
		// Ignore unmarshal failures here — non-JSON error bodies still get a
		// useful APIError with the HTTP status.
		_ = json.Unmarshal(rawResponseBody, &r)

		return parseError(response.StatusCode, &r)
	}
}

func (c *clientImpl) Stream(method, url string, options ...RequestOpt) (io.ReadCloser, error) {
	req, err := c.newRequest(method, url, nil, options)
	if err != nil {
		return nil, err
	}

	response, err := c.HTTPClient().Do(req)
	if err != nil {
		return nil, err
	}

	if response.StatusCode < 200 || response.StatusCode > 299 {
		defer response.Body.Close()

		rawResponseBody, _ := io.ReadAll(response.Body)

		var r squarecloud.APIResponse[any]
		_ = json.Unmarshal(rawResponseBody, &r)

		return nil, parseError(response.StatusCode, &r)
	}

	return response.Body, nil
}
