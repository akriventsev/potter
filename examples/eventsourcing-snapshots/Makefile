.PHONY: up run test clean help

help:
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

up:
	docker-compose up -d
	@sleep 5

run:
	go run cmd/server/main.go

test:
	go test -v ./...

clean:
	go clean -cache -testcache

