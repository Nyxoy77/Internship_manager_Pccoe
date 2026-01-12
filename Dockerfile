# ---------- STAGE 1: Build ----------
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install git (needed for some Go modules)
RUN apk add --no-cache git

# Copy dependency files first (cache-friendly)
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o app ./cmd/server/main.go


# ---------- STAGE 2: Runtime ----------
FROM alpine:latest

WORKDIR /app

# Add CA certificates (needed for HTTPS, JWT, MinIO, etc.)
RUN apk add --no-cache ca-certificates

# Copy ONLY the binary from builder
COPY --from=builder /app/app .

EXPOSE 8080

CMD ["./app"]
