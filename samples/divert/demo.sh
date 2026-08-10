#!/usr/bin/env bash
#
# Demonstrates `okteto divert`: the same request, with and without a routing header,
# taking two different paths through the call chain.
#
# Assumes shared.yaml is deployed in the shared namespace, one of the developer-*.yaml
# manifests is deployed in yours, and `okteto divert up` has already run. See README.md.
#
# It only ever calls frontend, so it works whichever service in the chain you diverted.
#
#   ./demo.sh <shared-namespace> <routing-key>

set -euo pipefail

SHARED_NS="${1:-}"
KEY="${2:-}"

if [ -z "$SHARED_NS" ] || [ -z "$KEY" ]; then
  echo "usage: $0 <shared-namespace> <routing-key>" >&2
  exit 1
fi

PORT="${PORT:-18080}"
SAMPLES="${SAMPLES:-20}"

echo "==> port-forwarding frontend.${SHARED_NS} to localhost:${PORT}"
kubectl port-forward -n "$SHARED_NS" svc/frontend "${PORT}:80" >/dev/null &
FORWARD_PID=$!
trap 'kill "$FORWARD_PID" 2>/dev/null || true' EXIT

# Wait for the forward rather than sleeping a guessed number of seconds.
for _ in $(seq 1 50); do
  if curl -fsS --max-time 1 "http://localhost:${PORT}/" >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done

echo
echo "==> without the routing header (everything shared)"
curl -fsS "http://localhost:${PORT}/"
echo

echo
echo "==> with 'baggage: divert=${KEY}' (the middle hop moves, the rest stays shared)"
curl -fsS -H "baggage: divert=${KEY}" "http://localhost:${PORT}/"
echo

echo
echo "==> with an unknown routing key (must fall back to baseline, never 404)"
curl -fsS -H "baggage: divert=this-key-does-not-exist" "http://localhost:${PORT}/"
echo

echo
echo "==> extra hop cost, ${SAMPLES} requests each"
measure() {
  local label="$1"
  shift
  local total=0
  for _ in $(seq 1 "$SAMPLES"); do
    local t
    t=$(curl -fsS -o /dev/null -w '%{time_total}' "$@" "http://localhost:${PORT}/")
    total=$(echo "$total + $t" | bc -l)
  done
  echo "    ${label}: $(echo "scale=1; $total * 1000 / $SAMPLES" | bc -l) ms avg"
}

measure "baseline (no header) "
measure "diverted (with header)" -H "baggage: divert=${KEY}"

echo
echo "The difference is the one extra in-cluster proxy hop."
