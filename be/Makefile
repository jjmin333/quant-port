.PHONY: run run-dev build test tidy docker-up docker-down

BIN := bin/api
ifeq ($(shell go env GOOS),windows)
BIN := bin/api.exe
endif

# Preferred: build once, run fixed binary (avoids Windows Firewall prompts from go run temp exe).
run: build
	./$(BIN)

# Fast iteration during Go-only development (may trigger firewall on Windows).
run-dev:
	go run ./cmd/api

build:
	go build -o $(BIN) ./cmd/api

test:
	go test ./...

tidy:
	go mod tidy

docker-up:
	docker compose -f ../docker-compose.yml up -d --build

docker-down:
	docker compose -f ../docker-compose.yml down
