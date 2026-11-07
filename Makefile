# Convenience shortcuts for Linux/macOS/WSL2. Docker Compose itself is
# the portable interface, including Windows PowerShell.
DC = docker compose
LAB = docker compose -f compose.yaml -f compose.lab.yaml --profile lab
ADMIN = docker compose -f compose.yaml -f compose.control.yaml --profile admin

.PHONY: help build init check render lab-build lab-up lab-down lab-logs admin-build admin-up admin-down admin-logs admin-ps diag test lint

help:
	@echo "mrkvpn (Phase 1 alpha, not production-ready)"
	@echo "  make build       Build tools image (no gateway)"
	@echo "  make init        Create local gateway key and JSON (idempotent)"
	@echo "  make check       Validate local gateway config"
	@echo "  make render      Render a new wg0.conf (never overwrite)"
	@echo "  make lab-build   Build optional Linux lab gateway"
	@echo "  make lab-up      Start Linux lab gateway, UDP 51820"
	@echo "  make lab-down    Stop lab gateway; keep keys and configuration"
	@echo "  make lab-logs    Follow lab logs"
	@echo "  make diag        Open isolated diagnostics toolbox"
	@echo "  make admin-up    Build and start localhost-only admin panel"
	@echo "  make admin-down  Stop panel; keep its persistent data"
	@echo "  make admin-logs  Follow control-plane logs (never print secrets)"
	@echo "  make admin-ps    Show control-plane status"
	@echo "  make test        Run Go tests inside the builder image (local Go optional)"

build:
	$(DC) --profile tools build init cli

init:
	$(DC) --profile tools run --rm init

check:
	$(DC) --profile tools run --rm cli gateway check -config /config/gateway.json

render:
	$(DC) --profile tools run --rm cli gateway render -config /config/gateway.json -out /output/wg0.conf

lab-build:
	$(LAB) build gateway

lab-up:
	@test -f .local/output/wg0.conf || (echo "Missing .local/output/wg0.conf; run make render first" >&2; exit 1)
	$(LAB) up -d --build gateway

lab-down:
	$(LAB) down

lab-logs:
	$(LAB) logs --follow gateway

admin-build:
	$(ADMIN) build control

admin-up:
	$(ADMIN) up -d --build control

admin-down:
	$(ADMIN) down

admin-logs:
	$(ADMIN) logs --follow control

admin-ps:
	$(ADMIN) ps

diag:
	$(DC) --profile diagnostics run --rm diag

test:
	docker build --target build -t mrkvpn/go-build:local .
	docker run --rm --entrypoint /bin/sh mrkvpn/go-build:local -ec 'go test -race ./... && go vet ./...'

lint:
	docker build --target build -t mrkvpn/go-build:local .
	docker run --rm --entrypoint /bin/sh mrkvpn/go-build:local -ec 'test -z "$$(gofmt -l core cmd internal)"'
