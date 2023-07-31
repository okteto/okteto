package analytics

type AnalyticsTracker struct {
	TrackFn func(event string, success bool, props map[string]interface{})
}

func NewAnalyticsTracker() *AnalyticsTracker {
	return &AnalyticsTracker{
		TrackFn: track,
	}
}
