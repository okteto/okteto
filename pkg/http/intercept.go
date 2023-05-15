package http

import (
	"net"
	"net/url"
)

type Intercept map[string]struct{}

func (i Intercept) ShouldInterceptAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	_, ok := i[host]
	return ok
}

func (i Intercept) AppendURLs(urls ...string) {
	for _, u := range urls {
		up, err := url.Parse(u)
		if err != nil {
			continue
		}
		if up.Hostname() == "" {
			continue
		}
		i[up.Hostname()] = struct{}{}
	}
}
