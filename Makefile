.PHONY: build test lint fmt vet tidy clean

BINARY := meridian
CMD     := ./cmd/meridian

build:
	go build -o bin/$(BINARY) $(CMD)

test:
	go test ./...

test-verbose:
	go test -v -race ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf bin/

# Run the server locally
run:
	go run $(CMD) --addr=:8080

# Run all checks (CI equivalent)
check: fmt vet test
