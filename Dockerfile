FROM golang:1.21-alpine AS builder
RUN apk add --no-cache git make
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o dcache ./cmd/server

FROM alpine:3.18
RUN apk add --no-cache ca-certificates
RUN adduser -D -g '' dcache
WORKDIR /app
COPY --from=builder /build/dcache .
RUN chown -R dcache:dcache /app
USER dcache
EXPOSE 8080 6379
CMD ["./dcache"]
