.PHONY: generate
generate:
	@echo "Generating public APIs..."
	@go run ./cmd/orlop-gen
	@echo "Running controller-gen for public APIs..."
	@controller-gen object:headerFile=hack/boilerplate.go.txt paths="./apis/public/..."
	@echo "Done!"

.PHONY: clean
clean:
	@echo "Cleaning generated files..."
	@rm -rf apis/public
	@echo "Done!"

# PostgreSQL test database configuration
POSTGRES_CONTAINER_NAME ?= orlop-test-postgres
POSTGRES_PORT ?= 5433
POSTGRES_DB ?= orlop_test
POSTGRES_USER ?= orlop
POSTGRES_PASSWORD ?= orlop_test_password
POSTGRES_TEST_URL ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@localhost:$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=disable

.PHONY: postgres-start
postgres-start:
	@echo "Starting PostgreSQL test database..."
	@docker run -d \
		--name $(POSTGRES_CONTAINER_NAME) \
		-e POSTGRES_DB=$(POSTGRES_DB) \
		-e POSTGRES_USER=$(POSTGRES_USER) \
		-e POSTGRES_PASSWORD=$(POSTGRES_PASSWORD) \
		-p $(POSTGRES_PORT):5432 \
		postgres:15-alpine \
		|| (echo "Container may already exist, trying to start it..." && docker start $(POSTGRES_CONTAINER_NAME))
	@echo "Waiting for PostgreSQL to be ready..."
	@for i in $$(seq 1 30); do \
		docker exec $(POSTGRES_CONTAINER_NAME) pg_isready -U $(POSTGRES_USER) > /dev/null 2>&1 && break || sleep 1; \
	done
	@echo "PostgreSQL is ready at localhost:$(POSTGRES_PORT)"

.PHONY: postgres-stop
postgres-stop:
	@echo "Stopping PostgreSQL test database..."
	@docker stop $(POSTGRES_CONTAINER_NAME) 2>/dev/null || true
	@echo "PostgreSQL stopped"

.PHONY: postgres-clean
postgres-clean:
	@echo "Removing PostgreSQL test database..."
	@docker rm -f $(POSTGRES_CONTAINER_NAME) 2>/dev/null || true
	@echo "PostgreSQL removed"

.PHONY: postgres-logs
postgres-logs:
	@docker logs -f $(POSTGRES_CONTAINER_NAME)

.PHONY: postgres-psql
postgres-psql:
	@docker exec -it $(POSTGRES_CONTAINER_NAME) psql -U $(POSTGRES_USER) -d $(POSTGRES_DB)

.PHONY: test
test:
	@echo "Running all tests..."
	@go test ./...
	@echo "Done!"

.PHONY: test-unit
test-unit:
	@echo "Running unit tests (without database)..."
	@go test -short ./...
	@echo "Done!"

.PHONY: test-postgres
test-postgres: postgres-start
	@echo "Running PostgreSQL integration tests..."
	@POSTGRES_TEST_URL=$(POSTGRES_TEST_URL) go test -v ./pkg/apiserver/storage/postgres/...
	@echo "Done!"

.PHONY: test-all
test-all: postgres-start
	@echo "Running all tests with PostgreSQL..."
	@POSTGRES_TEST_URL=$(POSTGRES_TEST_URL) go test -v ./...
	@echo "Done!"

.PHONY: test-coverage
test-coverage: postgres-start
	@echo "Running tests with coverage..."
	@POSTGRES_TEST_URL=$(POSTGRES_TEST_URL) go test -v -coverprofile=coverage.out ./pkg/...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
