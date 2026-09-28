package squarecloud

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ---- Realtime (SSE) ----

func sseServer(t *testing.T, h http.HandlerFunc, opts ...Option) *Client {
	return testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		h(w, r)
	}, opts...)
}

func drain(t *testing.T, s *Realtime) ([]RealtimeEvent, error) {
	t.Helper()
	var evs []RealtimeEvent
	for {
		ev, err := s.Next()
		if err != nil {
			return evs, err
		}
		evs = append(evs, ev)
	}
}

func openStream(t *testing.T, c *Client) *Realtime {
	t.Helper()
	s, err := c.Apps.Realtime(context.Background(), "app1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

const (
	fullStatus = `{"cpu":12.5,"cpuLimit":0.5,"ram":[128,512],"status":"running","netIO":{"i":2048,"o":4096,"new":{"i":64,"o":128}},"bIO":{"i":0,"o":0},"uptime":1716000000000}`
	leanStatus = `{"cpu":13.1,"ram":[131,512],"netIO":{"i":2176,"o":4288,"new":{"i":72,"o":140}},"bIO":{"i":1,"o":2}}`
	sseFrames  = "event: system\ndata: REALTIME_CONNECTING | abc\n\n" +
		"event: status\ndata: " + fullStatus + "\n\n" +
		"event: status\ndata: " + leanStatus + "\n\n" +
		"event: logs\ndata: \x01Server listening  \n\n" +
		"event: logs\r\ndata: \x02Error: boom\r\n\r\n" +
		"event: logs\ndata: no prefix\n\n" +
		"event: error\ndata: CONTAINER_NOT_FOUND\n\n"
)

func TestSSEParse(t *testing.T) {
	var accept string
	c := sseServer(t, func(w http.ResponseWriter, r *http.Request) {
		accept = r.Header.Get("Accept")
		// Split mid-line across writes: the parser must not care about chunks.
		for _, part := range []string{sseFrames[:17], sseFrames[17:60], sseFrames[60:]} {
			_, _ = io.WriteString(w, part)
			w.(http.Flusher).Flush()
		}
	})
	evs, err := drain(t, openStream(t, c))
	if err != io.EOF || len(evs) != 7 || accept != "text/event-stream" {
		t.Fatalf("%d events, %v, accept %q", len(evs), err, accept)
	}
	if evs[0].Event != "system" || evs[0].Data != "REALTIME_CONNECTING | abc" || evs[0].Status != nil {
		t.Fatalf("%+v", evs[0])
	}
	full, lean := evs[1].Status, evs[2].Status
	if full == nil || full.CPU != 12.5 || full.CPULimit != 0.5 || full.NetIO.New.Out != 128 || evs[1].Data != fullStatus {
		t.Fatalf("%+v", full)
	}
	// The lean frame is merged: run-constant fields survive.
	if lean == nil || lean.CPU != 13.1 || lean.CPULimit != 0.5 || lean.Status != "running" ||
		lean.Uptime != 1716000000000 || lean.RAM != [2]float64{131, 512} || lean.BlockIO.Out != 2 {
		t.Fatalf("%+v", lean)
	}
	if full.CPU != 12.5 {
		t.Fatal("earlier Status must not be mutated by later frames")
	}
	if evs[3].Stream != "stdout" || evs[3].Line != "Server listening  " || evs[3].Data != "\x01Server listening  " {
		t.Fatalf("%q", evs[3].Line) // trailing spaces are data; Data stays raw
	}
	if evs[4].Stream != "stderr" || evs[4].Line != "Error: boom" {
		t.Fatalf("%+v", evs[4])
	}
	if evs[5].Stream != "stdout" || evs[5].Line != "no prefix" {
		t.Fatalf("%+v", evs[5])
	}
	if evs[6].Event != "error" || evs[6].Data != "CONTAINER_NOT_FOUND" {
		t.Fatalf("%+v", evs[6])
	}
}

func TestSSEFieldRules(t *testing.T) {
	body := ": comment\n" +
		"id: 7\nretry: 100\ndata:no space\n\n" +
		"data:  two spaces\ndata:\ndata: third\n\n" +
		"id: a\x00b\nevent: only-type\n\n" + // no data: not dispatched; a NUL id is ignored
		"data: " + strings.Repeat("x", 2<<20) + "\n\n" + // no 1 MB line cap
		"event: logs\ndata: cut off" // unterminated: discarded at EOF
	c := sseServer(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, body) })
	evs, err := drain(t, openStream(t, c))
	if err != io.EOF || len(evs) != 3 {
		t.Fatalf("%d %v", len(evs), err)
	}
	if evs[0].Event != "message" || evs[0].Data != "no space" || evs[0].ID != "7" {
		t.Fatalf("%+v", evs[0])
	}
	if evs[1].Data != " two spaces\n\nthird" || evs[1].ID != "7" {
		t.Fatalf("%q", evs[1].Data)
	}
	if len(evs[2].Data) != 2<<20 || evs[2].ID != "7" {
		t.Fatal(len(evs[2].Data), evs[2].ID)
	}
}

