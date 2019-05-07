package analytics

import (
	"net"
	"net/http"
	"runtime"
	"time"

	"github.com/dukex/mixpanel"
	"github.com/okteto/app/cli/pkg/log"
	"github.com/okteto/app/cli/pkg/okteto"
)

const (
	mixpanelToken = "92fe782cdffa212d8f03861fbf1ea301"

	createDatabaseEvent = "Create Database"
	upEvent             = "Up"
	loginEvent          = "Login"
	createEvent         = "Create Manifest"
)

var mixpanelClient mixpanel.Mixpanel

func init() {
	c := &http.Client{
		Timeout: time.Second * 5,
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 5 * time.Second,
		},
	}

	mixpanelClient = mixpanel.NewFromClient(c, mixpanelToken, "")
}

// TrackCreate sends a tracking event to mixpanel when the user creates a manifest
func TrackCreate(language, image, version string) {
	if err := mixpanelClient.Track(okteto.GetUserID(), createEvent, &mixpanel.Event{
		Properties: map[string]interface{}{
			"image":    image,
			"language": language,
			"os":       runtime.GOOS,
			"version":  version,
		},
	}); err != nil {
		log.Errorf("Failed to send analytics: %s", err)
	}
}

// TrackUp sends a tracking event to mixpanel when the user starts a development environment
func TrackUp(image, version string) {
	if err := mixpanelClient.Track(okteto.GetUserID(), upEvent, &mixpanel.Event{
		Properties: map[string]interface{}{
			"image":   image,
			"os":      runtime.GOOS,
			"version": version,
		},
	}); err != nil {
		log.Errorf("Failed to send analytics: %s", err)
	}
}

// TrackLogin sends a tracking event to mixpanel when the user logs in
func TrackLogin(version string) {
	if err := mixpanelClient.Track(okteto.GetUserID(), loginEvent, &mixpanel.Event{
		Properties: map[string]interface{}{
			"os":      runtime.GOOS,
			"version": version,
		},
	}); err != nil {
		log.Errorf("Failed to send analytics: %s", err)
	}
}

// TrackCreateDatabase sends a tracking event to mixpanel when the user creates a database
func TrackCreateDatabase(database, version string) {
	if err := mixpanelClient.Track(okteto.GetUserID(), createDatabaseEvent, &mixpanel.Event{
		Properties: map[string]interface{}{
			"database": database,
			"os":       runtime.GOOS,
			"version":  version,
		},
	}); err != nil {
		log.Errorf("Failed to send analytics: %s", err)
	}
}
