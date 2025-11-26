# Build stage
FROM golang:1.23 AS builder
WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN go build -o order-status-service ./cmd/server

# Runtime stage
FROM debian:bookworm-slim
WORKDIR /app

COPY --from=builder /app/order-status-service /app/
EXPOSE 8080

CMD ["./order-status-service"]
