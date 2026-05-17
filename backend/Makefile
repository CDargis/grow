.PHONY: build clean test tidy

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o dist/bootstrap ./cmd/api

build-local:
	go build -o dist/api-local ./cmd/api

clean:
	rm -rf dist/

test:
	go test ./...

tidy:
	go mod tidy
