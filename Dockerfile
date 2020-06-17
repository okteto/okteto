# syntax = docker/dockerfile:experimental

FROM bitnami/kubectl:1.17.4 as kubectl
FROM alpine/helm:3.2.3 as helm

FROM okteto/golang:1.14-buster as builder
WORKDIR /okteto

ARG VERSION_STRING=docker
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build make linux/amd64/okteto-Linux-x86_64
RUN chmod +x /okteto/bin/okteto-Linux-x86_64

# Test
RUN /okteto/bin/okteto-Linux-x86_64 version

FROM alpine:3 as okteto

RUN apk add --no-cache bash ca-certificates
COPY --from=kubectl /opt/bitnami/kubectl/bin/kubectl /usr/local/bin/kubectl
COPY --from=helm /usr/bin/helm /usr/local/bin/helm
COPY --from=builder /okteto/bin/okteto-Linux-x86_64 /usr/local/bin/okteto

ENV PS1="\[\e[36m\]\${OKTETO_NAMESPACE:-okteto}:\e[32m\]\${OKTETO_NAME:-dev} \[\e[m\]\W> "
