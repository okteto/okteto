# syntax = docker/dockerfile:experimental

FROM golang:1.12 as builder

WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY pkg pkg
COPY cmd cmd
RUN --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 GOOS=linux go build -o webhook cmd/webhook/main.go

FROM alpine
RUN apk update \
        && apk upgrade \
        && apk add --no-cache \
        ca-certificates \
        && update-ca-certificates 2>/dev/null || true

COPY --from=builder /app/webhook /webhook
EXPOSE 8443
CMD ["/webhook"]