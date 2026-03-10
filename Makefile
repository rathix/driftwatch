.PHONY: build test lint vet vulncheck

build:
	go build -o driftwatch .

test:
	go test ./... -v -race

lint:
	golangci-lint run

vet:
	go vet ./...

vulncheck:
	govulncheck ./...
