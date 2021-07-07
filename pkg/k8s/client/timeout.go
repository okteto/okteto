package client

import (
	"os"
	"sync"
	"time"

	"github.com/okteto/okteto/pkg/log"
)

var timeout time.Duration
var tOnce sync.Once

func getKubernetesTimeout() time.Duration {
	tOnce.Do(func() {
		timeout = 30 * time.Second
		t, ok := os.LookupEnv("OKTETO_KUBERNETES_TIMEOUT")
		if !ok {
			return
		}

		parsed, err := time.ParseDuration(t)
		if err != nil {
			log.Infof("'%s' is not a valid duration, ignoring", t)
			return
		}

		log.Infof("OKTETO_KUBERNETES_TIMEOUT applied: '%s'", parsed.String())
		timeout = parsed
	})

	return timeout
}