// A malformed status frame yields the last merged state, never nil.
func TestSSEMalformedStatus(t *testing.T) {
	c := sseServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "event: status\ndata: {bad\n\nevent: status\ndata: "+fullStatus+"\n\nevent: status\ndata: {\"cpu\":\"x\"}\n\n")
	})
	evs, err := drain(t, openStream(t, c))
	if err != io.EOF || len(evs) != 3 {
		t.Fatal(len(evs), err)
	}
	if evs[0].Status == nil || *evs[0].Status != (RealtimeStatus{}) {
		t.Fatalf("%+v", evs[0].Status)
	}
	if evs[2].Status == nil || *evs[2].Status != *evs[1].Status || evs[2].Status.CPU != 12.5 || evs[2].Data != `{"cpu":"x"}` {
		t.Fatalf("%+v", evs[2].Status)
	}
}

// Status frames are shallow-merged: a top-level key replaces its whole value.
func TestSSEStatusShallowMerge(t *testing.T) {
	c := sseServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "event: status\ndata: "+fullStatus+"\n\nevent: status\ndata: {\"netIO\":{\"i\":1},\"status\":null}\n\n")
	})
	evs, err := drain(t, openStream(t, c))
	if err != io.EOF || len(evs) != 2 {
		t.Fatal(len(evs), err)
	}
	want := *evs[0].Status
	want.NetIO, want.Status = NetIO{In: 1}, ""
	if *evs[1].Status != want {
		t.Fatalf("%+v", *evs[1].Status)
	}
}

func TestSSEStatusCheckedFirst(t *testing.T) {
	c := sseServer(t, reply(429, `{"status":"error","code":"REALTIME_MAX_CONNECTIONS","message":"max 5"}`))
	_, err := c.Apps.Realtime(context.Background(), "app1")
	if e := asAPIError(t, err); e.Status != 429 || e.Code != "REALTIME_MAX_CONNECTIONS" {
		t.Fatal(err)
	}
}

func TestSSECancel(t *testing.T) {
	c := sseServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "data: hi\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	s, err := c.Apps.Realtime(ctx, "app1")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if ev, err := s.Next(); err != nil || ev.Data != "hi" {
		t.Fatal(ev, err)
	}
	time.AfterFunc(20*time.Millisecond, cancel)
	if _, err := s.Next(); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

// Cancelling during the wait before a reopen ends the stream at once.
func TestSSECancelDuringReopenWait(t *testing.T) {
	old := realtimePace
	realtimePace = time.Hour
	t.Cleanup(func() { realtimePace = old })
	c := sseServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "event: system\ndata: REALTIME_RECONNECT\n\n")
	})
	ctx, cancel := context.WithCancel(context.Background())
	s, err := c.Apps.Realtime(ctx, "app1")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Next(); err != nil {
		t.Fatal(err)
	}
	time.AfterFunc(20*time.Millisecond, cancel)
	start := time.Now()
	if _, err := s.Next(); !errors.Is(err, context.Canceled) || time.Since(start) > 5*time.Second {
		t.Fatal(err)
	}
}

