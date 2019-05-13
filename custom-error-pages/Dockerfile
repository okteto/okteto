# syntax = docker/dockerfile:experimental

FROM golang:1.12 as builder

WORKDIR /app

ENV GO111MODULE=on
COPY go.mod .
RUN go mod download

COPY . . 
RUN --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 GOOS=linux go build -o pages

FROM alpine
RUN apk update \
        && apk upgrade \
        && apk add --no-cache \
        ca-certificates \
        && update-ca-certificates 2>/dev/null || true

COPY --from=builder /app/pages /pages
COPY www www
EXPOSE 8080
CMD ["/pages"]