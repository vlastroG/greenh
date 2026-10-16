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

# multi-arch builds; images are not loaded locally or pushed
image:
	docker buildx build --platform linux/amd64,linux/arm64 --target waterer -t greenh/waterer .
	docker buildx build --platform linux/amd64,linux/arm64 --target simulator -t greenh/simulator .
