package squarecloud

// RealtimeEvent is one server-sent event of GET /apps/{id}/realtime.
// System-level events use Event names like REALTIME_CONNECTING,
// REALTIME_TIMEOUT, REALTIME_DISCONNECTED, REALTIME_RECONNECT and
// REALTIME_ERROR; regular frames arrive as "message" with an opaque Data
// payload (typically JSON).
//
// Connection limits: at most 5 simultaneous connections per user; each
// connection has a 10-minute TTL — clients must reconnect when it expires.
type RealtimeEvent struct {
	Event string
	Data  string
}
