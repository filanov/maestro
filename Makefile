.PHONY: all proto clean build test run-server run-agent migrate-up migrate-down lint fmt fmt-check

all: proto build lint

proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/agent/v1/agent.proto

clean:
	rm -f proto/agent/v1/*.pb.go
	rm -f bin/server bin/agent

build: proto
	go build -o bin/server ./cmd/server
	go build -o bin/agent ./cmd/agent

test:
	go test -v ./...

lint:
	@echo "Running go vet..."
	@go vet ./...
	@echo "Checking formatting..."
	@test -z "$$(gofmt -l . | grep -v 'proto/.*\.pb\.go' | tee /dev/stderr)" || (echo "Files not formatted, run 'make fmt'" && exit 1)
	@echo "Running golangci-lint (if available)..."
	@if command -v golangci-lint > /dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)"; \
	fi

fmt:
	@echo "Formatting code..."
	@gofmt -w -l $$(find . -name '*.go' | grep -v '.pb.go')

fmt-check:
	@echo "Checking formatting..."
	@test -z "$$(gofmt -l . | grep -v 'proto/.*\.pb\.go')" || (echo "Files not formatted:" && gofmt -l . | grep -v 'proto/.*\.pb\.go' && exit 1)

run-server:
	go run ./cmd/server

run-agent:
	go run ./cmd/agent

migrate-up:
	migrate -path migrations -database "$(MAESTRO_DB_URL)" up

migrate-down:
	migrate -path migrations -database "$(MAESTRO_DB_URL)" down 1
