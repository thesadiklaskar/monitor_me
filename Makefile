GO_DIR := backend
WEB_DIR := web

.PHONY: help tidy test vet fmt web-install web-lint web-typecheck web-build compose-up compose-down compose-logs db-migrate

help:
	@printf '%s\n' \
		'Available targets:' \
		'  make tidy          Resolve and tidy Go modules' \
		'  make test          Run Go unit tests' \
		'  make vet           Run go vet' \
		'  make fmt           Format Go source files' \
		'  make web-install   Install frontend dependencies' \
		'  make web-lint      Run frontend ESLint checks' \
		'  make web-typecheck Run the TypeScript compiler' \
		'  make web-build     Build the Next.js application' \
		'  make compose-up    Build and start the local platform stack' \
		'  make compose-down  Stop the local platform stack' \
		'  make compose-logs  Follow local platform logs' \
		'  make db-migrate    Run the one-shot database migration service'

tidy:
	cd $(GO_DIR) && go mod tidy

test:
	cd $(GO_DIR) && go test ./...

vet:
	cd $(GO_DIR) && go vet ./...

fmt:
	cd $(GO_DIR) && gofmt -w $$(find . -name '*.go' -type f)

web-install:
	cd $(WEB_DIR) && npm install

web-lint:
	cd $(WEB_DIR) && npm run lint

web-typecheck:
	cd $(WEB_DIR) && npm run typecheck

web-build:
	cd $(WEB_DIR) && npm run build

compose-up:
	docker compose up --build -d

compose-down:
	docker compose down

compose-logs:
	docker compose logs -f --tail=100

db-migrate:
	docker compose run --rm migrate