// The client timeout bounds the open until the headers arrive, never the body.
func TestSSETimeoutOnHeadersOnly(t *testing.T) {
	c := sseServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.(http.Flusher).Flush()
		time.Sleep(100 * time.Millisecond)
		_, _ = io.WriteString(w, "data: late\n\n")
	}, WithTimeout(20*time.Millisecond))
	if ev, err := openStream(t, c).Next(); err != nil || ev.Data != "late" {
		t.Fatal(ev, err)
	}
	c = sseServer(t, func(w http.ResponseWriter, r *http.Request) {
		select { // no headers until the client gives up
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}, WithTimeout(20*time.Millisecond))
	_, err := c.Apps.Realtime(context.Background(), "app1")
	if e := asAPIError(t, err); e.Code != CodeTimeout || e.Status != 0 || e.Path != "/v2/apps/app1/realtime" || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}

func TestSSEReconnect(t *testing.T) {
	var n atomic.Int32
	c := sseServer(t, func(w http.ResponseWriter, _ *http.Request) {
		switch n.Add(1) {
		case 1:
			_, _ = io.WriteString(w, "event: status\ndata: "+fullStatus+"\n\n")
			w.(http.Flusher).Flush()
			panic(http.ErrAbortHandler) // abrupt close, as on a deploy
		default:
			_, _ = io.WriteString(w, "event: status\ndata: "+leanStatus+"\n\n")
		}
	})
	evs, err := drain(t, openStream(t, c))
	if err != io.EOF || len(evs) != 2 || n.Load() != 2 {
		t.Fatal(evs, err, n.Load())
	}
	// The merged state survives the reconnect.
	if st := evs[1].Status; st == nil || st.CPU != 13.1 || st.CPULimit != 0.5 || st.Uptime != 1716000000000 {
		t.Fatalf("%+v", st)
	}
}

func TestSSEServerRequestedReconnect(t *testing.T) {
	if shippedPace != 5500*time.Millisecond {
		t.Fatal(shippedPace)
	}
	old := realtimePace
	realtimePace = 80 * time.Millisecond
	t.Cleanup(func() { realtimePace = old })
	var n atomic.Int32
	var gap time.Duration
	var first time.Time
	c := sseServer(t, func(w http.ResponseWriter, _ *http.Request) {
		if n.Add(1) == 1 {
			first = time.Now()
			_, _ = io.WriteString(w, "event: system\ndata: REALTIME_RECONNECT\n\n") // then a clean end
			return
		}
		gap = time.Since(first)
		_, _ = io.WriteString(w, "data: back\n\n")
	})
	evs, err := drain(t, openStream(t, c))
	if err != io.EOF || len(evs) != 2 || evs[1].Data != "back" || n.Load() != 2 {
		t.Fatal(evs, err, n.Load())
	}
	if gap < realtimePace-5*time.Millisecond {
		t.Fatalf("reconnected after %v, the API allows one open per %v", gap, realtimePace)
	}
}

func TestSSEStopsAfterMaxReconnects(t *testing.T) {
	var n atomic.Int32
	c := sseServer(t, func(w http.ResponseWriter, _ *http.Request) {
		n.Add(1)
		w.(http.Flusher).Flush()
		panic(http.ErrAbortHandler)
	})
	_, err := openStream(t, c).Next()
	if e := asAPIError(t, err); e.Code != CodeNetworkError || e.Status != 0 || e.Message == "" || n.Load() != 1+realtimeRetries {
		t.Fatal(err, n.Load())
	}
}

