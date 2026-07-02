.PHONY: build install run test

build:
	go build -buildvcs=false -o hardcover-goodreads .

install:
	go install -buildvcs=false .

run:
	go run -buildvcs=false .

test:
	go test ./...
