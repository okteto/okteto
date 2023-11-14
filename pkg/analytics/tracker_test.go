package analytics

type mockEvent struct {
	props   map[string]any
	event   string
	success bool
}