func TestSSENoReconnectAfterDisconnected(t *testing.T) {
	var n atomic.Int32
	c := sseServer(t, func(w http.ResponseWriter, _ *http.Request) {
		n.Add(1)
		_, _ = io.WriteString(w, "event: system\ndata: REALTIME_DISCONNECTED\n\n")
		w.(http.Flusher).Flush()
		panic(http.ErrAbortHandler)
	})
	// The stream ends right after the event: nothing more is read.
	evs, err := drain(t, openStream(t, c))
	if err != io.EOF || len(evs) != 1 || n.Load() != 1 {
		t.Fatal(evs, err, n.Load())
	}
	// A clean end after REALTIME_DISCONNECTED is a normal io.EOF.
	c = sseServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "event: system\ndata: REALTIME_DISCONNECTED\n\n")
	})
	if evs, err := drain(t, openStream(t, c)); err != io.EOF || len(evs) != 1 {
		t.Fatal(evs, err)
	}
}

// A failed reopen after REALTIME_RECONNECT is an error, not a normal io.EOF.
func TestSSEFailedReconnectIsNotEOF(t *testing.T) {
	var n atomic.Int32
	c := sseServer(t, func(w http.ResponseWriter, _ *http.Request) {
		if n.Add(1) == 1 {
			_, _ = io.WriteString(w, "event: system\ndata: REALTIME_RECONNECT\n\n")
			return
		}
		w.WriteHeader(429)
		_, _ = io.WriteString(w, `{"status":"error","code":"REALTIME_MAX_CONNECTIONS"}`)
	})
	evs, err := drain(t, openStream(t, c))
	var e *APIError
	if len(evs) != 1 || errors.Is(err, io.EOF) || !errors.As(err, &e) || e.Code != "REALTIME_MAX_CONNECTIONS" {
		t.Fatal(evs, err)
	}
}

// The reconnect cap counts failures in a row: a stream that keeps delivering
// events keeps being reopened.
func TestSSERetriesResetOnEvents(t *testing.T) {
	var n atomic.Int32
	c := sseServer(t, func(w http.ResponseWriter, _ *http.Request) {
		if n.Add(1) > realtimeRetries+2 {
			_, _ = io.WriteString(w, "data: last\n\n")
			return
		}
		_, _ = io.WriteString(w, "event: logs\ndata: x\n\n")
		w.(http.Flusher).Flush()
		panic(http.ErrAbortHandler)
	})
	evs, err := drain(t, openStream(t, c))
	if err != io.EOF || len(evs) != realtimeRetries+3 || evs[len(evs)-1].Data != "last" {
		t.Fatal(len(evs), err)
	}
}

// The API's system event on every open is not progress: a stream that
// connects and drops keeps counting toward the cap, then fails as
// NETWORK_ERROR.
func TestSSESystemEventsDoNotResetRetries(t *testing.T) {
	var n atomic.Int32
	c := sseServer(t, func(w http.ResponseWriter, _ *http.Request) {
		n.Add(1)
		_, _ = io.WriteString(w, "event: system\ndata: REALTIME_CONNECTING | x\n\n")
		w.(http.Flusher).Flush()
		panic(http.ErrAbortHandler)
	})
	evs, err := drain(t, openStream(t, c))
	var e *APIError
	if !errors.As(err, &e) || e.Code != CodeNetworkError || e.Path != "/v2/apps/app1/realtime" || errors.Unwrap(err) == nil {
		t.Fatal(err)
	}
	if n.Load() != 1+realtimeRetries || len(evs) != 1+realtimeRetries {
		t.Fatal(n.Load(), len(evs))
	}
}

