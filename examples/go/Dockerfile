FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod .
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o server .

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /app/server /app/server
EXPOSE 8080
ENV OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318
ENTRYPOINT ["/app/server"]

