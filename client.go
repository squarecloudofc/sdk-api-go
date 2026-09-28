// Package squarecloud is the official Go SDK for the Square Cloud API v2
// (contract pinned in spec/openapi.json). It has no dependencies besides the
// standard library.
//
//	c := squarecloud.New(os.Getenv("SQUARECLOUD_API_KEY"))
//	me, err := c.Account.Me(ctx)
//
// Every method takes a context.Context first. A *Client is safe for concurrent
// use; build it once and reuse it.
//
// Workspace-shared applications: wherever an appID is accepted you can pass the
// composite "<appId>-<workspaceId>" to act on an app shared with you through a
// workspace. The API splits it server side.
//
// Errors from the API are *APIError values; inspect them with errors.As.
package squarecloud

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/rand/v2"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// Version is the SDK version, sent in the User-Agent header.
	Version = "3.0.0"
	// DefaultBaseURL is the API root. Paths such as "/apps" are appended to it.
	DefaultBaseURL = "https://api.squarecloud.app/v2"

	maxErrorBody    = 64 << 10
	maxUploadSize   = 100 << 20 // zip cap enforced by the API
	maxFileSize     = 10 << 20  // decoded content cap of PUT /apps/{id}/files
	largeFile       = 1 << 20   // Files.Write content above this gets no default deadline
	maxBackoff      = 8 * time.Second
	defaultTimeout  = 30 * time.Second
	defaultRetries  = 2
	realtimeRetries = 3
	// heldFloor is the least timeout of the calls the API holds for up to
	// 95s: start, stop and restart, database create, snapshot create and
	// restore.
	heldFloor = 2 * time.Minute
	// aiFloor is the least timeout of AI.Chat: the gateway gives the whole
	// request one 90s deadline and answers 503 server_overloaded past it.
	aiFloor = 2 * time.Minute

	msgZipTooLarge  = "Uploads are limited to 100 MB"
	msgFileTooLarge = "File content is limited to 10 MB"
)

// retryBase is the first backoff step (500ms·2^n, jittered, capped at 8s). A
// variable only so tests can shrink it.
var retryBase = 500 * time.Millisecond

// Client talks to the Square Cloud API. Build it with New.
type Client struct {
	Account    AccountAPI
	Service    ServiceAPI
	AI         AIAPI
	Apps       AppsAPI
	Databases  DatabasesAPI
	Workspaces WorkspacesAPI

	hc         *http.Client
	base       string
	key        string
	ua         string
	timeout    time.Duration
	maxRetries int
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient sets the underlying *http.Client. Do not set
// http.Client.Timeout on it: that timeout also covers reading the body and
// would cut realtime streams and large downloads. Use contexts and Transport
// timeouts instead.
// A nil hc keeps the default.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.hc = hc
		}
	}
}

// WithBaseURL overrides DefaultBaseURL (for tests or proxies).
func WithBaseURL(u string) Option {
	return func(c *Client) { c.base = strings.TrimRight(u, "/") }
}

// WithUserAgent overrides the default "squarecloud-sdk-go/<version>" User-Agent.
func WithUserAgent(ua string) Option { return func(c *Client) { c.ua = ua } }

// WithMaxRetries sets how many times a retryable failure is retried (default 2,
// 0 disables retries). See APIError for what is retryable.
func WithMaxRetries(n int) Option { return func(c *Client) { c.maxRetries = max(n, 0) } }

// WithTimeout sets the default deadline of a call whose ctx has none (default
// 30s). Held operations (Start, Stop, Restart, Databases.Create, snapshot
// Create and Restore) and AI.Chat get at least 2 minutes. Uploads,
// Files.Write above 1 MiB and downloads get none, and a realtime stream gets
// it only until the response headers arrive. d <= 0 disables every default
// deadline, the floors included; a ctx deadline always wins.
func WithTimeout(d time.Duration) Option { return func(c *Client) { c.timeout = d } }

