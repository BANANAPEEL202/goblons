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

WORKDIR /frontend

# Copy frontend source
COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

# Production stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/main .

# Copy built frontend files from frontend-builder
COPY --from=frontend-builder /frontend/dist ./static

EXPOSE 8080

CMD ["./main"]