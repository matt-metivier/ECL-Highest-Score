FROM golang:1.23.1-alpine

# Set up working directory
WORKDIR /app

# Copy source code
COPY go/src/highest.go .

# Install dependencies
COPY go/src/go.mod .
COPY go/src/go.sum .
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o highest

# Set up OpenTelemetry endpoint
ENV OTEL_EXPORTER_OTLP_ENDPOINT=grpc://otel-collector:4317

# Create data directory
WORKDIR /app/data

# Run the implementation
ENTRYPOINT ["/app/highest"] 