// New returns a Client authenticated with apiKey. The key is sent as-is in the
// Authorization header. With an empty or whitespace-only key every call
// except Service.Status fails locally with CodeInvalidAPIKey (New itself
// returns no error).
func New(apiKey string, opts ...Option) *Client {
	if strings.TrimSpace(apiKey) == "" {
		apiKey = ""
	}
	c := &Client{
		hc:         &http.Client{},
		base:       DefaultBaseURL,
		key:        apiKey,
		ua:         "squarecloud-sdk-go/" + Version,
		timeout:    defaultTimeout,
		maxRetries: defaultRetries,
	}
	for _, o := range opts {
		o(c)
	}
	c.Account = AccountAPI{c}
	c.Service = ServiceAPI{c}
	c.AI = AIAPI{c}
	c.Apps = AppsAPI{
		c:         c,
		Files:     AppFilesAPI{c},
		Envs:      AppEnvsAPI{c},
		Snapshots: SnapshotsAPI{c, "/apps/"},
		Network:   AppNetworkAPI{c},
		Deploys:   AppDeploysAPI{c},
	}
	c.Databases = DatabasesAPI{c: c, Snapshots: SnapshotsAPI{c, "/databases/"}}
	c.Workspaces = WorkspacesAPI{c: c, Members: WorkspaceMembersAPI{c}, Apps: WorkspaceAppsAPI{c}}
	return c
}

// APIError is every SDK failure except caller-side ones (a nil upload reader,
// an unparsable snapshot or base URL, an input encoding/json rejects, a
// failing io.Writer in DownloadSnapshot):
//   - a non-2xx response, or a 2xx whose envelope says {"status":"error"};
//   - a 2xx body that is not valid JSON (CodeUnknown, cause in Unwrap);
//   - a network failure before or while reading the response, e.g. a
//     deadline hit mid-body (Status 0, CodeNetworkError or CodeTimeout,
//     Message is the cause's text, cause in Unwrap);
//   - a local check, sent nowhere (Status 0, CodeInvalidID, CodeFileTooLarge
//     or CodeInvalidAPIKey).
//
// A canceled ctx is detectable with errors.Is(err, context.Canceled)
// wherever it ends the call; a deadline hit during a retry wait is
// CodeTimeout.
//
// A response without a code gets CodeUnknown and, if it has no message
// either, Message "HTTP <status>". Both error dialects are decoded: the Square
// envelope {status, code, message} and the OpenAI-style {error: {message,
// type, param, code}} that every AI.Chat error uses, auth, 429 and 503
// included (an object in "error" wins; Code is its lowercase code, or type
// when code is absent). An expired API key is 401 ACCESS_DENIED, like an
// unknown one.
//
// Retries (WithMaxRetries): network errors on GET (a body that breaks off
// mid-read included), 503 UPLOAD_BUSY and ANALYTICS_BUSY, and 503
// DATABASE_UNAVAILABLE on GET only: it can fire after a mutation's handler
// has started, so the SDK does not repeat mutations; the caller may retry an
// idempotent one (e.g. Envs.Replace, Files.Write, a Delete). An HTTP 429 (RATE_LIMITED,
// which can be a 30-minute block, KEEP_CALM, the deprecated RATE_LIMIT and
// RATE_LIMIT_EXCEEDED, or the AI's rate_limit_exceeded) is never retried;
// check Status == 429.
type APIError struct {
	Status  int
	Code    string
	Message string
	Method  string
	Path    string
	err     error
}

// Error renders "squarecloud: <METHOD> <path>: HTTP <status> <CODE>: <message>".
// The HTTP part is left out when Status is 0, and the message when empty.
func (e *APIError) Error() string {
	s := "squarecloud: " + e.Method + " " + e.Path + ":"
	if e.Status != 0 {
		s += " HTTP " + strconv.Itoa(e.Status)
	}
	s += " " + e.Code
	if e.Message != "" {
		s += ": " + e.Message
	}
	return s
}

// Unwrap returns the cause (a transport, decode or ctx error), or nil.
func (e *APIError) Unwrap() error { return e.err }

// netError is a failure with no complete response; the cause is the message.
func netError(code, method, path string, err error) *APIError {
	return &APIError{Code: code, Message: err.Error(), Method: method, Path: path, err: err}
}

// raw marks a response that is not wrapped in {status, response}.
type raw struct{ v any }

// call is do for operations with a response body.
func call[T any](ctx context.Context, c *Client, method, path string, in any) (T, error) {
	var out T
	err := c.do(ctx, method, path, in, &out)
	return out, err
}

// bound applies the client timeout, at least floor, when ctx has no deadline.
// A timeout <= 0 disables it, the floor included.
func (c *Client) bound(ctx context.Context, floor time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok || c.timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, max(c.timeout, floor))
}

