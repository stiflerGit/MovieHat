ifneq (,$(wildcard ./.env))
    include .env
    export
endif

.PHONY: help run test fmt tidy proto-build proto-generate proto-lint proto-format oas-generate docker-build docker-run lint clean

##@ Development

run: ## Run the server locally
	go run ./cmd/moviehat

test: ## Run all tests
	go test ./...

fmt: ## Format Go code
	gofmt -w $$(find . -path './vendor' -prune -o -name '*.go' -print)

tidy: ## Tidy root and tools modules
	go mod tidy
	go -C tools mod tidy

##@ Protobuf

proto-build: ## Build proto files with Buf
	go -C tools tool buf build --config ../proto/buf.yaml ../proto

proto-generate: proto-build proto-format proto-lint
proto-generate: ## Regenerate protobuf/connect code
	go -C tools tool buf generate --config ../proto/buf.yaml --template ../proto/buf.gen.yaml ../proto

proto-lint: ## Lint proto files
	go -C tools tool buf lint --config ../proto/buf.yaml ../proto

proto-format: ## Format proto files
	go -C tools tool buf format --config ../proto/buf.yaml -w ../proto

##@ OpenAPI

oas-generate: ## Regenerate TMDB client from third_party/tmdb/api.json
	go -C tools tool oapi-codegen -package client -o ../gen/tmdb/client.gen.go -generate "models,client"  ../third_party/tmdb/api.json

##@ Docker

docker-build: ## Build the Docker image
	docker build -f build/docker/Dockerfile -t $(IMAGE) .

docker-run: ## Run the Docker image locally
	docker run --rm \
		-p 8080:8080 \
		--env-file=.env.example \
		-v moviehat-data:/data \
		$(IMAGE)

##@ Quality

lint: ## Run golangci-lint via Docker
	# it is not recommended to install golangci-lint as go tool. That's why docker
	docker run --rm -t -v $$(pwd):/app -w /app \
	    --user $$(id -u):$$(id -g) \
	    -v $$(go env GOCACHE):/.cache/go-build -e GOCACHE=/.cache/go-build \
	    -v $$(go env GOMODCACHE):/.cache/mod -e GOMODCACHE=/.cache/mod \
	    -v ~/.cache/golangci-lint:/.cache/golangci-lint -e GOLANGCI_LINT_CACHE=/.cache/golangci-lint \
	    golangci/golangci-lint:v2.13.2 golangci-lint run

clean: ## Remove local data
	rm -rf .data

##@ Help

help: ## Display this help screen
	@echo
	@echo "Usage:"
	@echo
	@sed -n 's/^\([A-Za-z0-9_.-]*\):.*## \(.*\)$$/\t\1: \2/p' Makefile | sort | column -t -s ':'
	@echo
