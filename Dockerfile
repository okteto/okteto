# Base image versions - Centralized version control for easier updates
# Kubernetes tools
ARG KUBECTL_VERSION=1.31.6
ARG HELM_VERSION=3.17.1
ARG KUSTOMIZE_VERSION=5.5.0
# Okteto components
ARG SYNCTHING_VERSION=1.29.2
ARG OKTETO_REMOTE_VERSION=0.6.2
ARG OKTETO_SUPERVISOR_VERSION=0.4.1
ARG OKTETO_CLEAN_VERSION=0.2.2
# Base images
ARG GOLANG_VERSION=1.23.6
ARG ALPINE_VERSION=3.20
ARG BUSYBOX_VERSION=1.36.1
ARG GIT_VERSION=2.42.0

# Stage 1: Prepare base components
# SSL certificates for secure connections
FROM alpine:${ALPINE_VERSION} AS certs
RUN apk add --no-cache ca-certificates

# UPX binary compression tool - used to reduce binary sizes
FROM alpine:${ALPINE_VERSION} AS upx-provider
RUN apk add --no-cache curl \
    && curl -L -o /usr/bin/upx.tar.xz https://github.com/upx/upx/releases/download/v4.2.1/upx-4.2.1-amd64_linux.tar.xz \
    && tar -xf /usr/bin/upx.tar.xz -C /tmp \
    && cp /tmp/upx-4.2.1-amd64_linux/upx /usr/bin/upx \
    && chmod +x /usr/bin/upx

# Stage 2: Compress Okteto utility binaries to reduce final image size
# File synchronization tool
FROM syncthing/syncthing:${SYNCTHING_VERSION} AS syncthing
COPY --from=upx-provider /usr/bin/upx /usr/bin/upx
RUN cp /bin/syncthing /tmp/syncthing && \
    /usr/bin/upx --best --lzma /tmp/syncthing

# Remote execution component
FROM okteto/remote:${OKTETO_REMOTE_VERSION} AS remote
COPY --from=upx-provider /usr/bin/upx /usr/bin/upx
RUN cp /usr/local/bin/remote /tmp/remote && \
    /usr/bin/upx --best --lzma /tmp/remote

# Process supervisor component
FROM okteto/supervisor:${OKTETO_SUPERVISOR_VERSION} AS supervisor
COPY --from=upx-provider /usr/bin/upx /usr/bin/upx
RUN cp /usr/local/bin/supervisor /tmp/supervisor && \
    /usr/bin/upx --best --lzma /tmp/supervisor

# Cleanup utility
FROM okteto/clean:${OKTETO_CLEAN_VERSION} AS clean
COPY --from=upx-provider /usr/bin/upx /usr/bin/upx
RUN cp /usr/local/bin/clean /tmp/clean && \
    /usr/bin/upx --best --lzma /tmp/clean

# Stage 3: Set up Go build environment for Kubernetes tools and Okteto CLI
FROM golang:${GOLANG_VERSION}-bookworm AS golang-builder
COPY --from=upx-provider /usr/bin/upx /usr/bin/upx

# Stage 3.1: Download and compress kustomize (Kubernetes resource customization tool)
FROM golang-builder AS kustomize-builder
ARG TARGETARCH
ARG KUSTOMIZE_VERSION
RUN curl -sLf --retry 3 -o kustomize.tar.gz \
    "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv${KUSTOMIZE_VERSION}/kustomize_v${KUSTOMIZE_VERSION}_linux_${TARGETARCH}.tar.gz" \
    && tar -xzf kustomize.tar.gz -C /usr/local/bin \
    && chmod +x /usr/local/bin/kustomize \
    && rm kustomize.tar.gz \
    # Verify binary works before compression
    && /usr/local/bin/kustomize version \
    # Compress binary to reduce size
    && cp /usr/local/bin/kustomize /tmp/kustomize \
    && /usr/bin/upx --best --lzma /tmp/kustomize \
    # Verify compressed binary still works
    && /tmp/kustomize version

# Stage 3.2: Download and compress kubectl (Kubernetes CLI)
FROM golang-builder AS kubectl-builder
ARG TARGETARCH
ARG KUBECTL_VERSION
RUN curl -sLf --retry 3 -o kubectl \
    "https://dl.k8s.io/release/v${KUBECTL_VERSION}/bin/linux/${TARGETARCH}/kubectl" \
    && chmod +x kubectl \
    && mv kubectl /usr/local/bin/ \
    # Verify binary works before compression
    && /usr/local/bin/kubectl version --client=true \
    # Compress binary to reduce size
    && cp /usr/local/bin/kubectl /tmp/kubectl \
    && /usr/bin/upx --best --lzma /tmp/kubectl \
    # Verify compressed binary still works
    && /usr/local/bin/kubectl version --client=true

# Stage 3.3: Download and compress Helm (Kubernetes package manager)
FROM golang-builder AS helm-builder
ARG TARGETARCH
ARG HELM_VERSION
RUN curl -sLf --retry 3 -o helm.tar.gz \
    "https://get.helm.sh/helm-v${HELM_VERSION}-linux-${TARGETARCH}.tar.gz" \
    && mkdir -p helm \
    && tar -C helm -xf helm.tar.gz \
    && mv helm/linux-${TARGETARCH}/helm /usr/local/bin/ \
    && chmod +x /usr/local/bin/helm \
    && rm -rf helm helm.tar.gz \
    # Verify binary works before compression
    && /usr/local/bin/helm version \
    # Compress binary to reduce size
    && cp /usr/local/bin/helm /tmp/helm \
    && /usr/bin/upx --best --lzma /tmp/helm \
    # Verify compressed binary still works
    && /tmp/helm version

