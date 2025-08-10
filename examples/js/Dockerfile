FROM node:20-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --omit=dev
COPY . .
ENV PORT=3000
ENV OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318
EXPOSE 3000
CMD ["node", "index.js"]

