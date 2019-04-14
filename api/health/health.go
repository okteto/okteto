package health

import (
	"net/http"

	"github.com/okteto/app/api/log"
)

var ok = []byte(`{'status': 'ok'}`)

func isHealthy() bool {
	return true
}

// HealthcheckHandler handles the healthchecks
func HealthcheckHandler() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if isHealthy() {
			w.Write(ok)
			return
		}

		log.Errorf("system is unhealthy")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	return http.HandlerFunc(fn)
}
