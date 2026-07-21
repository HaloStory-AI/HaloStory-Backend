# Stage 1: Build
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/scheduler ./cmd/scheduler

# Stage 2: Runtime
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata curl

WORKDIR /app

COPY --from=builder /app/server .
COPY --from=builder /app/worker .
COPY --from=builder /app/scheduler .

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:3000/api/ping || exit 1

RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

CMD ["./server"]
