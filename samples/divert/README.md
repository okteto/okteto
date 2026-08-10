# Header-based traffic divert

Deploy only the service you changed into your own namespace, and have requests carrying a
routing header reach that copy — while every other service in the call chain stays shared.

```text
Without header:  frontend(staging) -> api(staging) -> catalog(staging)     -> inventory(staging)
With header:     frontend(staging) -> api(staging) -> catalog-DEV(you)     -> inventory(staging)
```

No service mesh is involved. `okteto divert up` puts a small reverse proxy in front of the
shared Service by repointing that Service's selector at it, and takes it back out again on
`okteto divert down`.

## What the sample is

A four-service chain — `frontend → api → catalog → inventory` — where one tiny Go binary
plays every role. Each instance reports its name and namespace, calls its upstream, and
appends the answer, so the response body _is_ the path the request took.

Four services rather than two, because the interesting case is a divert **in the middle**:
shared services in front of your copy and behind it at the same time.

There are two developer manifests, and they demonstrate different things:

| Manifest                                           | Diverts   | Why it is interesting                                    |
| -------------------------------------------------- | --------- | -------------------------------------------------------- |
| [`developer-api.yaml`](developer-api.yaml)         | `api`     | The first hop. Only `frontend` is in front of your copy. |
| [`developer-catalog.yaml`](developer-catalog.yaml) | `catalog` | Mid-chain. Two shared services in front, one behind.     |

The one thing the application has to do itself is **forward the `baggage` header on its own
outbound calls** ([`main.go`](main.go), `call`). No tool can do this for you, and diverting
`catalog` is where it stops being theoretical: the header has to survive `frontend` _and_
`api`, neither of which you deployed, before it can reach your copy.

## Requirements

- **Any Kubernetes cluster.** No service mesh, no CRDs, no controller, and no Okteto
  installation — `okteto divert` works against whatever your kubeconfig points at,
  including kind and vanilla managed clusters.
- A shared namespace you can create and patch Services in (see the RBAC in the
  `cmd/divert` package docs).
- Your own namespace.
- `kubectl` and `curl`. `bc` for the latency numbers.

## Running it

Build and push the image once, wherever your cluster can pull it from, and update the
`image:` field in the manifests if you use a different reference:

```bash
docker build -t ifbyol/divert-sample:1 samples/divert
docker push ifbyol/divert-sample:1
```

Deploy the shared baseline — all four services:

```bash
kubectl apply -n staging -f samples/divert/shared.yaml
```

### Diverting the middle of the chain

Deploy your own copy of `catalog` — and nothing else:

```bash
kubectl apply -n "$MY_NS" -f samples/divert/developer-catalog.yaml
okteto divert up --service catalog --from staging --key alice
./samples/divert/demo.sh staging alice
```

Expected output:

```text
==> without the routing header (everything shared)
frontend(staging) -> api(staging) -> catalog(staging) -> inventory(staging)

==> with 'baggage: divert=alice' (the middle hop moves, the rest stays shared)
frontend(staging) -> api(staging) -> catalog-DEV(alice-dev) -> inventory(staging)

==> with an unknown routing key (must fall back to baseline, never 404)
frontend(staging) -> api(staging) -> catalog(staging) -> inventory(staging)
```

Three things are worth reading off that output:

- **`frontend` and `api` did not move.** You deployed one service and the rest of the chain
  stayed shared, which is the whole proposition.
- **The header crossed two services you do not own.** If `api` dropped it, the second line
  would look exactly like the first — and that is the most common reason a real divert
  appears not to work.
- **`inventory` is still shared.** It is not deployed in your namespace; `okteto divert up`
  mirrored it in as an `ExternalName` service so your copy of `catalog` can reach it. The
  third case matters as much as the second: a stale routing key degrades to normal
  behaviour rather than failing.

### Diverting the first hop instead

```bash
kubectl apply -n "$MY_NS" -f samples/divert/developer-api.yaml
okteto divert up --service api --from staging --key alice
```

```text
frontend(staging) -> api-DEV(alice-dev) -> catalog(staging) -> inventory(staging)
```

### Diverting both at once

Nothing stops you deploying both manifests and diverting both services. Note what happens:

```text
frontend(staging) -> api-DEV(alice-dev) -> catalog-DEV(alice-dev) -> inventory(staging)
```

Your copy of `api` reaches your copy of `catalog` **directly**, without going through
`catalog`'s router at all — `catalog` resolves inside your own namespace, where your copy
wins over the mirror. The router in the shared namespace is still doing its job for callers
that live there. This is the intended behaviour, not a bypass: a service you have deployed
locally should be reached locally.

## Cleaning up

```bash
okteto divert down --service catalog --from staging
okteto divert down --service api --from staging
```

Each restores its shared Service exactly as it was and removes its router. Both are safe to
run twice, and safe to run from a different machine than the one that started the divert:
everything needed to undo them is stored on the Services themselves.

By default this removes only your own routing key — if a colleague is diverting the same
service, their divert survives. Use `--all` to remove the whole thing.

## If something goes wrong

The router logs every routing decision. Find it and read them:

```bash
kubectl logs -n staging -l divert-router=catalog --tail=50
```

Each line reports the routing key it saw, where it sent the request, and how long it took.
A request that ended up on the baseline when you expected otherwise almost always means the
header did not arrive — check that every hop in front of the diverted service forwards it.

To undo a divert without the CLI, see the recovery procedure in the `pkg/divert/swap`
package documentation.

## If the divert seems intermittent

`okteto divert up` handles this for you, but it is worth understanding, because it is
inherent to the approach rather than a bug.

`kube-proxy` chooses a backend **when a TCP connection is established**, not per request. A
caller that already had a connection open to the shared service when you diverted stays
pinned to the pod it was talking to, and keeps reaching the shared version — possibly
forever, since a connection carrying steady traffic never idles out of the pool.

It is easy to assume this sample does not pool: it builds a fresh `http.Request` for every
incoming call. But it shares one `http.Client` across all of them, and Go's default
transport reuses the TCP connection underneath ([`main.go`](main.go), `newClient`). A new
request is not a new connection.

The fix is to make those callers reconnect, and the connections all terminate at the pods of
the workload being diverted — so `divert up` rolls that workload once the router is serving.
One restart, however many callers there are, and no need to know who they are.

Pass `--no-restart` to skip it. The divert still works for anything that connects afterwards;
existing callers keep reaching the shared version until they reconnect on their own. To do it
by hand later:

```bash
# The workload you diverted, not its callers. After divert up, not before.
kubectl rollout restart -n staging deployment/catalog
```

You can also see the underlying behaviour directly, by turning connection pooling off in the
sample:

```bash
# api is what calls catalog, so api is the one that must stop pooling.
kubectl set env -n staging deployment/api NEW_CONNECTION_PER_REQUEST=true
okteto divert up --service catalog --from staging --key alice --no-restart
```

With pooling off, the divert applies immediately even without a restart. With it on — the
realistic default — it does not. That contrast is the whole lesson.

A service mesh does not have this problem, because its sidecar intercepts inside the pod and
so applies per request even on an existing connection. It is the one real behavioural
difference between this approach and the Istio driver.

## Known limits

- Callers must reconnect to start going through the router (above).
- Only HTTP can be header-routed. Postgres, Redis, Kafka and raw TCP cannot, and a Service
  exposing a UDP port is refused rather than half-diverted.
- Shared state is shared: a write your copy makes to a shared database is visible to
  everyone.
