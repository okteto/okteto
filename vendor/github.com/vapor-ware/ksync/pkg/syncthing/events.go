package syncthing

import (
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/syncthing/syncthing/lib/events"
)

// Events returns a stream of events being issued from the syncthing server. It
// is not filtered and is up to the listener to choose which events they're
// interested in. Remember to call Server.Stop() to stop listening for events.
func (s *Server) Events() (<-chan events.Event, error) {
	out := make(chan events.Event)

	// TODO: this will replay all the events for new shares (potentially a ton
	// for long running watch processes). Should only get the latest.
	since := 0

	params := map[string]string{
		"since": strconv.Itoa(since),
	}

	go func() {
		for {
			select {
			case <-s.stop:
				log.WithFields(s.Fields()).Debug("halting events polling")
				close(out)
				return
			default:
				params["since"] = strconv.Itoa(since)
				resp, err := s.client.NewRequest().
					SetQueryParams(params).
					SetResult([]events.Event{}).
					Get("events")

				if err != nil {
					log.Warn(err)
					continue
				}

				for _, event := range *resp.Result().(*[]events.Event) {
					since = event.SubscriptionID
					out <- event
				}
			}
		}
	}()

	return out, nil
}
