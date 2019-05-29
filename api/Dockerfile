# syntax = docker/dockerfile:experimental

FROM golang:1.12 as builder

WORKDIR /app

ENV GO111MODULE=on
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY cmd cmd
COPY pkg pkg 
RUN --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 GOOS=linux go build -o okteto cmd/okteto/main.go

FROM alpine
RUN apk update \
        && apk upgrade \
        && apk add --no-cache \
        ca-certificates \
        && update-ca-certificates 2>/dev/null || true

COPY --from=builder /app/okteto /okteto
COPY assets assets
EXPOSE 8080
CMD ["/okteto"]