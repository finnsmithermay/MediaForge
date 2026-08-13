.PHONY: run dev build clean localstack test lint

# Run with default config
run:
	go run cmd/server/main.go

# Run pointed at LocalStack
dev:
	AWS_ENDPOINT=http://localhost:4566 go run cmd/server/main.go

# Build the binary
build:
	go build -o bin/mediaforge cmd/server/main.go

# Clean build artifacts
clean:
	rm -rf bin/

# Start local AWS (LocalStack) and create resources
localstack:
	docker-compose up -d
	@echo "Waiting for LocalStack..."
	@sleep 3
	@echo "LocalStack ready"

# Stop local AWS
localstack-down:
	docker-compose down -v

# Run tests
test:
	go test ./... -v

# Run linter
lint:
	go vet ./...
