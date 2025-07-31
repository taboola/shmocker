# Example Dockerfile for testing shmocker build functionality
# This demonstrates a simple single-stage build

# Use a minimal base image
FROM alpine:3.18

# Set build arguments with defaults
ARG VERSION=1.0.0
ARG BUILD_DATE
ARG VCS_REF

# Set metadata labels
LABEL maintainer="shmocker-team" \
      version="${VERSION}" \
      build-date="${BUILD_DATE}" \
      vcs-ref="${VCS_REF}" \
      description="Example application for testing shmocker"

# Install basic utilities
RUN apk add --no-cache \
    curl \
    ca-certificates \
    && rm -rf /var/cache/apk/*

# Create a non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy application files
COPY --chown=appuser:appgroup . /app/

# Create a simple script
RUN echo '#!/bin/sh' > /app/run.sh && \
    echo 'echo "Hello from shmocker! Version: $VERSION"' >> /app/run.sh && \
    echo 'echo "Build date: $BUILD_DATE"' >> /app/run.sh && \
    echo 'echo "VCS ref: $VCS_REF"' >> /app/run.sh && \
    echo 'echo "Current user: $(whoami)"' >> /app/run.sh && \
    echo 'echo "Current directory: $(pwd)"' >> /app/run.sh && \
    echo 'exec "$@"' >> /app/run.sh && \
    chmod +x /app/run.sh

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Set environment variables
ENV VERSION=${VERSION} \
    APP_ENV=production

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Default command
CMD ["/app/run.sh", "sh", "-c", "while true; do sleep 30; done"]