t # Use the official Go image as the base image
FROM golang:1.24 AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the source code into the container
COPY . .

# Download and install any required dependencies
RUN go mod download

# Build the application
RUN go build -o gkeep2dynalist

# Use a minimal Alpine Linux image for the final image
FROM alpine:latest

# Set the working directory inside the container
WORKDIR /app

# Copy the built executable from the builder stage
COPY --from=builder /app/gkeep2dynalist .

# Install any required dependencies (if any)
RUN apk update && apk add --no-cache ca-certificates

# Set the entry point for the container
ENTRYPOINT ["/app/gkeep2dynalist"]