package squarecloud

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The shipped values, read before init shrinks them for the tests.
var shippedRetryBase, shippedPace = retryBase, realtimePace

func init() {
	retryBase = time.Millisecond
	realtimePace = 10 * time.Millisecond
	http.DefaultTransport = loopbackOnly{http.DefaultTransport}
}

// loopbackOnly guarantees the suite stays offline: any request to a host other
// than an httptest server fails instead of reaching the real API.
type loopbackOnly struct{ rt http.RoundTripper }

func (l loopbackOnly) RoundTrip(r *http.Request) (*http.Response, error) {
	if h := r.URL.Hostname(); h != "127.0.0.1" && h != "::1" && h != "localhost" {
		return nil, errors.New("test tried to reach " + h)
	}
	return l.rt.RoundTrip(r)
}

func TestSuiteIsOffline(t *testing.T) {
	_, err := New("k").Service.Status(context.Background())
	if e := asAPIError(t, err); e.Code != CodeNetworkError || !strings.Contains(err.Error(), "test tried to reach api.squarecloud.app") {
		t.Fatal(err)
	}
}

// testClient points a Client at an httptest server. Everything is offline.
func testClient(t *testing.T, h http.HandlerFunc, opts ...Option) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New("test-key", append([]Option{WithBaseURL(srv.URL + "/v2")}, opts...)...)
}

func reply(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

// hangUp drops the TCP connection without an HTTP response.
func hangUp(w http.ResponseWriter) {
	conn, _, _ := w.(http.Hijacker).Hijack()
	_ = conn.Close()
}

func asAPIError(t *testing.T, err error) *APIError {
	t.Helper()
	var e *APIError
	if !errors.As(err, &e) {
		t.Fatalf("want *APIError, got %T: %v", err, err)
	}
	return e
}
