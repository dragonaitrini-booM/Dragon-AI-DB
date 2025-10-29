# Stage 1: Build the Go backend
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy backend source code
COPY backend .

# Download dependencies
RUN cd cmd/server && go mod tidy

# Build the backend
RUN cd cmd/server && CGO_ENABLED=0 GOOS=linux go build -o /server

# Stage 2: Build the final image with backend and frontend
FROM nginx:alpine

# Install supervisord
RUN apk --no-cache add supervisor

# Copy the Go binary from the builder stage
COPY --from=builder /server /usr/local/bin/server

# Copy the frontend files
COPY frontend /usr/share/nginx/html

# Copy nginx and supervisord configs
COPY nginx.conf /etc/nginx/nginx.conf
COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf

# Expose the port Hugging Face requires
EXPOSE 7860

# Start supervisord
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisor/conf.d/supervisord.conf"]
