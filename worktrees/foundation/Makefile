.PHONY: build run test lint clean

build:
	go build -o purge .

run:
	go run .

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -f purge
