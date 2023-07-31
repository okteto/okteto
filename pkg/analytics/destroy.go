package analytics

// DestroyMetadata contains the metadata of a destroy event
type DestroyMetadata struct {
	Success      bool
	IsDestroyAll bool
	IsRemote     bool
}

// TrackDestroy sends a tracking event to mixpanel when the user destroys a pipeline from local
func (a *AnalyticsTracker) TrackDestroy(metadata DestroyMetadata) {
	props := map[string]any{
		"isDestroyAll": metadata.IsDestroyAll,
		"isRemote":     metadata.IsRemote,
	}
	a.TrackFn(destroyEvent, metadata.Success, props)
}
