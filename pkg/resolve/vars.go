package resolve

import (
	"net"
	"net/http"
	"time"
)

const (
	// LatestVersionAlias is an alias for the latest (unknown) version.
	// When used for example in Pull it will automatically resolve the latest
	// known version
	LatestVersionAlias = "latest"
)

var (
	defaultChannel   = "stable"
	defaultPullHost  = "https://downloads.okteto.com"
	defaultTransport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 30 * time.Second,
	}
)
