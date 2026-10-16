.PHONY: generate test build up e2e image

generate:
	go generate ./...
	cd web && npm run gen

test:
	go test ./...
	cd web && npm ci && npm run typecheck && npm test -- --run

build:
	CGO_ENABLED=0 go build -o bin/ ./cmd/...

up:
	docker compose up --build

e2e:
	go test -tags e2e ./e2e/... -count=1

# multi-arch builds; images are not loaded locally or pushed
image:
	docker buildx build --platform linux/amd64,linux/arm64 --target waterer -t greenh/waterer .
	docker buildx build --platform linux/amd64,linux/arm64 --target simulator -t greenh/simulator .
