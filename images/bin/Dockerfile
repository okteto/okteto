FROM syncthing/syncthing:1.13.1 AS syncthing
FROM okteto/remote:0.4.1 AS remote
FROM okteto/supervisor:0.1.3 AS supervisor
FROM okteto/clean:0.1.6 AS clean

FROM busybox

COPY --from=remote /usr/local/bin/remote /usr/local/bin/okteto-remote
COPY --from=supervisor /usr/local/bin/supervisor /usr/local/bin/okteto-supervisor
COPY --from=syncthing /bin/syncthing /usr/local/bin/syncthing
COPY --from=clean /usr/local/bin/clean /usr/local/bin/clean

# copy start
COPY start.sh /usr/local/bin/start.sh
