ARG KUBECTL_VERSION=1.30.4
ARG HELM_VERSION=3.15.4

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
COPY --from=certs /etc/ssl/certs /etc/ssl/certs
COPY --from=kubectl-builder /usr/local/bin/kubectl /usr/local/bin/kubectl
COPY --from=helm-builder /usr/local/bin/helm /usr/local/bin/helm
COPY --from=builder /okteto/bin/okteto /usr/local/bin/okteto
COPY --from=builder /okteto/bin/docker-credential-okteto /usr/local/bin/docker-credential-okteto
COPY --from=okteto-bin /usr/local/bin/* /usr/bin-image/bin

ENV OKTETO_DISABLE_SPINNER=true
ENV PS1="\[\e[36m\]\${OKTETO_NAMESPACE:-okteto}:\e[32m\]\${OKTETO_NAME:-dev} \[\e[m\]\W> "
