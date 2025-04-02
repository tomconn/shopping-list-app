# Stage 1: Build the Go application
FROM golang:1.24-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy go module files
COPY go.mod go.sum ./

# Download Go module dependencies
# Using -mod=readonly ensures we don't accidentally change go.mod/go.sum
RUN go mod download && go mod verify

# Copy the source code into the container
COPY . .

# Run unit tests
# The build will fail here if any tests fail.
# Use -v for verbose output, helpful for debugging build failures.
RUN go test -v ./...

# Build the Go app - static build if possible, disable CGO
# -ldflags="-w -s" strips debug symbols and reduces binary size
# CGO_ENABLED=0 ensures a static build without C dependencies (good for Alpine)
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /shopping-list-backend .

# Stage 2: Create the final, smaller image
FROM alpine:latest

# Install ca-certificates for HTTPS requests if the app needs to make them
# Also needed for TLS database connections if SSLMODE is not 'disable'
RUN apk --no-cache add ca-certificates

# Set the working directory
WORKDIR /app

# Copy the built binary from the builder stage
COPY --from=builder /shopping-list-backend /app/shopping-list-backend

# Expose the port the app runs on (must match APP_PORT in main.go/docker-compose)
EXPOSE 8080

# Command to run the application
# Use the full path to the executable
CMD ["/app/shopping-list-backend"]