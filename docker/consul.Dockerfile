# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o consul ./cmd/consul/main.go

# Run stage
FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/consul .
COPY config/consul.yaml ./config/consul.yaml

EXPOSE 8500
ENTRYPOINT ["./consul"]