// The first open retries a network error like any GET: plain backoff, the
// 5.5s pace applies only before a reopen.
func TestSSEOpenRetriesNetworkError(t *testing.T) {
	old := realtimePace
	realtimePace = time.Hour
	t.Cleanup(func() { realtimePace = old })
	start := time.Now()
	var n atomic.Int32
	c := sseServer(t, func(w http.ResponseWriter, _ *http.Request) {
		if n.Add(1) == 1 {
			hangUp(w)
			return
		}
		_, _ = io.WriteString(w, "data: ok\n\n")
	})
	if ev, err := openStream(t, c).Next(); err != nil || ev.Data != "ok" || n.Load() != 2 {
		t.Fatal(ev, err, n.Load())
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("paced the open retry (%v)", d)
	}
	// A ctx that ends during the open's backoff is detectable with errors.Is.
	oldBase := retryBase
	retryBase = time.Hour
	t.Cleanup(func() { retryBase = oldBase })
	c = sseServer(t, func(w http.ResponseWriter, _ *http.Request) { hangUp(w) })
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(20*time.Millisecond, cancel)
	_, err := c.Apps.Realtime(ctx, "app1")
	if e := asAPIError(t, err); !errors.Is(err, context.Canceled) || e.Path != "/v2/apps/app1/realtime" {
		t.Fatal(err)
	}
}

// Endless REALTIME_RECONNECT requests hit the same cap and never read as io.EOF.
func TestSSERepeatedReconnectRequestsAreCapped(t *testing.T) {
	var n atomic.Int32
	c := sseServer(t, func(w http.ResponseWriter, _ *http.Request) {
		if n.Add(1) == 1 {
			_, _ = io.WriteString(w, "id: 7\n")
		}
		_, _ = io.WriteString(w, "event: system\ndata: REALTIME_RECONNECT\n\n")
	})
	evs, err := drain(t, openStream(t, c))
	if e := asAPIError(t, err); e.Code != CodeNetworkError || e.Message != "REALTIME_RECONNECT" || errors.Is(err, io.EOF) || n.Load() != 1+realtimeRetries {
		t.Fatal(err, n.Load())
	}
	// The last event id is per connection.
	if evs[0].ID != "7" || evs[1].ID != "" {
		t.Fatal(evs[0].ID, evs[1].ID)
	}
}

// ---- Multipart uploads ----

type gotUpload struct {
	contentLength  int64
	chunked        bool
	filename, file string
	query, ctype   string
	partType       string
	bodyLen        int
}

func uploadServer(t *testing.T, got *gotUpload) *Client {
	return testClient(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		*got = gotUpload{
			contentLength: r.ContentLength,
			chunked:       len(r.TransferEncoding) > 0 && r.TransferEncoding[0] == "chunked",
			query:         r.URL.RawQuery,
			ctype:         r.Header.Get("Content-Type"),
			bodyLen:       len(raw),
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		mr, err := r.MultipartReader()
		if err != nil {
			t.Error(err)
			return
		}
		p, err := mr.NextPart()
		if err != nil {
			t.Error(err)
			return
		}
		b, _ := io.ReadAll(p)
		got.filename, got.file, got.partType = p.FileName(), string(b), p.Header.Get("Content-Type")
		if p.FormName() != "file" {
			t.Errorf("part %q", p.FormName())
		}
		if _, err := mr.NextPart(); err != io.EOF {
			t.Errorf("want exactly one part, got %v", err)
		}
		_, _ = io.WriteString(w, `{"status":"success","response":{"id":"a1","cpu":1.5}}`)
	})
}

func TestUploadSeekableHasContentLength(t *testing.T) {
	var got gotUpload
	c := uploadServer(t, &got)
	zip := strings.Repeat("PK\x03\x04", 1000)
	r := strings.NewReader("skip" + zip)
	_, _ = r.Seek(4, io.SeekStart) // uploads start from the current offset
	app, err := c.Apps.Create(context.Background(), r)
	if err != nil || app.CPU != 1.5 {
		t.Fatal(app, err)
	}
	if got.file != zip || got.filename != "app.zip" || got.chunked || got.contentLength != int64(got.bodyLen) || got.partType != "application/octet-stream" {
		t.Fatalf("%+v", got)
	}
	if !strings.HasPrefix(got.ctype, "multipart/form-data; boundary=") {
		t.Fatal(got.ctype)
	}
}