# Stage 3.4: Download and compress git (Version control system)
FROM debian:bookworm-slim AS git-builder

ARG GIT_VERSION="2.42.0"
ENV CC=gcc

RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential autoconf pkg-config ca-certificates \
      libcurl4-openssl-dev libssl-dev libexpat1-dev zlib1g-dev \
      gettext wget curl \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /usr/src/git
RUN curl -fSL \
      "https://mirrors.edge.kernel.org/pub/software/scm/git/git-${GIT_VERSION}.tar.gz" \
      -o git.tar.gz \
  && tar -xzf git.tar.gz --strip-components=1 \
  && rm git.tar.gz

RUN make configure \
 && ./configure --prefix=/usr \
      CFLAGS="-static" LDFLAGS="-static" \
      NO_GETTEXT=YesPlease NO_PYTHON=YesPlease \
 && make -j"$(nproc)" \
 && make install \
 && strip /usr/bin/git \
 && rm -rf /usr/src/git

# Stage 4: Build and compress the Okteto CLI
FROM golang-builder AS builder
WORKDIR /okteto
# Disable CGO for a more portable binary
ENV CGO_ENABLED=0
ARG VERSION_STRING=docker

# Step 1: Copy only dependency files first to leverage Docker cache
# This creates a separate layer for dependencies that changes less frequently
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Step 2: Copy source code and build the application
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    make build && \
    # Validate binary functionality with health checks
    /okteto/bin/okteto version && \
    /okteto/bin/okteto --help > /dev/null && \
    echo "Binary validation successful" && \
    # Prepare docker-credential-okteto helper
    mkdir -p /okteto/bin && \
    cp docker-credential-okteto /okteto/bin/ && \
    # Compress the binary with UPX to reduce size
    cp /okteto/bin/okteto /tmp/okteto && \
    /usr/bin/upx --best --lzma /tmp/okteto && \
    # Verify compressed binary still works
    /tmp/okteto --help > /dev/null

# Stage 5: Create the final minimal image
# Using BusyBox as the base for a tiny footprint
FROM busybox:${BUSYBOX_VERSION}

# Step 1: Copy SSL certificates for secure connections
COPY --link --chmod=755 --from=certs /etc/ssl/certs /etc/ssl/certs

# Step 2: Copy compressed Kubernetes tools
COPY --link --chmod=755 --from=kubectl-builder /tmp/kubectl /usr/local/bin/kubectl
COPY --link --chmod=755 --from=kustomize-builder /tmp/kustomize /usr/local/bin/kustomize
COPY --link --chmod=755 --from=helm-builder /tmp/helm /usr/local/bin/helm

# Step 3: Copy Okteto CLI and credential helper
COPY --link --chmod=755 --from=builder /tmp/okteto /usr/local/bin/okteto
COPY --link --chmod=755 --from=builder /okteto/bin/docker-credential-okteto /usr/local/bin/docker-credential-okteto

# Step 4: Copy Okteto supporting utilities
COPY --link --chmod=755 --from=remote /tmp/remote /usr/bin-image/bin/okteto-remote
COPY --link --chmod=755 --from=supervisor /tmp/supervisor /usr/bin-image/bin/okteto-supervisor
COPY --link --chmod=755 --from=syncthing /tmp/syncthing /usr/bin-image/bin/syncthing
COPY --link --chmod=755 --from=clean /tmp/clean /usr/bin-image/bin/clean
COPY --link --chmod=755 scripts/start.sh /usr/bin-image/bin/start.sh
COPY --link --chmod=755 --from=git-builder /usr/bin/git /usr/bin/git

# Step 5: Add OCI-compliant metadata labels
# https://github.com/opencontainers/image-spec/blob/main/annotations.md
ARG VERSION_STRING
# Generate build date at build time
LABEL org.opencontainers.image.created="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      org.opencontainers.image.source="https://github.com/okteto/okteto" \
      org.opencontainers.image.url="https://www.okteto.com" \
      org.opencontainers.image.documentation="https://www.okteto.com/docs/" \
      org.opencontainers.image.version="${VERSION_STRING}" \
      org.opencontainers.image.revision="${VERSION_STRING}" \
      org.opencontainers.image.title="Okteto CLI" \
      org.opencontainers.image.description="Okteto accelerates the development workflow of Kubernetes applications, enabling you to code locally while running your applications in a remote cluster." \
      org.opencontainers.image.vendor="Okteto" \
      org.opencontainers.image.licenses="Apache-2.0"

# Step 6: Configure runtime environment
# Disable spinner for cleaner logs in container environments
ENV OKTETO_DISABLE_SPINNER=true
# Set a colorful and informative prompt
ENV PS1="\[\e[36m\]\${OKTETO_NAMESPACE:-okteto}:\e[32m\]\${OKTETO_NAME:-dev} \[\e[m\]\W> "