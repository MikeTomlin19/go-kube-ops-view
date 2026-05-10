# Multi-stage Dockerfile for Go kube-ops-view application
# Stage 1: Build frontend assets
FROM node:26-alpine AS frontend-builder

WORKDIR /app
RUN mkdir -p /assets/static

# Copy package files
COPY app/package*.json ./
RUN npm install

# Copy frontend source
COPY app/ ./

# Build frontend assets
RUN npm run build

# Stage 2: Build Go application
FROM golang:1.25-alpine AS go-builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy built frontend assets from previous stage
COPY --from=frontend-builder /assets/static/build ./assets/static/build/

# Build arguments for version information
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

# Build the Go binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -a -installsuffix cgo \
    -o kube-ops-view \
    ./main.go

# Stage 3: Final runtime image
FROM alpine:3.19

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata && \
    update-ca-certificates

# Create non-root user for security
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Create necessary directories
RUN mkdir -p /app/static /app/templates && \
    chown -R appuser:appgroup /app

WORKDIR /app

# Copy the binary from builder stage
COPY --from=go-builder /src/kube-ops-view .

# Copy static assets and templates if they exist
COPY --from=go-builder --chown=appuser:appgroup /src/assets/static ./static/
COPY --from=go-builder --chown=appuser:appgroup /src/assets/templates ./templates/

# Switch to non-root user
USER appuser

# Expose the default port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Set up signal handling for graceful shutdown
STOPSIGNAL SIGTERM

# Run the application
ENTRYPOINT ["./kube-ops-view"]
CMD ["--port=8080"]
