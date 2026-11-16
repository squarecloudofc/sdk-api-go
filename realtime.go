package squarecloud

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

// realtimePace is the API's limit of one realtime open per 5s per (user,
// app), plus 0.5s for clock skew; reopens wait it out. A variable only so
// tests can shrink it.
var realtimePace = 5500 * time.Millisecond

// RealtimeEvent is one event of a [Realtime] stream. Event names its kind,
// and the kind decides which other fields are set:
//
//   - "logs" is a line of output. Line holds the text, and Stream is
//     "stdout" or "stderr".
//   - "status" is the application's resource usage. Status holds the full
//     state. Each status event can carry only some of the values, so the SDK
//     keeps the others from earlier events, across reconnects too. A
//     malformed status event leaves the state unchanged.
//   - "system" is a notice from the server. Data is a code such as
//     REALTIME_CONNECTING, REALTIME_TIMEOUT, REALTIME_DISCONNECTED,
//     REALTIME_RECONNECT or REALTIME_ERROR.
//   - "error" is a problem with the application. Data is a code such as
//     CONTAINER_NOT_FOUND.
//   - "message" is an event the server sent without a kind.
type RealtimeEvent struct {
	// Event is the kind of event, such as "logs" or "status".
	Event string
	// Data is the text of the event as the server sent it.
	Data string
	// ID is the id of the latest event on the current connection that carried
	// one, or "" when none did.
	ID string
	// Stream is "stdout" or "stderr" on a logs event, and "" otherwise.
	Stream string
	// Line is the log text on a logs event, and "" otherwise.
	Line string
	// Status is the full resource usage on a status event, where it is never
	// nil. It is nil on every other event.
	Status *RealtimeStatus
}

// RealtimeStatus is the resource usage that a status [RealtimeEvent]
// reports.
type RealtimeStatus struct {
	// CPU is the CPU usage in percent.
	CPU float64 `json:"cpu"`
	// CPULimit is the number of CPU cores allocated, such as 1 or 0.5.
	CPULimit float64 `json:"cpuLimit"`
	// RAM holds the memory in use and the memory limit, both in MB.
	RAM [2]float64 `json:"ram"`
	// Status is the state of the application, such as "running".
	Status string `json:"status"`
	// NetIO is the network traffic.
	NetIO NetIO `json:"netIO"`
	// BlockIO is the disk traffic, in bytes read and written.
	BlockIO IOCounter `json:"bIO"`
	// Uptime is when the application started, in Unix milliseconds.
	Uptime int64 `json:"uptime"`
}

// IOCounter is a pair of byte counts, one for each direction.
type IOCounter struct {
	In  float64 `json:"i"`
	Out float64 `json:"o"`
}

// NetIO is the network traffic of a [RealtimeStatus]. In and Out are the
// total bytes received and sent, and New is the current rate in bytes per
// second.
type NetIO struct {
	In  float64   `json:"i"`
	Out float64   `json:"o"`
	New IOCounter `json:"new"`
}

// Realtime is a live stream of an application's logs and resource usage,
// opened with [AppsAPI.Realtime]. Call [Realtime.Next] in a loop until it
// returns an error, and always call [Realtime.Close] when you are done.
//
// Next returns [io.EOF] when the stream ends normally, which happens after
// the server's 10-minute limit or right after a REALTIME_DISCONNECTED event.
// When the connection drops, or the server sends REALTIME_RECONNECT, the
// stream reopens by itself. The API allows one open per application every 5
// seconds, so a reopen waits until at least 5.5 seconds have passed since the
// previous one. After 3 reopens in a row without a logs or status event, Next
// returns an [*APIError] with code [CodeNetworkError] that wraps the last
// error.
//
// Cancel the context passed to [AppsAPI.Realtime] to stop the stream at any
// time. Next then returns the context's error, such as [context.Canceled].
type Realtime struct {
	ctx       context.Context
	cancel    context.CancelFunc // ends the current connection
	c         *Client
	path      string
	body      io.ReadCloser
	r         *bufio.Reader
	status    RealtimeStatus
	lastID    string
	opened    time.Time
	retries   int
	reconnect bool // REALTIME_RECONNECT seen: reopen after the clean EOF
	done      bool // REALTIME_DISCONNECTED seen or Close called: never reopen
}

// Realtime opens a live stream of the application's logs and resource usage.
// See [Realtime] for how to read it, and [AppsAPI] for workspace
// applications.
//
// When the stream cannot open, Realtime returns the error before any event.
// You can have at most 5 streams open per account and 30 per application.
// Past those limits it returns 429 REALTIME_MAX_CONNECTIONS or 429
// REALTIME_MAX_CONNECTIONS_APP. The client timeout applies only until the
// stream opens. After that, only ctx limits it.
func (a AppsAPI) Realtime(ctx context.Context, appID string) (*Realtime, error) {
	s := &Realtime{ctx: ctx, c: a.c, path: appPath(appID, "/realtime")}
	if err := s.connect(); err != nil {
		return nil, err
	}
	return s, nil
}

// connect opens the stream, retrying like any GET. The merged status
// survives, so a reopen continues from it; the last event id is per
// connection.
func (s *Realtime) connect() error {
	s.lastID = ""
	for attempt := 0; ; attempt++ {
		err := s.open()
		var e *APIError
		if err == nil || !errors.As(err, &e) || attempt >= s.c.maxRetries || !retryable(http.MethodGet, e) {
			return err
		}
		if !wait(s.ctx, backoff(attempt)) {
			return ctxError(s.ctx, http.MethodGet, e.Path)
		}
	}
}

