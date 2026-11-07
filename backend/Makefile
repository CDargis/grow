.PHONY: build clean test tidy

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o dist/bootstrap ./cmd/api

build-local:
	go build -o dist/api-local ./cmd/api

run:
	LOCAL=1 \
	PLANTS_TABLE=grow-plants \
	ENVIRONMENTS_TABLE=grow-environments \
	LOGS_TABLE=grow-logs \
	LOGS_DATE_GSI=user-date-index \
	LOGS_LOGTYPE_DATE_GSI=user-logtype-date-index \
	SETTINGS_TABLE=grow-settings \
	PRODUCTS_TABLE=grow-products \
	OIL_LOTS_TABLE=grow-oil-lots \
	OIL_BATCHES_TABLE=grow-oil-batches \
	MEDIA_BUCKET=grow-media \
	USER_ID=default \
	go run ./cmd/api

clean:
	rm -rf dist/

test:
	go test ./...

tidy:
	go mod tidy
