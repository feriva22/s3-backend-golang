# ---------- Builder stage ----------
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install git (needed for go modules)
RUN apk add --no-cache git

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary (static, no CGO)
RUN CGO_ENABLED=0 GOOS=linux go build -o s3-backend main.go

# ---------- Runtime stage ----------
FROM alpine:3.20

WORKDIR /app

# Copy compiled binary from builder
COPY --from=builder /app/s3-backend .

# Expose port
EXPOSE 4000

# Run binary
CMD ["./s3-backend"]
