BINARY := pobear
CMD    := ./cmd/pobear

.PHONY: build run test test-install lint-install lint fmt fmt-check tidy vet ci clean

build:
	go build -o bin/$(BINARY) $(CMD)

run: build
	./bin/$(BINARY)

test:
	go test ./...

test-install:
	sh test/install_test.sh install.sh

lint-install:
	shellcheck install.sh test/install_test.sh

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

ci: vet lint lint-install test test-install build

clean:
	rm -rf bin/
