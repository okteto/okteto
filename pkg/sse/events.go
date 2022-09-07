package sse

import "strings"

const (
	// dataHeader is the sse string header for sse-data
	dataHeader = "data: "

	// pingEvent is the type of event ping
	pingEvent = "ping"
)

// getMessage returns the parsed message from the sse-data
func getMessage(s string) string {
	if strings.HasPrefix(s, dataHeader) && !strings.Contains(s, pingEvent) {
		return strings.TrimPrefix(s, dataHeader)
	}
	return ""
}

// isDone returns bool if the event msg is EOF
func isDone(msg string) bool {
	return strings.Contains(msg, "EOF")
}