// do sends one API call. in is nil, an *upload, a pre-encoded
// json.RawMessage (Files.Write, which bounds ctx itself), or a value encoded
// as JSON. out is nil, a pointer decoded from the envelope's "response", or
// raw{ptr}. JSON calls get the client timeout unless ctx has a deadline;
// uploads get none, so a slow uplink is bounded only by ctx.
func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	var (
		body  []byte
		ctype = "application/json"
		up, _ = in.(*upload)
		timed = true
	)
	switch b := in.(type) {
	case nil:
		ctype = ""
	case *upload:
		ctype, timed = b.ctype, false
		if b.size > maxUploadSize {
			return c.localError(method, path, CodeFileTooLarge, msgZipTooLarge)
		}
	case json.RawMessage:
		body, timed = b, false // json.Marshal would copy it twice more
	default:
		var err error
		if body, err = json.Marshal(in); err != nil {
			return err
		}
	}
	if timed {
		var cancel context.CancelFunc
		ctx, cancel = c.bound(ctx, 0)
		defer cancel()
	}
	var data []byte
	res, err := c.sendRetry(ctx, method, up == nil || up.ra != nil, func() (*http.Request, error) {
		var rd io.Reader
		switch {
		case up != nil:
			rd = up.body()
		case body != nil:
			rd = bytes.NewReader(body) // NewRequest sets Content-Length and GetBody
		}
		req, err := c.newRequest(ctx, method, path, rd, ctype)
		if err != nil {
			return nil, err
		}
		if up != nil && up.size >= 0 {
			req.ContentLength = int64(len(up.head)) + up.size + int64(len(up.tail))
		}
		if up != nil && up.ra != nil {
			// Lets net/http replay the body: an HTTP/2 retry after a lost
			// connection, or a 307/308 redirect.
			req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(up.body()), nil }
		}
		return req, nil
	}, func(res *http.Response) (err error) {
		// Read inside the retry loop: a GET whose body breaks off is retried.
		defer closeBody(res.Body)
		if data, err = io.ReadAll(res.Body); err != nil {
			code, _ := netCode(err)
			return netError(code, method, res.Request.URL.EscapedPath(), err)
		}
		return nil
	})
	if errors.Is(err, errZipTooLarge) {
		return c.localError(method, path, CodeFileTooLarge, msgZipTooLarge)
	}
	if err != nil {
		return err
	}
	return decode(res, data, out)
}

// sendRetry sends the request build returns, then runs read on a 2xx
// (nil: the caller reads the body), retrying a retryable failure of either
// (see retryable) with backoff. replayable is false for a one-shot body. A
// ctx that ends during a backoff wait gives CodeTimeout or CodeNetworkError
// wrapping ctx.Err(), never the previous attempt's error.
func (c *Client) sendRetry(ctx context.Context, method string, replayable bool, build func() (*http.Request, error), read func(*http.Response) error) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		req, err := build()
		if err != nil {
			return nil, err
		}
		res, err := c.send(req)
		if err == nil && read != nil {
			err = read(res)
		}
		if err == nil {
			return res, nil
		}
		var e *APIError
		if !replayable || !errors.As(err, &e) || attempt >= c.maxRetries || !retryable(method, e) {
			return nil, err
		}
		if !wait(ctx, backoff(attempt)) {
			return nil, ctxError(ctx, method, req.URL.EscapedPath())
		}
	}
}

// ctxError is the error of a ctx that ended while no request was in flight,
// classified like one that ends mid-request.
func ctxError(ctx context.Context, method, path string) *APIError {
	code, _ := netCode(ctx.Err())
	return netError(code, method, path, ctx.Err())
}

// localError is a check that failed before anything was sent. Path matches
// what send reports (the base URL's path included, query excluded).
func (c *Client) localError(method, path, code, msg string) *APIError {
	p, _, _ := strings.Cut(path, "?")
	if u, err := url.Parse(c.base + p); err == nil {
		p = u.EscapedPath()
	}
	return &APIError{Code: code, Message: msg, Method: method, Path: p}
}

