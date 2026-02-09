.PHONY: proto clean build test run-server run-agent migrate-up migrate-down

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

run-server:
	go run ./cmd/server

run-agent:
	go run ./cmd/agent

migrate-up:
	migrate -path migrations -database "$(MAESTRO_DB_URL)" up

migrate-down:
	migrate -path migrations -database "$(MAESTRO_DB_URL)" down 1
