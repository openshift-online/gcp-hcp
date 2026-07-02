.PHONY: build test lint docker-build update-golden

BINARY_NAME=hyperfleet-adapters-go
IMAGE=quay.io/cveiga/hyperfleet-hc-adapter-go

build:
	go build -o bin/$(BINARY_NAME) ./cmd/...

test:
	go test ./...

lint:
	golangci-lint run ./...

docker-build:
	docker build -t $(IMAGE):latest .

update-golden:
	go test ./... -update-golden
