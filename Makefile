.PHONY: build web-build web-dev web-clean go-build

build: web-build go-build

web-build:
	cd web && pnpm install --frozen-lockfile && pnpm build
	rm -rf pkg/dashboard/dist
	mkdir -p pkg/dashboard/dist
	cp -R web/dist/. pkg/dashboard/dist/

go-build:
	go build -o bin/mneme ./cmd

web-dev:
	cd web && pnpm install --frozen-lockfile && pnpm dev

web-clean:
	rm -rf web/dist web/node_modules pkg/dashboard/dist
	mkdir -p web/dist pkg/dashboard/dist
	cp web/dist.placeholder.html web/dist/index.html
	cp web/dist.placeholder.html pkg/dashboard/dist/index.html
