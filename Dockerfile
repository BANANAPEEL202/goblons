# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copy backend source
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Frontend build stage
FROM node:18-alpine AS frontend-builder

WORKDIR /app

# Copy frontend source
COPY frontend/ ./

# Install dependencies and build
RUN npm install
RUN npm run build

# Production stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/main .

# Copy built frontend files from frontend-builder
COPY --from=frontend-builder /app/dist ./static/

EXPOSE 8080

CMD ["./main"]