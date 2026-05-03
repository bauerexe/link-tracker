COVERAGE_FILE ?= coverage.out

# Get all directories in cmd/ as available modules
MODULES := $(notdir $(wildcard cmd/*))

# Help target - display usage information
.PHONY: help
help:
	@echo "Available commands:"
	@echo "  \033[36mmake build\033[0m - Build all modules ($(MODULES))"
	@$(foreach mod,$(MODULES),echo "  \033[36mmake build_$(mod)\033[0m - Build $(mod) module";)
	@echo "  \033[36mmake test\033[0m - Run all tests"

.PHONY: build
build:
	@echo "Building all modules: $(MODULES)"
	@mkdir -p bin
	@$(foreach mod,$(MODULES),echo "Building module: $(mod)"; go build -o ./bin/$(mod) ./cmd/$(mod);)

# Convenience targets for building individual modules
.PHONY: $(addprefix build_,$(MODULES))
$(addprefix build_,$(MODULES)):
	@modulename=$(subst build_,,$@); \
	echo "Building module: $$modulename"; \
	mkdir -p bin; \
	go build -o ./bin/$$modulename ./cmd/$$modulename

## test: run all tests
.PHONY: test
test:
	@go test -coverpkg='github.com/es-debug/backend-academy-2024-go-template/...' --race -count=1 -coverprofile='$(COVERAGE_FILE)' ./...
	@go tool cover -func='$(COVERAGE_FILE)' | grep ^total | tr -s '\t'

tg_bot_run:
	make build && ./bin/bot

scrapper_run:
	make build && ./bin/scrapper

POSTGRES_SERVICE ?= postgres
POSTGRES_USER ?= postgres
POSTGRES_DB ?= link_tracker

BASE_URL ?= http://scrapper:8080
VUS ?= 8
STAGE_DURATION ?= 5m
SUMMARY_STATS ?= avg,min,med,p(50),p(90),p(95),p(99),max

COMPOSE_NOCACHE = docker compose -f docker-compose.yml -f docker-compose.nocache.yml
COMPOSE_VALKEY = docker compose -f docker-compose.yml -f docker-compose.valkey.yml

.PHONY: load_infra
load_infra:
	docker compose up -d --wait postgres valkey-node-1 zookeeper kafka-1 kafka-2 kafka-3 schema-registry
	-docker compose rm -f -s flyway init-schema
	docker compose up -d --no-deps --force-recreate flyway init-schema
	docker compose wait flyway init-schema
	docker compose logs --no-color flyway init-schema

.PHONY: load_seed_100k
load_seed_100k:
	docker compose cp load/sql/seed_100k.sql $(POSTGRES_SERVICE):/tmp/seed_100k.sql
	docker compose exec -T $(POSTGRES_SERVICE) psql -v ON_ERROR_STOP=1 -U $(POSTGRES_USER) -d $(POSTGRES_DB) -f /tmp/seed_100k.sql

.PHONY: clear_valkey
clear_valkey:
	docker compose exec -T valkey-node-1 valkey-cli FLUSHALL

.PHONY: prepare_load_results
prepare_load_results:
	powershell -NoProfile -Command "New-Item -ItemType Directory -Force load/results | Out-Null"

.PHONY: load_test_nocache
load_test_nocache: prepare_load_results
	docker compose -f docker-compose.yml -f docker-compose.nocache.yml run --rm \
		-e BASE_URL="$(BASE_URL)" \
		-e VUS="$(VUS)" \
		-e STAGE_DURATION="$(STAGE_DURATION)" \
		k6 run \
		--summary-trend-stats "$(SUMMARY_STATS)" \
		--summary-export /results/nocache.json \
		/scripts/list_load.js

.PHONY: load_test_valkey
load_test_valkey: prepare_load_results
	docker compose -f docker-compose.yml -f docker-compose.valkey.yml run --rm \
		-e BASE_URL="$(BASE_URL)" \
		-e VUS="$(VUS)" \
		-e STAGE_DURATION="$(STAGE_DURATION)" \
		k6 run \
		--summary-trend-stats "$(SUMMARY_STATS)" \
		--summary-export /results/valkey.json \
		/scripts/list_load.js

.PHONY: load_nocache
load_nocache:
	$(COMPOSE_NOCACHE) up -d --wait --force-recreate scrapper
	$(MAKE) load_test_nocache

.PHONY: load_valkey
load_valkey:
	$(COMPOSE_VALKEY) up -d --wait --force-recreate scrapper
	$(MAKE) clear_valkey
	$(MAKE) load_test_valkey


.PHONY: load_compare
load_compare: load_infra load_seed_100k load_nocache load_valkey