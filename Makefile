.DEFAULT_GOAL := help

DOCKER_IMAGE := dcache
VERSION := latest

.PHONY: help
help:
	@echo "DCache Development Commands:"
	@echo ""
	@echo "  make dev        - Start development server with hot reload"
	@echo "  make test       - Run tests"
	@echo "  make bench      - Run benchmarks"
	@echo "  make build      - Build production image"
	@echo "  make clean      - Clean up containers and volumes"
	@echo ""

.PHONY: dev
dev:
	docker compose up dev

.PHONY: test
test:
	docker compose run --rm dev go test -v -race ./...

.PHONY: bench
bench:
	docker compose run --rm dev go test -bench=. -benchmem ./...

.PHONY: build
build:
	docker build -t $(DOCKER_IMAGE):$(VERSION) .

.PHONY: clean
clean:
	docker compose down -v
	rm -rf tmp/

.PHONY: logs
logs:
	docker compose logs -f dev

.PHONY: shell
shell:
	docker compose exec dev sh
