.PHONY: test test-coverage test-unit test-integration lint clean deps example-warehouse example-warehouse-docker help

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

# Установка всех инструментов кодогенерации
install-codegen-tools: install-potter-gen install-protoc-gen-potter
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

# Вывод справки
help:
	@echo "Available commands:"
	@echo "  make test                    - Run all tests"
	@echo "  make test-coverage           - Run tests with coverage report"
	@echo "  make test-unit               - Run unit tests only"
	@echo "  make test-integration        - Run integration tests"
	@echo "  make lint                    - Run linter"
	@echo "  make clean                   - Clean build artifacts"
	@echo "  make deps                    - Install dependencies"
	@echo "  make example-warehouse      - Run warehouse example"
	@echo "  make example-warehouse-docker - Start warehouse infrastructure"
	@echo "  make install-codegen-tools   - Install potter-gen and protoc-gen-potter"
	@echo "  make test-codegen            - Test code generator"
	@echo "  make example-codegen         - Run codegen example"
	@echo "  make clean-codegen           - Clean generated code"
	@echo "  make help                    - Show this help message"


