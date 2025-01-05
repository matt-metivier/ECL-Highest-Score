FROM node:18-slim

WORKDIR /app

COPY gen.js .
COPY package.json .
RUN npm install

RUN mkdir -p /app/data && chmod 777 /app/data

CMD ["node", "gen.js"] 