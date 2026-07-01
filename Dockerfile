# Build stage
FROM golang:1.27-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /build/server ./cmd/server
RUN CGO_ENABLED=0 go build -o /build/worker ./cmd/worker
RUN CGO_ENABLED=0 go build -o /build/scheduler ./cmd/scheduler

# Server stage
FROM alpine:3.19 AS server
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /build/server .
COPY --from=builder /build/internal/services/templates ./internal/services/templates
EXPOSE 18000
CMD ["./server"]

# Worker stage
FROM alpine:3.19 AS worker
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /build/worker .
CMD ["./worker"]

# Scheduler stage
FROM alpine:3.19 AS scheduler
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /build/scheduler .
CMD ["./scheduler"]
