BINARY := pobear
CMD    := ./cmd/core

.PHONY: build run test lint fmt fmt-check tidy vet ci clean

build:
	go build -o bin/$(BINARY) $(CMD)

run: build
	./bin/$(BINARY)

test:
	go test ./...

lint:
	golangci-lint run ./...

fmt:
	golangci-lint fmt ./...

fmt-check:
	golangci-lint fmt -d ./...

tidy:
	go mod tidy

vet:
	go vet ./...

ci: vet lint test build

clean:
	rm -rf bin/
