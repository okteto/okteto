package analytics

type AnalyticsTracker struct {
	trackFn func(event string, success bool, props map[string]interface{})
}

func NewAnalyticsTracker() *AnalyticsTracker {
	return &AnalyticsTracker{
		trackFn: track,
	}
}
