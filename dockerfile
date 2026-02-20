# Use the official Go image as the base image
FROM golang:1.23-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the Go module files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the Go app
RUN go build -o app

# Use a lightweight Alpine image for the final stage
FROM alpine:latest

# Set the working directory
WORKDIR /app

# Copy the built binary from the builder stage
COPY --from=builder /app/app .

# Copy the "public" and "static" folders
COPY public ./public
COPY static ./static

# Expose port 4000
EXPOSE 4000

# Run the app
CMD ["./app"]