.PHONY: build web-build web-dev web-clean go-build snapshot release

VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILDTIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  := -X main.version=$(VERSION) \
            -X main.releaseRepo=ranwei/mneme \
            -X main.releaseChannel=github

build: web-build go-build

snapshot:
	goreleaser release --snapshot --clean

release:
	goreleaser release --clean

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
