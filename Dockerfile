FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o lawrence .

FROM alpine:3.19

# Install ca-certificates and create non-root user
RUN apk --no-cache add ca-certificates && \
    addgroup -g 1001 lawrence && \
    adduser -D -s /bin/sh -u 1001 -G lawrence lawrence

# Switch to non-root user
USER lawrence

WORKDIR /home/lawrence

# Copy the binary from builder stage
COPY --from=builder /app/lawrence .

# Make it executable
RUN chmod +x ./lawrence

ENTRYPOINT ["./lawrence"]