func TestUploadUnknownSizeIsChunked(t *testing.T) {
	var got gotUpload
	c := uploadServer(t, &got)
	if err := c.Apps.Commit(context.Background(), "app1", io.MultiReader(strings.NewReader("ZIP")), "src/lib", ""); err != nil {
		t.Fatal(err)
	}
	if got.file != "ZIP" || got.filename != "commit.zip" || !got.chunked || got.query != "path=src%2Flib" {
		t.Fatalf("%+v", got)
	}
}

// The SDK must send the first bytes before the reader is exhausted, which is
// impossible if the zip is buffered in memory first.
func TestUploadStreams(t *testing.T) {
	firstChunk := make(chan struct{})
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		mr, err := r.MultipartReader()
		if err != nil {
			t.Error(err)
			return
		}
		p, _ := mr.NextPart()
		buf := make([]byte, 5)
		if _, err := io.ReadFull(p, buf); err != nil || string(buf) != "first" {
			t.Errorf("%q %v", buf, err)
		}
		close(firstChunk)
		rest, _ := io.ReadAll(p)
		if string(rest) != "second" {
			t.Errorf("%q", rest)
		}
		_, _ = io.WriteString(w, `{"status":"success"}`)
	})
	pr, pw := io.Pipe()
	go func() {
		_, _ = pw.Write([]byte("first"))
		select {
		case <-firstChunk:
			_, _ = pw.Write([]byte("second"))
			_ = pw.Close()
		case <-time.After(5 * time.Second):
			_ = pw.CloseWithError(errors.New("upload was buffered: server never saw the first chunk"))
		}
	}()
	if err := c.Apps.Commit(context.Background(), "app1", pr, "", ""); err != nil {
		t.Fatal(err)
	}
}

// hugeZip reports a size above the API cap without allocating it.
type hugeZip struct{ off int64 }

func (h *hugeZip) Read(p []byte) (int, error) { return len(p), nil }
func (h *hugeZip) Seek(off int64, whence int) (int64, error) {
	switch whence {
	case io.SeekEnd:
		h.off = maxUploadSize + 1
	case io.SeekStart:
		h.off = off
	}
	return h.off, nil
}

func TestUploadRejectsOversizedZip(t *testing.T) {
	called := false
	c := testClient(t, func(http.ResponseWriter, *http.Request) { called = true })
	if _, err := c.Apps.Create(context.Background(), &hugeZip{}); err == nil || called {
		t.Fatal("want a local error before any request", err)
	}
	if _, err := c.Apps.Create(context.Background(), nil); err == nil || called {
		t.Fatal("want nil reader rejected", err)
	}
	// f, _ := os.Open(missing) gives a nil *os.File: an error, not a panic.
	var f *os.File
	if err := c.Apps.Commit(context.Background(), "app1", f, "", ""); err == nil || called {
		t.Fatal("want nil *os.File rejected", err)
	}
}

// A stream of unknown size is cut once it passes the cap, not sent to the end.
func TestUploadCapsUnknownSize(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = io.WriteString(w, `{"status":"success"}`)
	})
	err := c.Apps.Commit(context.Background(), "app1", io.LimitReader(&hugeZip{}, maxUploadSize+1), "", "")
	if e := asAPIError(t, err); e.Code != CodeFileTooLarge || e.Path != "/v2/apps/app1/commit" {
		t.Fatal(err)
	}
}

