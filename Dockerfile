# Build stage
FROM golang:1.25-alpine AS builder

# Set build arguments
ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT

# Install build dependencies
RUN apk add --no-cache \
    git \
    make \
    build-base \
    linux-headers

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.gitCommit=${GIT_COMMIT}" \
    -trimpath \
    -o ember \
    .

# Verify the binary
RUN ./ember --version

# Runtime stage
FROM alpine:3.20

# Set metadata labels
LABEL org.opencontainers.image.title="Ember"
LABEL org.opencontainers.image.description="Modern, cross-platform terminal application for creating bootable Windows USB drives"
LABEL org.opencontainers.image.vendor="Entro314 Labs"
LABEL org.opencontainers.image.licenses="Apache-2.0"
LABEL org.opencontainers.image.source="https://github.com/entro314-labs/ember"
LABEL org.opencontainers.image.documentation="https://github.com/entro314-labs/ember#readme"

# Install runtime dependencies
RUN apk add --no-cache \
    util-linux \
    coreutils \
    findutils \
    e2fsprogs \
    dosfstools \
    ntfs-3g \
    exfat-utils \
    parted \
    gdisk \
    ca-certificates \
    tzdata \
    && rm -rf /var/cache/apk/*

# Create non-root user
RUN addgroup -g 1000 ember && \
    adduser -D -s /bin/sh -u 1000 -G ember ember

# Copy binary from builder stage
COPY --from=builder /app/ember /usr/local/bin/ember

# Copy any additional binaries if they exist
RUN if [ -d /app/binaries ]; then cp -r /app/binaries/* /usr/local/bin/ || true; fi

# Set permissions
RUN chmod +x /usr/local/bin/ember

# Create directories for USB operations
RUN mkdir -p /mnt/usb /mnt/iso && \
    chown ember:ember /mnt/usb /mnt/iso

# Switch to non-root user for security
USER ember

# Set working directory
WORKDIR /home/ember

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ember --version || exit 1

# Set default entrypoint and command
ENTRYPOINT ["ember"]
CMD ["--help"]

# Add build information
ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT
ENV EMBER_VERSION=${VERSION}
ENV EMBER_BUILD_TIME=${BUILD_TIME}
ENV EMBER_GIT_COMMIT=${GIT_COMMIT}