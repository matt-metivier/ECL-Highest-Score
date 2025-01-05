FROM golang:1.21-alpine AS builder

WORKDIR /build

# Copy Go module files first for better caching
COPY go/src/go.mod go/src/go.sum* ./
RUN go mod download

# Copy source code and build
COPY go/src/highest.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o highest

FROM alpine:latest

RUN apk add --no-cache nodejs npm

WORKDIR /app

# Copy data generator
COPY common/data-gen/package.json common/data-gen/gen.js ./
RUN npm install

# Copy the compiled binary
COPY --from=builder /build/highest ./

# Create entrypoint script
RUN printf '#!/bin/sh\ncd /app/data\nif [ ! -f "$1" ]; then\n    echo "Generating data..."\n    node /app/gen.js "$1" 60000\nfi\n/app/highest -file "$1" -n "$2"\n' > /app/entrypoint.sh && \
    chmod +x /app/entrypoint.sh

# Create data directory
RUN mkdir -p /app/data && chmod 777 /app/data

# Expose Prometheus metrics port
EXPOSE 8001

ENTRYPOINT ["/app/entrypoint.sh"] 