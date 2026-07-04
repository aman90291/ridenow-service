.PHONY: run build test tidy fmt vet

run:
	go run ./cmd/ridenow

build:
	go build -o bin/ridenow ./cmd/ridenow

test:
	go test -race ./...

tidy:
	go mod tidy

fmt:
	gofmt -w .

vet:
	go vet ./...
