.PHONY: build run dev vet clean docker-build docker-up

build:
	go build -o bin/server ./cmd/server
	go build -o bin/worker ./cmd/worker
	go build -o bin/scheduler ./cmd/scheduler

run:
	go run ./cmd/server

worker:
	go run ./cmd/worker

scheduler:
	go run ./cmd/scheduler

dev:
	go run ./cmd/server &
	go run ./cmd/worker &
	go run ./cmd/scheduler &
	wait

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf bin/

docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f
