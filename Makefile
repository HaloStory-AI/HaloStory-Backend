.PHONY: dev dev-win dev-win-worker build build-server build-worker build-scheduler build-all test clean docker-build docker-up docker-down docker-logs docker-worker-up docker-worker-down docker-worker-logs docker-worker-build

dev:
	air

dev-win:
	go run ./cmd/server/main.go

dev-win-worker:
	go run ./cmd/worker/main.go

dev-win-air:
	air -c .air.windows.toml

build-server:
	go build -o ./tmp/server ./cmd/server

build-worker:
	go build -o ./tmp/worker ./cmd/worker

build-scheduler:
	go build -o ./tmp/scheduler ./cmd/scheduler

build-all: build-server build-worker build-scheduler

build: build-all

test:
	go test -v -count=1 ./...

test-coverage:
	go test -v -count=1 -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean:
	rm -rf tmp/ coverage.out coverage.html

docker-build:
	docker compose build

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

docker-worker-build:
	docker compose -f docker-compose.worker.yml build

docker-worker-up:
	docker compose -f docker-compose.worker.yml up -d --build

docker-worker-down:
	docker compose -f docker-compose.worker.yml down

docker-worker-logs:
	docker compose -f docker-compose.worker.yml logs -f
