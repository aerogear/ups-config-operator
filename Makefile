.PHONY: generate
generate:
	./scripts/generate.sh

.PHONY: build_linux
build_linux:
	env GOOS=linux GOARCH=amd64 go build cmd/server/main.go cmd/server/types.go cmd/server/upsClient.go
