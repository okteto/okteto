# syntax = docker/dockerfile:experimental

FROM syncthing/syncthing:1.5.0 AS syncthing
FROM okteto/remote:0.2.5 AS remote
FROM okteto/supervisor:0.1.0 AS supervisor
FROM okteto/clean:0.1.0 AS clean
FROM bitnami/kubectl:1.17.4 as kubectl
FROM alpine/helm:3.2.1 as helm

FROM okteto/golang:1 as builder
WORKDIR /okteto

COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build make linux/amd64/okteto-Linux-x86_64
RUN chmod +x /okteto/bin/okteto-Linux-x86_64

# Test
RUN /okteto/bin/okteto-Linux-x86_64 version

FROM busybox as bin

COPY --from=remote /usr/local/bin/remote /usr/local/bin/remote
COPY --from=supervisor /usr/local/bin/supervisor /usr/local/bin/supervisor
COPY --from=syncthing /bin/syncthing /usr/local/bin/syncthing
COPY --from=clean /usr/local/bin/clean /usr/local/bin/clean

# copy start
COPY scripts/start.sh /usr/local/bin/start.sh

FROM alpine:3 as okteto

ARG VERSION_STRING=docker
RUN apk add --no-cache bash ca-certificates
COPY --from=kubectl /opt/bitnami/kubectl/bin/kubectl /usr/local/bin/kubectl
COPY --from=helm /usr/bin/helm /usr/local/bin/helm
COPY --from=builder /okteto/bin/okteto-Linux-x86_64 /usr/local/bin/okteto
ENV PS1="\[\e[36m\]\${OKTETO_NAMESPACE:-okteto}:\e[32m\]\${OKTETO_NAME:-dev} \[\e[m\]\W> "
