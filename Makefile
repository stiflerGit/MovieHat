.PHONY: help run test fmt tidy proto-build proto-generate proto-lint proto-format docker-build docker-run clean

ADDR ?= 0.0.0.0:8080
DB_PATH ?= ./moviehat.db
AUTH_SECRET ?= dev-secret
IMAGE ?= moviehat:dev

help:
	@printf "Targets:\n"
	@printf "  make run            Run the server locally\n"
	@printf "  make test           Run all tests\n"
	@printf "  make fmt            Format Go code\n"
	@printf "  make tidy           Tidy root and tools modules\n"
	@printf "  make proto-generate Regenerate protobuf/connect code\n"
	@printf "  make proto-lint     Lint proto files\n"
	@printf "  make docker-build   Build the Docker image\n"
	@printf "  make docker-run     Run the Docker image locally\n"

run:
	ADDR=$(ADDR) \
	DB_PATH=$(DB_PATH) \
	AUTH_SECRET=$(AUTH_SECRET) \
	BOOTSTRAP_ENABLED=true \
	BOOTSTRAP_EMAIL="admin@moviehat.com" \
	BOOTSTRAP_PASSWORD="moviehat" \
	    go run ./cmd/moviehat

test:
	go test ./...

fmt:
	gofmt -w $$(find . -path './vendor' -prune -o -name '*.go' -print)

tidy:
	go mod tidy
	go -C tools mod tidy

proto-build:
	go -C tools tool buf build --config ../proto/buf.yaml ../proto

proto-generate: proto-build proto-format proto-lint
	go -C tools tool buf generate --config ../proto/buf.yaml --template ../proto/buf.gen.yaml ../proto

proto-lint:
	go -C tools tool buf lint --config ../proto/buf.yaml ../proto

proto-format:
	go -C tools tool buf format --config ../proto/buf.yaml -w ../proto

docker-build:
	docker build -f build/docker/Dockerfile -t $(IMAGE) .

docker-run:
	docker run --rm \
		-p 8080:8080 \
		--env-file=.env.example \
		-v moviehat-data:/data \
		$(IMAGE)

clean:
	rm -rf .data
