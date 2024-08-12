PROJECT_NAME=htmlpdfviewer

up:
	@if [ ! -f .env ]; then \
        cp .env.example .env; \
    fi
	docker compose -p $(PROJECT_NAME) up -d

ps:
	docker compose -p $(PROJECT_NAME) ps

exec:
	docker compose -p $(PROJECT_NAME) exec app sh

down:
	docker compose -p $(PROJECT_NAME) down

fresh:
	@if [ ! -f .env ]; then \
        cp .env.example .env; \
    fi
	docker compose -p $(PROJECT_NAME) down --remove-orphans
	docker compose -p $(PROJECT_NAME) build --no-cache
	docker compose -p $(PROJECT_NAME) up -d --build -V

logs:
	docker compose -p $(PROJECT_NAME) logs -f

test:
	go test -v -race -cover -count=1 -failfast ./...

lint:
	golangci-lint run -v