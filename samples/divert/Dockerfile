FROM golang:1.24-alpine AS builder
WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN CGO_ENABLED=0 go build -o /bin/hop .

FROM gcr.io/distroless/static-debian12
COPY --from=builder /bin/hop /hop
ENTRYPOINT ["/hop"]
