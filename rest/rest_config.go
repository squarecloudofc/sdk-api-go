package rest

import (
	"log/slog"
	"net/http"
	"time"
)

func DefaultConfig() *Config {
	return &Config{
		Logger:     slog.Default(),
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		URL:        APIURL,
		UserAgent:  "Square GO",
	}
}

type Config struct {
	Logger     *slog.Logger
	HTTPClient *http.Client
	URL        string
	UserAgent  string
}

type ConfigOpt func(config *Config)

func (c *Config) Apply(opts []ConfigOpt) {
	for _, opt := range opts {
		opt(c)
	}
}

func WithLogger(logger *slog.Logger) ConfigOpt {
	return func(config *Config) {
		config.Logger = logger
	}
}

func WithHTTPClient(httpClient *http.Client) ConfigOpt {
	return func(config *Config) {
		config.HTTPClient = httpClient
	}
}

func WithURL(url string) ConfigOpt {
	return func(config *Config) {
		config.URL = url
	}
}

func WithUserAgent(userAgent string) ConfigOpt {
	return func(config *Config) {
		config.UserAgent = userAgent
	}
}
