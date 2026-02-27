.PHONY: all build run test test-unit clean docs-view certs

# Генерация RSA ключей для JWT
certs:
	chmod +x scripts/gen_certs.sh
	./scripts/gen_certs.sh

# Запуск всего проекта в Docker
run: certs
	docker-compose up --build

# Остановка проекта
stop:
	docker-compose down

# Запуск всех тестов (включая интеграционные в Testcontainers)
test:
	go test -v -race ./...

# Запуск только Unit-тестов (быстро)
test-unit:
	go test -v -short ./...

# Загрузка зависимостей
deps:
	go mod tidy
	go mod download

# Линтер (если установлен golangci-lint)
lint:
	golangci-lint run

# Просмотр документации API в консоли (Zero Dependency)
docs-view:
	curl -s http://localhost:8080/api/v1/docs | python3 -m json.tool || curl -s http://localhost:8080/api/v1/docs
