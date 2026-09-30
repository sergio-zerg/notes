# Build Stage
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Copy go mod files first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application.
# CGO is not required: pgx is pure Go and the driver is database/sql based.
# Migration SQL files are embedded into the binary via go:embed, so nothing
# needs to be copied alongside the executable.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/notes_app ./cmd/api

# Final Stage
FROM alpine:latest

# ca-certificates: outbound TLS. curl: container healthcheck.
RUN apk add --no-cache ca-certificates curl

# Create a non-root user for security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /out/notes_app .

# Ensure the binary is executable
RUN chmod +x ./notes_app

# Run as non-root user
USER appuser

# Expose the application ports
EXPOSE 8080 50051

# Run the application
CMD ["./notes_app"]
