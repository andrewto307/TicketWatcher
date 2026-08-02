.PHONY: run build test tidy clean

# Run the API locally. Loads .env via godotenv.
run:
	go run ./cmd/api

# Build a small static binary to bin/api.
build:
	CGO_ENABLED=0 go build -o bin/api ./cmd/api

# Run all tests.
test:
	go test ./...

# Resolve and lock dependencies (creates/updates go.sum). Run this first.
tidy:
	go mod tidy

clean:
	rm -rf bin
