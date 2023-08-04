package analytics

type mockEvent struct {
	event   string
	success bool
	props   map[string]any
}
