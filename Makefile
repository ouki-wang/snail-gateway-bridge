.PHONY: build clean package serve run-compose-test
VERSION := v1.0.0

build:
	@echo "Compiling source"
	@rm -rf build
	@mkdir build
	@cp snail-gateway-bridge.toml build/
	go build -ldflags "-s -w -X main.version=v1.0.0" -o build/snail-gateway-bridge.exe cmd/snail-gateway-bridge/main.go

clean:
	@echo "Cleaning up workspace"
	@rm -rf build
	@rm -rf dist


snapshot:
	@goreleaser --snapshot

dev-requirements:
	go install golang.org/x/lint/golint
	go install github.com/goreleaser/goreleaser
	go install github.com/goreleaser/nfpm

# shortcuts for development

serve: build
	./build/chirpstack-gateway-bridge

run-compose-test:
	docker-compose run --rm chirpstack-gateway-bridge make test
