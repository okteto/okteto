FROM --platform=$BUILDPLATFORM syncthing/syncthing:1.8.0 AS syncthing
FROM --platform=$BUILDPLATFORM okteto/remote:0.3.1 AS remote
FROM --platform=$BUILDPLATFORM okteto/supervisor:0.1.2 AS supervisor
FROM --platform=$BUILDPLATFORM okteto/clean:0.1.2 AS clean

FROM --platform=$BUILDPLATFORM busybox

COPY --from=remote /usr/local/bin/remote /usr/local/bin/remote
COPY --from=supervisor /usr/local/bin/supervisor /usr/local/bin/supervisor
COPY --from=syncthing /bin/syncthing /usr/local/bin/syncthing
COPY --from=clean /usr/local/bin/clean /usr/local/bin/clean

# copy start
COPY start.sh /usr/local/bin/start.sh