// Commit names the part: explicit filename, else the file's own name, else
// commit.zip. Quotes and line breaks cannot escape the header.
func TestCommitFilename(t *testing.T) {
	var got gotUpload
	c := uploadServer(t, &got)
	f, err := os.Create(filepath.Join(t.TempDir(), "main.py"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, _ = f.WriteString("print(1)")
	for _, tc := range []struct{ filename, want string }{
		{"", "main.py"},
		{"app.py", "app.py"},
		{"a\"b\r\nc.py", "a%22bc.py"},
	} {
		_, _ = f.Seek(0, io.SeekStart)
		if err := c.Apps.Commit(context.Background(), "app1", f, "src", tc.filename); err != nil {
			t.Fatal(err)
		}
		if got.filename != tc.want || got.file != "print(1)" || got.chunked || got.query != "path=src" {
			t.Fatalf("%q: %+v", tc.filename, got)
		}
	}
}

// ---- Snapshot download ----

func TestDownloadSnapshot(t *testing.T) {
	var auth, accept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, accept = r.Header.Get("Authorization"), r.Header.Get("Accept")
		if r.URL.Path == "/missing.zip" {
			w.WriteHeader(403)
			_, _ = io.WriteString(w, "<Error><Code>AccessDenied</Code></Error>")
			return
		}
		_, _ = io.WriteString(w, strings.Repeat("z", 1<<20))
	}))
	defer srv.Close()
	c := New("secret-key")
	var buf bytes.Buffer
	var flaky atomic.Int32
	flakySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if flaky.Add(1) == 1 {
			hangUp(w) // a network error before the response is retried
			return
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer flakySrv.Close()
	if err := c.DownloadSnapshot(context.Background(), flakySrv.URL+"/s.zip", &buf); err != nil || buf.String() != "ok" || flaky.Load() != 2 {
		t.Fatal(err, buf.String(), flaky.Load())
	}
	buf.Reset()
	err := c.DownloadSnapshot(context.Background(), srv.URL+"/s.zip?Signature=x", &buf)
	if err != nil || buf.Len() != 1<<20 || auth != "" || accept != "" {
		t.Fatal(buf.Len(), err, auth, accept)
	}
	err = c.DownloadSnapshot(context.Background(), srv.URL+"/missing.zip", io.Discard)
	if e := asAPIError(t, err); e.Status != 403 || e.Code != CodeUnknown || e.Message != "HTTP 403" || e.Path != "/missing.zip" {
		t.Fatal(err)
	}
	// A transport failure must not print the presigned URL (a bearer link).
	dead := httptest.NewServer(http.NotFoundHandler())
	dead.Close()
	err = c.DownloadSnapshot(context.Background(), dead.URL+"/s.zip?Signature=secret", io.Discard)
	if e := asAPIError(t, err); e.Code != CodeNetworkError || strings.Contains(err.Error(), "secret") {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	hang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer hang.Close()
	if e := asAPIError(t, c.DownloadSnapshot(ctx, hang.URL+"/s.zip?Signature=secret", io.Discard)); e.Code != CodeTimeout {
		t.Fatal(e)
	}
	// Caller-side problems stay plain errors: a bad URL and a failing writer.
	if err := c.DownloadSnapshot(context.Background(), "::bad", io.Discard); err == nil || errors.As(err, new(*APIError)) {
		t.Fatal(err)
	}
	werr := errors.New("disk full")
	if err := c.DownloadSnapshot(context.Background(), srv.URL+"/s.zip", failWriter{werr}); err != werr {
		t.Fatal(err)
	}
}

// The download is streamed: w gets the first chunk before the server ends.
func TestDownloadSnapshotStreams(t *testing.T) {
	got := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "PK first")
		w.(http.Flusher).Flush()
		select {
		case <-got:
		case <-time.After(5 * time.Second):
			t.Error("the first chunk never reached the writer while the body was open")
		}
		_, _ = io.WriteString(w, " rest")
	}))
	defer srv.Close()
	var buf bytes.Buffer
	w := writerFunc(func(p []byte) (int, error) {
		if buf.Len() == 0 {
			close(got)
		}
		return buf.Write(p)
	})
	if err := New("k").DownloadSnapshot(context.Background(), srv.URL+"/s.zip", w); err != nil || buf.String() != "PK first rest" {
		t.Fatal(err, buf.String())
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

type failWriter struct{ err error }

func (f failWriter) Write([]byte) (int, error) { return 0, f.err }
