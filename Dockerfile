# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o tuki

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache git bash openssh docker-cli

# Copy binary from builder
COPY --from=builder /app/tuki /usr/local/bin/

# Allow access to git repositories mounted as volumes
RUN git config --global --add safe.directory '*'

# Run the application
ENTRYPOINT ["tuki"]
