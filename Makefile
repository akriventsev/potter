.PHONY: test test-coverage test-unit test-integration lint clean deps example-warehouse example-warehouse-docker help example-eventsourcing-basic example-eventsourcing-docker example-eventsourcing-migrate test-eventsourcing benchmark-eventsourcing test-all example-saga-order example-saga-order-test example-saga-warehouse test-saga test-saga-integration benchmark-saga install-potter-migrate

# Тестирование
test:
	@echo "Running tests..."
	@go test -v ./...

# Тестирование с покрытием
test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Только unit тесты
test-unit:
	@echo "Running unit tests..."
	@go test -v -short ./...

# Integration тесты (если есть)
test-integration:
	@echo "Running integration tests..."
	@go test -v -tags=integration ./...

# Линтинг
lint:
	@echo "Running linter..."
	@golangci-lint run ./...

# Очистка артефактов сборки
clean:
	@echo "Cleaning build artifacts..."
	@rm -f server *.exe *.dll *.so *.dylib *.test *.out coverage.out coverage.html
	@rm -rf bin/ dist/ tmp/ temp/

# Установка зависимостей
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

# Запуск warehouse примера
example-warehouse:
	@echo "Running warehouse example..."
	@cd examples/warehouse && make run

# Запуск инфраструктуры для warehouse
example-warehouse-docker:
	@echo "Starting warehouse infrastructure..."
	@cd examples/warehouse && make docker-up

# Установка potter-gen CLI
install-potter-gen:
	@echo "Installing potter-gen..."
	@go install ./cmd/potter-gen
	@echo "potter-gen installed successfully"

# Установка protoc-gen-potter плагина
install-protoc-gen-potter:
	@echo "Installing protoc-gen-potter..."
	@go install ./cmd/protoc-gen-potter
	@echo "protoc-gen-potter installed successfully"

# Установка potter-migrate CLI
install-potter-migrate:
	@echo "Installing potter-migrate..."
	@go install ./cmd/potter-migrate
	@echo "potter-migrate installed successfully"

# Установка goose CLI
install-goose: ## Установить goose CLI
	@echo "Installing goose..."
	@go install github.com/pressly/goose/v3/cmd/goose@latest
	@echo "Goose installed successfully"

# Установка всех инструментов кодогенерации
install-codegen-tools: install-potter-gen install-protoc-gen-potter install-potter-migrate install-goose
	@echo "All codegen tools installed"

# Тестирование кодогенератора
test-codegen:
	@echo "Testing code generator..."
	@go test -v ./framework/codegen/...

# Запуск примера кодогенерации
example-codegen:
	@echo "Running codegen example..."
	@cd examples/codegen && potter-gen init --proto simple-service.proto --module simple-service --output ./generated
	@echo "Code generated in examples/codegen/generated/"

# Очистка сгенерированного кода
clean-codegen:
	@echo "Cleaning generated code..."
	@rm -rf examples/codegen/generated/

# Event Sourcing Examples
example-eventsourcing-basic:
	@echo "Running Event Sourcing basic example..."
	@cd examples/eventsourcing-basic && make run

example-eventsourcing-docker:
	@echo "Starting Event Sourcing example infrastructure..."
	@cd examples/eventsourcing-basic && make docker-up

example-eventsourcing-migrate:
	@echo "Running Event Sourcing migrations..."
	@cd examples/eventsourcing-basic && make migrate

test-eventsourcing:
	@echo "Running Event Sourcing tests..."
	@go test -v ./framework/eventsourcing/...

benchmark-eventsourcing:
	@echo "Running Event Sourcing benchmarks..."
	@go test -bench=. -benchmem ./framework/eventsourcing/...

# Все тесты включая Event Sourcing
test-all: test test-eventsourcing test-saga
	@echo "All tests completed"

# Saga Pattern examples
example-saga-order:
	@echo "Running Order Saga example..."
	@cd examples/saga-order && make docker-up && make migrate && make run

example-saga-order-test:
	@echo "Testing Order Saga example..."
	@cd examples/saga-order && make test

example-saga-warehouse:
	@echo "Running Warehouse Saga integration example..."
	@cd examples/saga-warehouse-integration && make docker-up && make migrate && make run

test-saga:
	@echo "Testing Saga module..."
	@go test -v -race -coverprofile=coverage-saga.out ./framework/saga/...

test-saga-integration:
	@echo "Running Saga integration tests..."
	@go test -v -tags=integration ./framework/saga/...

benchmark-saga:
	@echo "Running Saga benchmarks..."
	@go test -bench=. -benchmem ./framework/saga/...

# Вывод справки
help:
	@echo "Available commands:"
	@echo "  make test                    - Run all tests"
	@echo "  make test-coverage           - Run tests with coverage report"
	@echo "  make test-unit               - Run unit tests only"
	@echo "  make test-integration        - Run integration tests"
	@echo "  make test-all                - Run all tests including Event Sourcing"
	@echo "  make lint                    - Run linter"
	@echo "  make clean                   - Clean build artifacts"
	@echo "  make deps                    - Install dependencies"
	@echo "  make example-warehouse      - Run warehouse example"
	@echo "  make example-warehouse-docker - Start warehouse infrastructure"
	@echo "  make example-eventsourcing-basic - Run Event Sourcing basic example"
	@echo "  make example-eventsourcing-docker - Start Event Sourcing infrastructure"
	@echo "  make example-eventsourcing-migrate - Run Event Sourcing migrations"
	@echo "  make test-eventsourcing      - Run Event Sourcing tests"
	@echo "  make benchmark-eventsourcing - Run Event Sourcing benchmarks"
	@echo "  make install-codegen-tools   - Install potter-gen, protoc-gen-potter, potter-migrate and goose"
	@echo "  make install-potter-migrate   - Install potter-migrate CLI"
	@echo "  make install-goose            - Install goose CLI"
	@echo "  make test-codegen            - Test code generator"
	@echo "  make example-codegen         - Run codegen example"
	@echo "  make clean-codegen           - Clean generated code"
	@echo "  make example-saga-order     - Run Order Saga example"
	@echo "  make example-saga-order-test - Test Order Saga example"
	@echo "  make example-saga-warehouse - Run Warehouse Saga integration example"
	@echo "  make test-saga               - Test Saga module"
	@echo "  make test-saga-integration   - Run Saga integration tests"
	@echo "  make benchmark-saga          - Run Saga benchmarks"
	@echo "  make help                    - Show this help message"


