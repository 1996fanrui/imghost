# syntax=docker/dockerfile:1.7

# Build stage: compile swag + imghost into a static binary.
FROM golang:1.26.2-alpine3.22 AS builder

RUN apk add --no-cache git

WORKDIR /src

# Cache modules first.
COPY go.mod go.sum ./
RUN go mod download

# Pin swag CLI to the same version as the library in go.sum.
RUN go install github.com/swaggo/swag/cmd/swag@v1.16.6

COPY . .

# Regenerate swagger docs inside the image so they match the source in context.
RUN swag init -g cmd/imghost/main.go -o docs

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /out/imghost \
    ./cmd/imghost

# Runtime stage: alpine for shell + ca-certificates (outbound HTTPS).
FROM alpine:3.22

RUN apk add --no-cache ca-certificates

COPY --from=builder /out/imghost /usr/local/bin/imghost

EXPOSE 34286
ENTRYPOINT ["/usr/local/bin/imghost"]
