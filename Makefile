.PHONY: all gen tidy test lint

all: gen tidy test lint

gen:
	go generate ./...

tidy:
	go mod tidy

test:
	go test -v ./...

lint:
	revive
