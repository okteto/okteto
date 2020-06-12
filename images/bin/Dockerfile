# syntax = docker/dockerfile:experimental

FROM syncthing/syncthing:1.6.1 AS syncthing
FROM okteto/remote:0.2.5 AS remote
FROM okteto/supervisor:0.1.0 AS supervisor
FROM okteto/clean:0.1.0 AS clean
FROM busybox as bin

COPY --from=remote /usr/local/bin/remote /usr/local/bin/remote
COPY --from=supervisor /usr/local/bin/supervisor /usr/local/bin/supervisor
COPY --from=syncthing /bin/syncthing /usr/local/bin/syncthing
COPY --from=clean /usr/local/bin/clean /usr/local/bin/clean

# copy start
COPY start.sh /usr/local/bin/start.sh