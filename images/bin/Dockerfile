ARG TARGETPLATFORM
FROM --platform=$TARGETPLATFORM syncthing/syncthing:1.8.0 AS syncthing
FROM --platform=$TARGETPLATFORM okteto/remote:0.3.1 AS remote
FROM --platform=$TARGETPLATFORM okteto/supervisor:0.1.2 AS supervisor
FROM --platform=$TARGETPLATFORM okteto/clean:0.1.2 AS clean

FROM --platform=$TARGETPLATFORM busybox

COPY --from=remote /usr/local/bin/remote /usr/local/bin/remote
COPY --from=supervisor /usr/local/bin/supervisor /usr/local/bin/supervisor
COPY --from=syncthing /bin/syncthing /usr/local/bin/syncthing
COPY --from=clean /usr/local/bin/clean /usr/local/bin/clean

# copy start
COPY start.sh /usr/local/bin/start.sh