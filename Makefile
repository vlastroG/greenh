.PHONY: generate test build up e2e image

generate:
	go generate ./...

test:
	go test ./...

build:
	CGO_ENABLED=0 go build -o bin/ ./cmd/...

up:
	docker compose up --build

e2e:
	go test -tags e2e ./e2e/...

# image target is defined in Task 14
image:
	@echo "not implemented yet"; exit 1
