FROM python:3.11-slim

RUN apt-get update && \
    apt-get install -y nodejs npm && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY common/data-gen/package.json common/data-gen/gen.js ./
RUN npm install
COPY python/src/highest.py ./

RUN pip install opentelemetry-api opentelemetry-sdk opentelemetry-exporter-otlp psutil

RUN printf '#!/bin/bash\nexport OTEL_EXPORTER_OTLP_ENDPOINT=grpc://otel-collector:4317\ncd /app/data\nif [ ! -f "$1" ]; then\n    echo "Generating data..."\n    node /app/gen.js "$1" 60000\nfi\npython /app/highest.py "$1" "$2"\n' > /app/entrypoint.sh && \
    chmod +x /app/entrypoint.sh

ENTRYPOINT ["/app/entrypoint.sh"] 