// newRequest builds an API request. It rejects a path with an empty, "." or
// ".." segment: IDs are escaped, so only such an ID produces one, and it would
// reach another route. An empty key is rejected except for the public status.
func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader, ctype string) (*http.Request, error) {
	p, _, _ := strings.Cut(path, "?")
	for _, seg := range strings.Split(p[1:], "/") {
		if seg == "" || seg == "." || seg == ".." {
			return nil, c.localError(method, p, CodeInvalidID, `Empty, "." and ".." are not valid ids`)
		}
	}
	if c.key == "" && p != "/service/status" {
		return nil, c.localError(method, p, CodeInvalidAPIKey, "The API key is empty")
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept", "application/json")
	if c.key != "" {
		req.Header.Set("Authorization", c.key)
	}
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	return req, nil
}

// send runs req. Non-2xx responses and transport failures come back as
// *APIError with the body already closed.
func (c *Client) send(req *http.Request) (*http.Response, error) {
	res, err := c.hc.Do(req)
	if err != nil {
		code, _ := netCode(err)
		// Keep only the cause: *url.Error prints the full URL, and a presigned
		// snapshot URL is a bearer credential.
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return nil, netError(code, req.Method, req.URL.EscapedPath(), err)
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		defer closeBody(res.Body)
		var b struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Error   *struct {
				Code    string `json:"code"`
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(res.Body, maxErrorBody)).Decode(&b)
		e := &APIError{Status: res.StatusCode, Code: b.Code, Message: b.Message, Method: req.Method, Path: req.URL.EscapedPath()}
		if b.Error != nil { // an object in "error" is the OpenAI dialect
			e.Code, e.Message = cmp.Or(b.Error.Code, b.Error.Type), b.Error.Message
		}
		if e.Code == "" {
			e.Code, e.Message = CodeUnknown, cmp.Or(e.Message, "HTTP "+strconv.Itoa(res.StatusCode))
		}
		return nil, e
	}
	return res, nil
}

// decode parses a 2xx body into out. An envelope that says "error" is an
// *APIError whatever the HTTP status; raw bodies (status, AI) have no envelope.
func decode(res *http.Response, data []byte, out any) error {
	if res.StatusCode == http.StatusAccepted {
		// The only documented 202 is SNAPSHOT_PROCESSING: pending, not failed.
		if s, ok := out.(*SnapshotCreated); ok {
			s.Pending = true
		}
		return nil
	}
	var env struct {
		Status   string `json:"status"`
		Code     string `json:"code"`
		Message  string `json:"message"`
		Response any    `json:"response"`
	}
	target := any(&env)
	switch r := out.(type) {
	case raw:
		target = r.v
	case nil:
		env.Response = new(json.RawMessage) // a void call still checks the envelope
	default:
		env.Response = out
	}
	err := json.NewDecoder(bytes.NewReader(data)).Decode(target) // tolerates an empty body and trailing data
	e := &APIError{Status: res.StatusCode, Method: res.Request.Method, Path: res.Request.URL.EscapedPath()}
	if env.Status == "error" {
		e.Code, e.Message = cmp.Or(env.Code, CodeUnknown), env.Message
		if env.Code == "" && env.Message == "" {
			e.Message = "HTTP " + strconv.Itoa(res.StatusCode)
		}
		return e
	}
	if err != nil && err != io.EOF {
		e.Code, e.Message, e.err = CodeUnknown, "Invalid JSON in HTTP "+strconv.Itoa(res.StatusCode)+" response", err
		return e
	}
	return nil
}

// netCode classifies a transport failure: CodeTimeout for a deadline,
// CodeNetworkError otherwise. ok is false when err does not look like one
// (a JSON syntax or io.Writer error).
func netCode(err error) (code string, ok bool) {
	var ne net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded) || errors.As(err, &ne) && ne.Timeout():
		return CodeTimeout, true
	case errors.Is(err, context.Canceled) || errors.As(err, &ne) || errors.Is(err, io.ErrUnexpectedEOF):
		return CodeNetworkError, true
	}
	return CodeNetworkError, false
}

// closeBody drains a little so the keep-alive connection can be reused.
func closeBody(b io.ReadCloser) {
	_, _ = io.CopyN(io.Discard, b, 4<<10)
	_ = b.Close()
}

func retryable(method string, e *APIError) bool {
	switch {
	case e.Status == 0:
		// Mutations are not idempotent, and a TIMEOUT (any deadline, the
		// transport's included) is never retried.
		return e.Code == CodeNetworkError && method == http.MethodGet
	case e.Status != http.StatusServiceUnavailable:
		return false // 429 included: RATE_LIMITED can be a 30-minute block
	case e.Code == CodeUploadBusy, e.Code == CodeAnalyticsBusy:
		return true // rejected before any processing
	case e.Code == CodeDatabaseUnavailable:
		return method == http.MethodGet
	}
	return false
}

