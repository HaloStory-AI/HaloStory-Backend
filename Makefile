.PHONY: dev build test clean docker-up docker-down docker-build

dev:
	air

build:
	go build -o ./tmp/main ./cmd/server

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
