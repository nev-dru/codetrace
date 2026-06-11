.PHONY: build test install lint

build:
	go build -o bin/codetrace ./cmd/codetrace

test:
	go test ./...

install:
	go install ./cmd/codetrace

lint:
	go vet ./...
