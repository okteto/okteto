// A three-hop demo service for `okteto divert`.
//
// One binary plays every role in the chain. Each instance reports who it is and which
// namespace it is running in, then calls its upstream and appends the answer:
//
//	frontend(staging) -> api(staging) -> catalog(staging)
//
// The whole point of the demo is the middle hop changing namespace when a routing header is
// present, which is only possible because each instance forwards the `baggage` header on
// its own outbound call. No tool can do that for you: an application that drops the header
// cannot be diverted past its first hop.
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	upstreamTimeout   = 5 * time.Second
	readHeaderTimeout = 5 * time.Second
)

func main() {
	name := envOr("NAME", "unknown")
	namespace := envOr("NAMESPACE", "unknown")
	upstream := os.Getenv("UPSTREAM")
	port := envOr("PORT", "8080")

	client := newClient(os.Getenv("NEW_CONNECTION_PER_REQUEST") == "true")

	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		hop := fmt.Sprintf("%s(%s)", name, namespace)

		if upstream == "" {
			fmt.Fprint(w, hop)
			return
		}

		next, err := call(client, upstream, r.Header.Get("baggage"))
		if err != nil {
			fmt.Fprintf(w, "%s -> ERROR(%s)", hop, err)
			return
		}

		fmt.Fprintf(w, "%s -> %s", hop, next)
	})

	log.Printf("%s listening on :%s (upstream: %q)", hop(name, namespace), port, upstream)
	server := &http.Server{
		Addr:              ":" + port,
		ReadHeaderTimeout: readHeaderTimeout,
	}
	log.Fatal(server.ListenAndServe())
}

// newClient builds the client used for every outbound call.
//
// The default is a normal, realistic Go client: one instance shared by every request, no
// Transport of its own, so it uses http.DefaultTransport — which keeps connections alive
// and pools them (IdleConnTimeout 90s). A new http.Request per incoming request does *not*
// mean a new TCP connection.
//
// That matters for divert. kube-proxy picks a backend when a connection is established, not
// per request, so a pooled connection opened before `okteto divert up` stays pinned to the
// pod it was already talking to and keeps reaching the baseline. It is the single most
// common reason a working divert looks intermittent.
//
// Setting NEW_CONNECTION_PER_REQUEST=true turns pooling off, so the divert takes effect as
// soon as the endpoints propagate, with no restart. Run the demo both ways: the difference
// between them is the behaviour, not a bug.
func newClient(newConnectionPerRequest bool) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableKeepAlives = newConnectionPerRequest

	return &http.Client{
		Timeout:   upstreamTimeout,
		Transport: transport,
	}
}

// call forwards the request to the upstream, carrying the baggage header with it. This one
// line is the application's whole contribution to making a mid-chain divert work.
func call(client *http.Client, upstream, baggage string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, upstream, nil)
	if err != nil {
		return "", err
	}

	if baggage != "" {
		req.Header.Set("baggage", baggage)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func hop(name, namespace string) string {
	return fmt.Sprintf("%s(%s)", name, namespace)
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}

	return fallback
}
