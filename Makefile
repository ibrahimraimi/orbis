.PHONY: build test lint docker-build docker-up clean

build:
	go build -o bin/consul ./cmd/consul/main.go
	go build -o bin/gateway ./cmd/gateway/main.go
	go build -o bin/orbisctl ./cmd/orbisctl/main.go

test:
	go test ./internal/... -v

lint:
	golangci-lint run

docker-build:
	docker-compose build

docker-up:
	docker-compose up -d

clean:
	rm -rf bin/
	rm -f consul.db
