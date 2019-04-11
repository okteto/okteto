FROM golang:1.12 as builder
WORKDIR /app
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . . 
RUN CGO_ENABLED=0 GOOS=linux go build -v -o okteto

FROM alpine
COPY --from=builder /app/okteto /okteto
EXPOSE 8000
CMD ["/okteto"]