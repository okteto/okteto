ARG KUBECTL_VERSION=1.30.4
ARG HELM_VERSION=3.16.2
ARG KUSTOMIZE_VERSION=5.5.0
ARG SYNCTHING_VERSION=1.27.10
ARG OKTETO_REMOTE_VERSION=0.6.0
ARG OKTETO_SUPERVISOR_VERSION=0.4.0
ARG OKTETO_CLEAN_VERSION=0.2.1


FROM syncthing/syncthing:${SYNCTHING_VERSION} AS syncthing
FROM okteto/remote:${OKTETO_REMOTE_VERSION} AS remote
FROM okteto/supervisor:${OKTETO_SUPERVISOR_VERSION} AS supervisor
FROM okteto/clean:${OKTETO_CLEAN_VERSION} AS clean
FROM golang:1.22-bookworm AS golang-builder
FROM okteto/bin:1.6.1 AS okteto-bin

FROM alpine:3.18 AS certs
RUN apk add --no-cache ca-certificates

FROM golang-builder AS kubectl-builder
ARG TARGETARCH
ARG KUBECTL_VERSION
RUN curl -sLf --retry 3 -o kubectl https://storage.googleapis.com/kubernetes-release/release/v${KUBECTL_VERSION}/bin/linux/${TARGETARCH}/kubectl && \
    cp kubectl /usr/local/bin/kubectl && \
    chmod +x /usr/local/bin/kubectl && \
    /usr/local/bin/kubectl version --client=true

FROM golang-builder AS helm-builder
ARG TARGETARCH
ARG HELM_VERSION
RUN curl -sLf --retry 3 -o helm.tar.gz https://get.helm.sh/helm-v${HELM_VERSION}-linux-${TARGETARCH}.tar.gz && \
    mkdir -p helm && tar -C helm -xf helm.tar.gz && \
    cp helm/linux-${TARGETARCH}/helm /usr/local/bin/helm && \
    chmod +x /usr/local/bin/helm && \
    /usr/local/bin/helm version


FROM golang-builder AS builder
WORKDIR /okteto
ENV CGO_ENABLED=0
ARG VERSION_STRING=docker
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    go mod download
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    make build && \
    /okteto/bin/okteto version

COPY docker-credential-okteto /okteto/bin/docker-credential-okteto

FROM busybox:1.34.0

RUN addgroup cligroup && \
    adduser -D -h /home/cliuser -G cligroup cliuser

COPY --chmod=755 --from=certs /etc/ssl/certs /etc/ssl/certs
COPY --chmod=755 --from=kubectl-builder /usr/local/bin/kubectl /usr/local/bin/kubectl
COPY --chmod=755 --from=helm-builder /usr/local/bin/helm /usr/local/bin/helm
COPY --chmod=755 --from=builder /okteto/bin/okteto /usr/local/bin/okteto
COPY --chmod=755 --from=builder /okteto/bin/docker-credential-okteto /usr/local/bin/docker-credential-okteto
COPY --chmod=755 --from=remote /usr/local/bin/remote /usr/bin-image/bin/okteto-remote
COPY --chmod=755 --from=supervisor /usr/local/bin/supervisor /usr/bin-image/bin/okteto-supervisor
COPY --chmod=755 --from=syncthing /bin/syncthing /usr/bin-image/bin/syncthing
COPY --chmod=755 --from=clean /usr/local/bin/clean /usr/bin-image/bin/clean
COPY --chmod=755 scripts/start.sh /usr/bin-image/bin/start.sh

USER cliuser

ENV OKTETO_DISABLE_SPINNER=true
ENV PS1="\[\e[36m\]\${OKTETO_NAMESPACE:-okteto}:\e[32m\]\${OKTETO_NAME:-dev} \[\e[m\]\W> "
