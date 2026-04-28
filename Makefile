.PHONY: build web-build web-dev web-clean go-build

build: web-build go-build

web-build:
	cd web && pnpm install --frozen-lockfile && pnpm build

go-build:
	go build -o bin/mneme ./cmd

web-dev:
	cd web && pnpm install --frozen-lockfile && pnpm dev

web-clean:
	rm -rf web/dist web/node_modules
	mkdir -p web/dist
	cp web/dist.placeholder.html web/dist/index.html
