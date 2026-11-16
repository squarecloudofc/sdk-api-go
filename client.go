// Package squarecloud is the official Go SDK for the Square Cloud API.
//
// Create a [Client] with your API key and call the API through its groups,
// such as Account, Apps, Databases and Workspaces:
//
//	ctx := context.Background()
//	c := squarecloud.New(os.Getenv("SQUARECLOUD_API_KEY"))
//
//	me, err := c.Account.Me(ctx)
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Println(me.User.Name, len(me.Applications))
//
// Every method takes a [context.Context] first. Cancel it, or give it a
// deadline, to stop a call. A Client is safe for concurrent use, so create one
// and share it.
//
// A failed call returns an [*APIError]. Use [errors.As] to read its HTTP
// status and error code.
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
	// Version is the version of this SDK. It is part of the default
	// User-Agent header.
	Version = "4.0.0"
	// DefaultBaseURL is the address of the Square Cloud API. Use
	// [WithBaseURL] to send requests somewhere else, such as a proxy or a
	// test server.
	DefaultBaseURL = "https://api.squarecloud.app/v2"

	maxErrorBody    = 64 << 10
	maxUploadSize   = 100 << 20 // zip cap enforced by the API
	maxFileSize     = 10 << 20  // The decoded content limit of Files.Write.
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

	msgZipTooLarge  = "The file is larger than the 100 MB upload limit. Leave dependency folders and build output out of the zip, or upload a smaller file."
	msgFileTooLarge = "The content is larger than the 10 MB file write limit, so it was not sent. Upload larger files with Apps.Commit."
)

// retryBase is the first backoff step (500ms·2^n, jittered, capped at 8s). A
// variable only so tests can shrink it.
var retryBase = 500 * time.Millisecond

// Client sends requests to the Square Cloud API. Create it with [New]. Its
// fields group the calls by what they act on. A Client is safe for concurrent
// use.
type Client struct {
	// Account reads the account that owns the API key.
	Account AccountAPI
	// Service reports the status of the platform. It works without an API
	// key.
	Service ServiceAPI
	// AI sends chat requests to the Square Cloud AI.
	AI AIAPI
	// Apps deploys and controls applications.
	Apps AppsAPI
	// Databases creates and controls databases.
	Databases DatabasesAPI
	// Workspaces shares applications with other accounts.
	Workspaces WorkspacesAPI

	hc         *http.Client
	base       string
	key        string
	ua         string
	timeout    time.Duration
	maxRetries int
}

// Option changes a setting of a [Client]. Pass options to [New].
type Option func(*Client)

// WithHTTPClient makes the client send its requests through hc. A nil hc
// keeps the default, an empty [http.Client].
//
// Do not set hc.Timeout. That limit also covers reading the response, so it
// would cut realtime streams and large downloads. Use [WithTimeout], contexts
// and the timeouts of hc.Transport instead.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.hc = hc
		}
	}
}

// WithBaseURL sends requests to u instead of [DefaultBaseURL], for example to
// a proxy or a test server. A trailing slash is removed.
func WithBaseURL(u string) Option {
	return func(c *Client) { c.base = strings.TrimRight(u, "/") }
}

// WithUserAgent replaces the whole User-Agent header, which is
// "squarecloud-sdk-go/<Version>" by default.
func WithUserAgent(ua string) Option { return func(c *Client) { c.ua = ua } }

// WithMaxRetries sets how many times a failed call is repeated when that is
// safe. The default is 2, and 0 or a negative n turns retries off. See
// [APIError] for the failures that are retried.
func WithMaxRetries(n int) Option { return func(c *Client) { c.maxRetries = max(n, 0) } }

// WithTimeout sets how long a call may take when its context has no deadline.
// The default is 30 seconds. A deadline on the context always wins.
//
// Calls that the API can hold open get at least 2 minutes. They are Start,
// Stop and Restart of applications and databases, [DatabasesAPI.Create],
// snapshot Create and Restore, and [AIAPI.Chat]. Uploads,
// [AppFilesAPI.Write] with more than 1 MiB of content, and
// [Client.DownloadSnapshot] have no time limit, so only the context stops
// them. [AppsAPI.Realtime] applies the timeout only until the stream opens.
//
// A d of 0 or less turns every time limit off, the 2-minute minimum included.
func WithTimeout(d time.Duration) Option { return func(c *Client) { c.timeout = d } }

// New returns a client that authenticates every call with apiKey.
//
// New never fails. With an empty or whitespace-only key, every call except
// [ServiceAPI.Status] returns an [*APIError] with code [CodeInvalidAPIKey]
// and sends nothing.
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

