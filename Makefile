GO_DIR := backend
AGENT_DIR := agent
WEB_DIR := web

.PHONY: help tidy test vet fmt proto certs agent-test agent-vet agent-build web-install web-lint web-typecheck web-build compose-up compose-down compose-logs db-migrate

help:
	@printf '%s\n' \
		'Available targets:' \
		'  make tidy          Resolve and tidy Go modules' \
		'  make test          Run backend Go tests' \
		'  make vet           Run backend go vet' \
		'  make fmt           Format backend and agent Go source files' \
		'  make proto         Generate Go gRPC sources from the versioned schema' \
		'  make certs         Generate a local development TLS certificate' \
		'  make agent-test    Run native agent tests with the race detector' \
		'  make agent-vet     Run native agent static analysis' \
		'  make agent-build   Build the native monitoring agent' \
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
	gofmt -w $$(find $(GO_DIR) $(AGENT_DIR) -name '*.go' -type f)

proto:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2
	sh scripts/generate-proto.sh

certs:
	mkdir -p .data/certs
	openssl req -x509 -newkey rsa:2048 -nodes -keyout .data/certs/server.key -out .data/certs/server.crt -days 365 -subj "/CN=localhost" -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"
	chmod 750 .data/certs
	chmod 644 .data/certs/server.crt
	# Local self-signed development key only: readable by the non-root API container.
	# Never reuse this certificate/key on a public or production host.
	chmod 644 .data/certs/server.key

agent-test:
	cd $(AGENT_DIR) && go test -race ./...

agent-vet:
	cd $(AGENT_DIR) && go vet ./...

agent-build:
	cd $(AGENT_DIR) && go build -trimpath ./cmd/monitor-agent

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
