FROM python:3.11-slim

# Set up working directory
WORKDIR /app

# Copy source code
COPY python/src/highest.py .

# Install dependencies
RUN pip install opentelemetry-api \
    opentelemetry-sdk \
    opentelemetry-exporter-otlp-proto-grpc \
    psutil

# Set up OpenTelemetry endpoint
ENV OTEL_EXPORTER_OTLP_ENDPOINT=grpc://otel-collector:4317

# Create data directory
WORKDIR /app/data

# Run the implementation
ENTRYPOINT ["python", "/app/highest.py"] 