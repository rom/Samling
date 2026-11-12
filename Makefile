# Samling build helpers. Run "make help" for the targets.

BIN      ?= bin/samling
DEV_PORT ?= 8080
DEV_DATA ?= ./data
GOFLAGS_BUILD = -trimpath -ldflags "-s -w"

.PHONY: help build test test-postgres check-i18n shellcheck e2e manual-screenshots vet fmt run-dev clean tag release publish

help: ## show this help
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-14s %s\n", $$1, $$2}'

build: ## build a static binary into bin/samling
	CGO_ENABLED=0 go build $(GOFLAGS_BUILD) -o $(BIN) ./cmd/samling

test: ## run all tests (SQLite) and the translation check
	go test ./...
	@if command -v node >/dev/null 2>&1; then node scripts/check-i18n.mjs; else echo "node not found: skipping translation check"; fi

check-i18n: ## verify that the sv/en translations are complete
	node scripts/check-i18n.mjs

shellcheck: ## lint the shell scripts (needs shellcheck)
	shellcheck scripts/*.sh tests/e2e/run.sh

e2e: ## browser smoke test (needs Node.js + playwright)
	tests/e2e/run.sh

manual-screenshots: ## regenerate the screenshots of docs/manual.md
	E2E_SCRIPT=scripts/manual-screenshots.js tests/e2e/run.sh --port 8098

test-postgres: ## run the API tests against PostgreSQL (set SAMLING_TEST_PG_DSN)
	@test -n "$(SAMLING_TEST_PG_DSN)" || { echo "set SAMLING_TEST_PG_DSN, e.g. postgres://postgres@127.0.0.1:5432/postgres?sslmode=disable"; exit 1; }
	SAMLING_TEST_PG_DSN="$(SAMLING_TEST_PG_DSN)" go test -count=1 ./internal/app

vet: ## go vet + gofmt check
	go vet ./...
	@test -z "$$(gofmt -l cmd internal web)" || { echo "gofmt needed:"; gofmt -l cmd internal web; exit 1; }

fmt: ## format the Go sources
	gofmt -w cmd internal web

run-dev: build ## run locally over plain HTTP on DEV_PORT (default 8080), data in ./data
	SAMLING_INSECURE_HTTP=1 $(BIN) serve --host 127.0.0.1 --port $(DEV_PORT) --data-dir $(DEV_DATA)

tag: ## create the release tag VERSION=vX.Y.Z on the current commit (must be on the release branch)
	scripts/release.sh tag "$(VERSION)"

release: ## build the release archives and checksums into dist/ (VERSION optional)
	scripts/release.sh build "$(VERSION)"

publish: ## push the tag VERSION=vX.Y.Z; GitHub Actions then tests, builds and publishes the release
	scripts/release.sh publish "$(VERSION)"

clean: ## remove build output
	rm -rf bin dist
