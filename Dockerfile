# Use a specific version of the Golang image for the build stage
FROM golang:1.22.2-alpine AS builder

# Install git, necessary for fetching the Go modules and source code
RUN apk --no-cache add git

# Set the working directory inside the container
WORKDIR /app

# Copy your local source code into the image
COPY . .

# Adjust permissions to ensure all files in /app are accessible by 'nobody'
# Set ownership of all files in the /app directory to 'nobody'
RUN chown -R nobody:nobody /app

# Create a directory for the build cache that the 'nobody' user can access
RUN mkdir /.cache && chown nobody:nobody /.cache

# Add a non-root user and switch to it
USER nobody

# Set environment variable for Go cache directory
ENV GOCACHE=/.cache/go-build

# Run go mod tidy to ensure all dependencies are correct and go.sum is updated based on your go.mod
RUN go mod tidy

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Build the application. Consider simplifying build flags if debug information is not an issue or necessary for troubleshooting
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o electronwall .

# Switch to a new stage using Alpine to keep the final image small
FROM alpine:latest

# Install Bash in the final image
RUN apk add --no-cache bash

# Create a user 'appuser' and switch to it
RUN adduser -D appuser
USER appuser

# Set work directory to /app
WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/electronwall .

# Copy the rules directory from the builder stage
COPY --from=builder /app/rules /app/rules

# Specify the container's executable
CMD ["./electronwall"]
