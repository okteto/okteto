# syntax = docker/dockerfile:experimental

FROM bitnami/kubectl:1.24.9 as kubectl

FROM alpine:3.16 as helm
ARG HELM_VERSION=3.11.1
RUN apk --no-cache add curl && \
    apk add --no-cache bash && \
    apk add --no-cache openssl && \
    bash <( curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 ) "--version" "v${HELM_VERSION}"

FROM golang:1.18-buster as builder
WORKDIR /okteto

ENV CGO_ENABLED=0
ARG VERSION_STRING=docker
COPY go.mod .
COPY go.sum .
RUN --mount=type=cache,target=/root/.cache go mod download
COPY . .
RUN --mount=type=cache,target=/root/.cache make build
RUN chmod +x /okteto/bin/okteto

# Test
RUN /okteto/bin/okteto version

FROM alpine:3 as okteto

RUN apk add --no-cache bash ca-certificates
COPY --from=kubectl /opt/bitnami/kubectl/bin/kubectl /usr/local/bin/kubectl
COPY --from=helm /usr/local/bin/helm /usr/local/bin/helm
COPY --from=builder /okteto/bin/okteto /usr/local/bin/okteto

ENV OKTETO_DISABLE_SPINNER=true

ENV PS1="\[\e[36m\]\${OKTETO_NAMESPACE:-okteto}:\e[32m\]\${OKTETO_NAME:-dev} \[\e[m\]\W> "
