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

// RealtimeEvent is one server-sent event of Apps.Realtime.
//
//   - "system": Data is a code such as "REALTIME_CONNECTING | <sseId>",
//     REALTIME_TIMEOUT, REALTIME_DISCONNECTED, REALTIME_RECONNECT or
//     REALTIME_ERROR.
//   - "logs": Line is the log line (stream-id byte removed), Stream is
//     "stdout" or "stderr".
//   - "status": Status is never nil: the merged container state (each
//     frame's top-level keys replace the previous values, across reconnects;
//     a malformed frame leaves it unchanged). Data is the raw JSON frame.
//   - "error": Data is a code such as CONTAINER_NOT_FOUND.
//   - "message": a frame without an event field.
//
// Data is always the raw event data and ID the last event id ("" if none).
type RealtimeEvent struct {
	Event  string
	Data   string
	ID     string
	Stream string // logs only
	Line   string // logs only
	Status *RealtimeStatus
}

// RealtimeStatus is the merged "status" frame. CPULimit is a number of cores
// (e.g. 1 or 0.5); RAM is [usedMB, limitMB]; Uptime is the start time in
// Unix ms; NetIO.New is bytes per second.
type RealtimeStatus struct {
	CPU      float64    `json:"cpu"`
	CPULimit float64    `json:"cpuLimit"`
	RAM      [2]float64 `json:"ram"`
	Status   string     `json:"status"`
	NetIO    NetIO      `json:"netIO"`
	BlockIO  IOCounter  `json:"bIO"`
	Uptime   int64      `json:"uptime"`
}

// IOCounter is a pair of input and output byte counters.
type IOCounter struct {
	In  float64 `json:"i"`
	Out float64 `json:"o"`
}

// NetIO is the network I/O of RealtimeStatus: total bytes and, in New, bytes
// per second.
type NetIO struct {
	In  float64   `json:"i"`
	Out float64   `json:"o"`
	New IOCounter `json:"new"`
}

// Realtime reads Apps.Realtime. Call Next until it returns an error (io.EOF
// when the server ends the stream, e.g. after the 10-minute TTL or right
// after the REALTIME_DISCONNECTED event) and always
// Close. A dropped connection, or a server-requested REALTIME_RECONNECT, is
// reopened (waiting out the API's pace of one open per 5s, and the retry
// backoff) until 3 reopens in a row deliver no logs or status event; Next
// then returns an *APIError with CodeNetworkError wrapping the last read
// error. Cancel ctx to stop.
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

// Realtime opens the SSE feed of an application (5 per account, 30 per app).
// The HTTP status is checked first, so 429 REALTIME_MAX_CONNECTIONS is
// returned here as *APIError. The open is bounded by the client timeout until
// the response headers arrive; the stream itself has no deadline besides ctx.
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
		e := s.c.localError(http.MethodGet, s.path, CodeTimeout, "no response headers within "+s.c.timeout.String())
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

// Close releases the connection. To interrupt a blocked Next from another
// goroutine, cancel the ctx passed to Realtime instead.
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

// Next blocks until the next event. After REALTIME_DISCONNECTED or Close it
// returns io.EOF.
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