// APIError is the error a call returns when the API refuses the request, the
// network fails, or a local check stops the call before anything is sent.
// Use [errors.As] to read it:
//
//	var apiErr *squarecloud.APIError
//	if errors.As(err, &apiErr) && apiErr.Code == squarecloud.CodeMissingScope {
//		fmt.Println(apiErr.Message)
//	}
//
// Status is 0 when no complete response arrived. Code is then
// [CodeNetworkError] or [CodeTimeout], and [APIError.Unwrap] returns the
// cause. Local checks also leave Status at 0. They return [CodeInvalidID] for
// an id that is empty, "." or "..", [CodeFileTooLarge] for content over the
// size limits, and [CodeInvalidAPIKey] for an empty API key.
//
// A response without an error code gets [CodeUnknown], with the server's
// message or "HTTP <status>" as Message. So does a successful response whose
// body is not valid JSON. A successful response whose body reports an error
// is returned as an error too, except a 202, which only means that the work
// is still running (see [SnapshotCreated] and [SnapshotRestored]). Errors
// from [AIAPI.Chat] follow the OpenAI format, so their Code is a lowercase
// OpenAI code such as "rate_limit_exceeded", or the error type when the code
// is missing. An API key that is unknown, revoked or expired gets 401
// ACCESS_DENIED.
//
// When you cancel the context, errors.Is(err, context.Canceled) reports
// true. When the context deadline or the client timeout passes first, Code is
// [CodeTimeout].
//
// The SDK repeats a call only when that is safe, up to the count set with
// [WithMaxRetries], and waits longer before each new attempt, up to 8
// seconds. It repeats network errors on GET requests. It repeats 503
// UPLOAD_BUSY and ANALYTICS_BUSY on any request, because the API refused
// those before doing any work. It repeats 503 DATABASE_UNAVAILABLE only on
// GET requests, because on other methods the change may already have
// started. You can repeat an idempotent call yourself, such as
// [AppEnvsAPI.Replace], [AppFilesAPI.Write] or a Delete. An upload is
// repeated only when its reader can be read again, such as an [*os.File].
//
// A 429 is never repeated. RATE_LIMITED can block the account, API key or IP
// address for about 30 minutes, and KEEP_CALM is the limit of a single
// operation.
// When Status is 429, wait before you try again.
//
// Some mistakes in the arguments return a plain error instead of an
// APIError. They are a nil upload reader, a snapshot URL or base URL that
// cannot be parsed, a value that encoding/json cannot encode, and an error
// from the [io.Writer] passed to [Client.DownloadSnapshot].
type APIError struct {
	// Status is the HTTP status of the response. It is 0 when no complete
	// response arrived or a local check stopped the call.
	Status int
	// Code is the error code, such as "APP_NOT_FOUND". Compare it with the
	// Code constants, such as [CodeAppNotFound].
	Code string
	// Message is the server's explanation of the error. It is "" when the
	// server sent only a code. When no response arrived, it is the text of
	// the cause.
	Message string
	// Method is the HTTP method of the request, such as "GET".
	Method string
	// Path is the URL path of the request, including the path of the base
	// URL. It never includes the query string, so a signed snapshot link
	// never appears in an error.
	Path string
	err  error
}

// Error formats the error as "squarecloud: <METHOD> <path>: HTTP <status>
// <CODE>: <message>". It leaves out the HTTP status when Status is 0, and the
// message when Message is empty.
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

// Unwrap returns the cause of the error, such as a network, decoding or
// context error. It returns nil when there is none.
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
			return nil, c.localError(method, p, CodeInvalidID, `The id is empty, "." or "..", so the request was not sent. Pass the id of the resource you want to reach.`)
		}
	}
	if c.key == "" && p != "/service/status" {
		return nil, c.localError(method, p, CodeInvalidAPIKey, "The API key is missing or empty, so the request was not sent. Create a key at https://squarecloud.app/account/security and pass it to New.")
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
	e := &APIError{Status: res.StatusCode, Method: res.Request.Method, Path: res.Request.URL.EscapedPath()}
	invalid := func(err error) error {
		e.Code, e.Message, e.err = CodeUnknown, "Invalid JSON in HTTP "+strconv.Itoa(res.StatusCode)+" response", err
		return e
	}
	if res.StatusCode == http.StatusAccepted {
		// Both documented 202s mean "still running", whatever the envelope
		// says: SNAPSHOT_PROCESSING (snapshot Create) and
		// SNAPSHOT_RESTORE_PROCESSING (Restore). Pending, not failed; but a
		// body that is not JSON (a proxy page) is not the API's answer.
		if err := json.NewDecoder(bytes.NewReader(data)).Decode(new(json.RawMessage)); err != nil && err != io.EOF {
			return invalid(err)
		}
		switch s := out.(type) {
		case *SnapshotCreated:
			s.Pending = true
		case *SnapshotRestored:
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
	if env.Status == "error" {
		e.Code, e.Message = cmp.Or(env.Code, CodeUnknown), env.Message
		if env.Code == "" && env.Message == "" {
			e.Message = "HTTP " + strconv.Itoa(res.StatusCode)
		}
		return e
	}
	if err != nil && err != io.EOF {
		return invalid(err)
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
		return nil, errors.New("squarecloud: the upload reader is nil and must be an open file or another io.Reader")
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

// DownloadSnapshot writes the snapshot archive at link to w as it arrives,
// without holding it in memory. Pass the URL field of a [Snapshot] or a
// [SnapshotCreated]. Those links stop working 1 hour after they are issued.
//
// Your API key is not sent with the download, and the link never appears in a
// returned error, because the link itself grants access to the archive. Only
// ctx limits how long the download takes. A network error before the download
// starts is retried like any GET. A failed download returns an [*APIError],
// and an error from w is returned unchanged.
func (c *Client) DownloadSnapshot(ctx context.Context, link string, w io.Writer) error {
	res, err := c.sendRetry(ctx, http.MethodGet, true, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
		if err != nil {
			return nil, errors.New("squarecloud: the snapshot link is not a valid URL and should be the URL field of a Snapshot or SnapshotCreated")
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
