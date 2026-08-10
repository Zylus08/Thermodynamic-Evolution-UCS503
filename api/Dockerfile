# ---------------------------------------------------------
# Stage 1: Build the Go binary
# ---------------------------------------------------------
FROM golang:1.21-alpine AS builder

# Set the working directory inside the builder container
WORKDIR /app

# Copy the go mod files and download dependencies
# (We copy these first to leverage Docker layer caching)
COPY go.mod ./
RUN go mod download

# Copy the rest of the application source code
COPY . .

# Build a statically linked Go application
# CGO_ENABLED=0 ensures the binary runs cleanly on Alpine
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o api_server main.go

# ---------------------------------------------------------
# Stage 2: Create a minimal production image
# ---------------------------------------------------------
FROM alpine:latest

# Add ca-certificates for potential HTTPS outbound requests
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the compiled binary from the builder stage
COPY --from=builder /app/api_server .

# Expose port 8080 to the internal Docker network
EXPOSE 8080

# Start the application
CMD ["./api_server"]
