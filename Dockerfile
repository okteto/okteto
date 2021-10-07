# syntax = docker/dockerfile:experimental

FROM bitnami/kubectl:1.17.4 as kubectl
FROM alpine/helm:3.3.0 as helm

FROM golang:1.17-buster as builder
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
COPY --from=helm /usr/bin/helm /usr/local/bin/helm
COPY --from=builder /okteto/bin/okteto /usr/local/bin/okteto

ENV PS1="\[\e[36m\]\${OKTETO_NAMESPACE:-okteto}:\e[32m\]\${OKTETO_NAME:-dev} \[\e[m\]\W> "
