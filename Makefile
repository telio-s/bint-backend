SQLC_VERSION ?= v1.31.1

.PHONY: sqlc test fmt

sqlc:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate

test:
	go test ./...

fmt:
	gofmt -w cmd internal
