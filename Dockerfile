# syntax = docker/dockerfile:experimental

ARG KUBECTL_VERSION=1.22.17
ARG HELM_VERSION=3.12.0
ARG KUSTOMIZE_VERSION=5.0.0

FROM golang:1.19.6-bullseye as kubectl-builder
ARG KUBECTL_VERSION
RUN curl -sLf --retry 3 -o kubectl https://storage.googleapis.com/kubernetes-release/release/v${KUBECTL_VERSION}/bin/linux/amd64/kubectl && \
    cp kubectl /usr/local/bin/kubectl && \
    chmod +x /usr/local/bin/kubectl && \
    /usr/local/bin/kubectl version --client=true

FROM golang:1.19.6-bullseye as helm-builder
ARG HELM_VERSION
RUN curl -sLf --retry 3 -o helm.tar.gz https://get.helm.sh/helm-v${HELM_VERSION}-linux-amd64.tar.gz && \
    mkdir -p helm && tar -C helm -xf helm.tar.gz && \
    cp helm/linux-amd64/helm /usr/local/bin/helm && \
    chmod +x /usr/local/bin/helm && \
    /usr/local/bin/helm version

FROM golang:1.19.6-bullseye as kustomize-builder
ARG KUSTOMIZE_VERSION
RUN curl -sLf --retry 3 -o kustomize.tar.gz https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv${KUSTOMIZE_VERSION}/kustomize_v${KUSTOMIZE_VERSION}_linux_amd64.tar.gz \
    && tar -xvzf kustomize.tar.gz -C /usr/local/bin \
    && chmod +x /usr/local/bin/kustomize \
    && /usr/local/bin/kustomize version

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

FROM alpine:3

RUN apk add --no-cache bash ca-certificates

COPY --from=kubectl-builder /usr/local/bin/kubectl /usr/local/bin/kubectl
COPY --from=helm-builder /usr/local/bin/helm /usr/local/bin/helm
COPY --from=kustomize-builder /usr/local/bin/kustomize /usr/local/bin/kustomize

COPY --from=builder /okteto/bin/okteto /usr/local/bin/okteto

ENV OKTETO_DISABLE_SPINNER=true

ENV PS1="\[\e[36m\]\${OKTETO_NAMESPACE:-okteto}:\e[32m\]\${OKTETO_NAME:-dev} \[\e[m\]\W> "