// open makes one attempt, bounded by the client timeout until the headers.
func (s *Realtime) open() error {
	s.opened = time.Now()
	ctx, cancel := context.WithCancel(s.ctx)
	var timer *time.Timer
	if _, ok := s.ctx.Deadline(); !ok && s.c.timeout > 0 {
		timer = time.AfterFunc(s.c.timeout, cancel)
	}
	req, err := s.c.newRequest(ctx, http.MethodGet, s.path, nil, "")
	var res *http.Response
	if err == nil {
		req.Header.Set("Accept", "text/event-stream")
		res, err = s.c.send(req)
	}
	if timer != nil && !timer.Stop() && s.ctx.Err() == nil {
		// The timeout fired before the headers (or together with them).
		cancel()
		if err == nil {
			_ = res.Body.Close()
		}
		e := s.c.localError(http.MethodGet, s.path, CodeTimeout, "The realtime stream did not open within "+s.c.timeout.String()+". Try again, or allow more time with WithTimeout.")
		e.err = context.DeadlineExceeded
		return e
	}
	if err != nil {
		cancel()
		return err
	}
	s.body, s.r, s.cancel = res.Body, bufio.NewReader(res.Body), cancel
	return nil
}

// Close ends the stream and releases its connection. To interrupt a Next call
// that is waiting in another goroutine, cancel the context passed to
// [AppsAPI.Realtime] instead.
func (s *Realtime) Close() error {
	s.done = true
	s.cancel()
	return s.body.Close()
}

// reopen waits max(backoff, the API's open pace), then reconnects.
func (s *Realtime) reopen() error {
	s.retries++
	s.reconnect = false
	s.cancel()
	_ = s.body.Close()
	if !wait(s.ctx, max(backoff(s.retries-1), realtimePace-time.Since(s.opened))) {
		return s.ctx.Err()
	}
	return s.connect()
}

// Next waits for the next event and returns it. It returns [io.EOF] when the
// server ends the stream, after a REALTIME_DISCONNECTED event, and after
// [Realtime.Close]. See [Realtime] for reconnects and the other errors.
func (s *Realtime) Next() (RealtimeEvent, error) {
	if s.done {
		return RealtimeEvent{}, io.EOF
	}
	var (
		typ     string
		data    strings.Builder
		hasData bool
	)
	for {
		line, err := s.r.ReadString('\n')
		if err != nil {
			// An unterminated event at EOF is discarded, as the SSE spec says.
			if ctxErr := s.ctx.Err(); ctxErr != nil {
				return RealtimeEvent{}, ctxErr
			}
			if s.done || (err == io.EOF && !s.reconnect) {
				return RealtimeEvent{}, err
			}
			if s.retries >= realtimeRetries {
				switch {
				case err == io.EOF && s.reconnect:
					err = errors.New("REALTIME_RECONNECT") // the last drop was a server handoff
				case err == io.EOF:
					err = io.ErrUnexpectedEOF // errors.Is(err, io.EOF) means a normal end
				}
				e := s.c.localError(http.MethodGet, s.path, CodeNetworkError, err.Error())
				e.err = err
				return RealtimeEvent{}, e
			}
			if err := s.reopen(); err != nil {
				return RealtimeEvent{}, err // not a normal end, even after a clean EOF
			}
			typ, hasData = "", false
			data.Reset()
			continue
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if line == "" {
			if hasData {
				if typ == "logs" || typ == "status" {
					// The cap is on reopens in a row without real data: the
					// API sends a system event on every open.
					s.retries = 0
				}
				return s.dispatch(typ, data.String()), nil
			}
			typ = ""
			continue
		}
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ") // exactly one space
		switch field {
		case "event":
			typ = value
		case "data":
			if hasData {
				data.WriteByte('\n')
			}
			data.WriteString(value)
			hasData = true
		case "id":
			if !strings.ContainsRune(value, 0) {
				s.lastID = value
			}
		}
		// "" (comment), "retry" and unknown fields are ignored.
	}
}

func (s *Realtime) dispatch(typ, data string) RealtimeEvent {
	if typ == "" {
		typ = "message"
	}
	ev := RealtimeEvent{Event: typ, Data: data, ID: s.lastID}
	switch typ {
	case "logs":
		ev.Stream, ev.Line = "stdout", data // stdout also for lines without the stream-id byte
		if data != "" && (data[0] == 1 || data[0] == 2) {
			if data[0] == 2 {
				ev.Stream = "stderr"
			}
			ev.Line = data[1:]
		}
	case "status":
		s.merge([]byte(data))
		st := s.status
		ev.Status = &st
	case "system":
		switch {
		case strings.HasPrefix(data, "REALTIME_DISCONNECTED"):
			s.done = true
		case strings.HasPrefix(data, "REALTIME_RECONNECT"):
			s.reconnect = true
		}
	}
	return ev
}

// merge shallow-merges a status frame onto the state (lean frames only carry
// cpu/ram/netIO/bIO): each top-level key present replaces that field whole,
// as in the JS and Python SDKs. A malformed frame is dropped.
func (s *Realtime) merge(data []byte) {
	var (
		keys map[string]json.RawMessage
		f    RealtimeStatus
	)
	if json.Unmarshal(data, &keys) != nil || json.Unmarshal(data, &f) != nil {
		return
	}
	for k := range keys {
		switch k {
		case "cpu":
			s.status.CPU = f.CPU
		case "cpuLimit":
			s.status.CPULimit = f.CPULimit
		case "ram":
			s.status.RAM = f.RAM
		case "status":
			s.status.Status = f.Status
		case "netIO":
			s.status.NetIO = f.NetIO
		case "bIO":
			s.status.BlockIO = f.BlockIO
		case "uptime":
			s.status.Uptime = f.Uptime
		}
	}
}
