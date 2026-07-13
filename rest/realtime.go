package rest

import (
	"bufio"
	"io"
	"strings"

	"github.com/squarecloudofc/sdk-api-go/v2/squarecloud"
)

// RealtimeStream reads server-sent events from /apps/{id}/realtime. Iterate
// with Next until io.EOF (or context cancellation) and always Close.
type RealtimeStream struct {
	body    io.ReadCloser
	scanner *bufio.Scanner
}

func newRealtimeStream(body io.ReadCloser) *RealtimeStream {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	return &RealtimeStream{body: body, scanner: scanner}
}

// Next blocks until the next complete SSE event arrives. It returns io.EOF
// when the server closes the stream (the connection has a 10-minute TTL —
// reconnect by calling ApplicationRealtime again).
func (r *RealtimeStream) Next() (squarecloud.RealtimeEvent, error) {
	var ev squarecloud.RealtimeEvent

	for r.scanner.Scan() {
		line := strings.TrimSuffix(r.scanner.Text(), "\r")

		switch {
		case strings.HasPrefix(line, "event:"):
			ev.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			if ev.Data != "" {
				ev.Data += "\n"
			}
			ev.Data += strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		case line == "":
			if ev.Event != "" || ev.Data != "" {
				return ev, nil
			}
		}
	}

	if err := r.scanner.Err(); err != nil {
		return ev, err
	}

	return ev, io.EOF
}

func (r *RealtimeStream) Close() error {
	return r.body.Close()
}
