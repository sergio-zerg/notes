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

# Install runtime dependencies for SQLite
RUN apk add --no-cache ca-certificates

WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/notes_app .

# Expose the application port
EXPOSE 8080

# Run the application
CMD ["./notes_app"]