// backoff is the wait before retry attempt+1: min(8s, 500ms·2^attempt)·U(0.5, 1).
func backoff(attempt int) time.Duration {
	d := maxBackoff
	if attempt < 16 { // a larger shift could overflow
		d = min(retryBase<<attempt, maxBackoff)
	}
	return d/2 + rand.N(d/2+1)
}

// wait sleeps d; false if ctx ended first.
func wait(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// upload streams a file as multipart/form-data with a single "file" part:
// head + file + tail, never buffered. A reader whose size can be found
// (io.Seeker) gets a Content-Length. One that is also an io.ReaderAt
// (*os.File, *bytes.Reader, ...) is read through a fresh section per attempt,
// so it can be retried without racing the previous attempt's reads.
type upload struct {
	r          io.Reader
	ra         io.ReaderAt // nil: one attempt only
	head, tail string
	ctype      string
	start      int64 // offset of the first byte to send
	size       int64 // -1 when unknown (sent chunked, capped while streaming)
}

var (
	// errZipTooLarge stops a capped stream; do reports it as msgZipTooLarge.
	errZipTooLarge = errors.New("squarecloud: zip over 100 MB")
	// filenameSafe keeps a filename inside its quoted header value.
	filenameSafe = strings.NewReplacer(`"`, "%22", "\r", "", "\n", "")
)

// newUpload names the part filename, else r's own name (the base of an
// *os.File's Name()), else fallback.
func newUpload(r io.Reader, filename, fallback string) (*upload, error) {
	// A nil *os.File (an unchecked os.Open) is nil too: its Name would panic.
	if f, ok := r.(*os.File); r == nil || ok && f == nil {
		return nil, errors.New("squarecloud: nil upload reader")
	}
	if f, ok := r.(interface{ Name() string }); ok && filename == "" && filepath.Base(f.Name()) != "." {
		filename = filepath.Base(f.Name())
	}
	u := &upload{r: r, size: -1}
	if s, ok := r.(io.Seeker); ok {
		if cur, err := s.Seek(0, io.SeekCurrent); err == nil {
			if end, err := s.Seek(0, io.SeekEnd); err == nil {
				if _, err := s.Seek(cur, io.SeekStart); err != nil {
					return nil, err
				}
				u.start, u.size = cur, end-cur
				u.ra, _ = r.(io.ReaderAt)
			}
		}
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if _, err := mw.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {`form-data; name="file"; filename="` + filenameSafe.Replace(cmp.Or(filename, fallback)) + `"`},
		"Content-Type":        {"application/octet-stream"},
	}); err != nil {
		return nil, err
	}
	u.head = buf.String()
	buf.Reset()
	_ = mw.Close()
	u.tail = buf.String()
	u.ctype = mw.FormDataContentType()
	return u, nil
}

// body returns one attempt's request body.
func (u *upload) body() io.Reader {
	r := u.r
	switch {
	case u.ra != nil:
		r = io.NewSectionReader(u.ra, u.start, u.size)
	case u.size < 0:
		r = &capped{r: u.r}
	}
	return io.MultiReader(strings.NewReader(u.head), r, strings.NewReader(u.tail))
}

// capped fails a stream of unknown size once it passes the API's zip cap, so
// the upload stops there instead of running to the end for a 413.
type capped struct {
	r io.Reader
	n int64
}

func (c *capped) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if c.n += int64(n); c.n > maxUploadSize {
		return n, errZipTooLarge
	}
	return n, err
}

// DownloadSnapshot streams a snapshot archive (SnapshotCreated.URL, a
// presigned URL) into w. The API key is not sent, and the URL never appears
// in the returned error. The download has no deadline besides ctx; a network
// error before the response is retried like any GET.
func (c *Client) DownloadSnapshot(ctx context.Context, link string, w io.Writer) error {
	res, err := c.sendRetry(ctx, http.MethodGet, true, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
		if err != nil {
			return nil, errors.New("squarecloud: invalid snapshot URL")
		}
		req.Header.Set("User-Agent", c.ua)
		return req, nil
	}, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if _, err = io.Copy(w, res.Body); err != nil {
		if code, ok := netCode(err); ok { // a writer error is returned as is
			return netError(code, http.MethodGet, res.Request.URL.EscapedPath(), err)
		}
	}
	return err
}
