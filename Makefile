.PHONY: build test lint clean

BIN := gha

build:
	go build -o $(BIN) .

test:
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...

lint:
	golangci-lint run ./...

clean:
	rm -f $(BIN) coverage.txt
