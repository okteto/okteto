# syntax = docker/dockerfile:experimental

ARG KUBECTL_VERSION=1.28.4
ARG HELM_VERSION=3.13.3
ARG KUSTOMIZE_VERSION=5.3.0


FROM golang:1.21-bullseye as kubectl-builder

ARG TARGETARCH
ARG KUBECTL_VERSION
RUN curl -sLf --retry 3 -o kubectl https://storage.googleapis.com/kubernetes-release/release/v${KUBECTL_VERSION}/bin/linux/${TARGETARCH}/kubectl && \
    cp kubectl /usr/local/bin/kubectl && \
    chmod +x /usr/local/bin/kubectl && \
    /usr/local/bin/kubectl version --client=true

FROM golang:1.21-bullseye as helm-builder

ARG TARGETARCH
ARG HELM_VERSION
RUN curl -sLf --retry 3 -o helm.tar.gz https://get.helm.sh/helm-v${HELM_VERSION}-linux-${TARGETARCH}.tar.gz && \
    mkdir -p helm && tar -C helm -xf helm.tar.gz && \
    cp helm/linux-${TARGETARCH}/helm /usr/local/bin/helm && \
    chmod +x /usr/local/bin/helm && \
    /usr/local/bin/helm version

FROM golang:1.21-bullseye as kustomize-builder
ARG TARGETARCH
ARG KUSTOMIZE_VERSION
RUN curl -sLf --retry 3 -o kustomize.tar.gz https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv${KUSTOMIZE_VERSION}/kustomize_v${KUSTOMIZE_VERSION}_linux_${TARGETARCH}.tar.gz \
    && tar -xvzf kustomize.tar.gz -C /usr/local/bin \
    && chmod +x /usr/local/bin/kustomize \
    && /usr/local/bin/kustomize version

FROM golang:1.21-bullseye as builder
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

COPY docker-credential-okteto /okteto/bin/docker-credential-okteto

FROM alpine:3

RUN apk add --no-cache bash ca-certificates

COPY --from=kubectl-builder /usr/local/bin/kubectl /usr/local/bin/kubectl
COPY --from=helm-builder /usr/local/bin/helm /usr/local/bin/helm
COPY --from=kustomize-builder /usr/local/bin/kustomize /usr/local/bin/kustomize

COPY --from=builder /okteto/bin/okteto /usr/local/bin/okteto
COPY --from=builder /okteto/bin/docker-credential-okteto /usr/local/bin/docker-credential-okteto


ENV OKTETO_DISABLE_SPINNER=true

ENV PS1="\[\e[36m\]\${OKTETO_NAMESPACE:-okteto}:\e[32m\]\${OKTETO_NAME:-dev} \[\e[m\]\W> "
