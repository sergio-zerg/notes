# Build Stage
FROM golang:1.23-alpine AS builder

# Install build dependencies for go-sqlite3
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Copy go mod files first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -o notes_app ./cmd/api/main.go

# Final Stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates

# Create a non-root user for security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/notes_app .

# Ensure the binary is executable
RUN chmod +x ./notes_app

# Run as non-root user
USER appuser

# Expose the application ports
EXPOSE 8080 50051

# Run the application
CMD ["./notes_app"]
