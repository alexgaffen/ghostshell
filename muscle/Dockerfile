# muscle/Dockerfile

# Stage 1: Build the Go binary
FROM golang:1.23-alpine AS builder
WORKDIR /app

# Copy dependency files
COPY go.mod ./
# (If you had a go.sum, you'd copy it here too)
RUN go mod download

# Copy source code
COPY . .
# Build the binary named "server"
RUN go build -o server main.go

# Stage 2: The lightweight runner
FROM alpine:latest
WORKDIR /root/

# Copy the binary from Stage 1
COPY --from=builder /app/server .
# Copy the host key (Required for SSH!)
COPY --from=builder /app/hostkey . 

CMD ["./server"]