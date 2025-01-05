FROM node:18-slim

WORKDIR /app

# Copy data generation files
COPY common/data-gen/gen.js .
COPY common/data-gen/package.json .

# Install dependencies
RUN npm install

# Create output directory
WORKDIR /app/data

# Run data generation
CMD ["node", "/app/gen.js"] 