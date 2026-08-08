DB_URL=postgres://chatuser:chatpass@localhost:5432/chatdb?sslmode=disable

MIGRATIONS_DIR=./migrations

create:
	@read -p "Migration name: " name; \
	goose -dir $(MIGRATIONS_DIR) create $$name sql

up:
	goose -dir $(MIGRATIONS_DIR) postgres "$(DB_URL)" up

down:
	goose -dir $(MIGRATIONS_DIR) postgres "$(DB_URL)" down

status:
	goose -dir $(MIGRATIONS_DIR) postgres "$(DB_URL)" status

reset:
	goose -dir $(MIGRATIONS_DIR) postgres "$(DB_URL)" reset

version:
	goose -dir $(MIGRATIONS_DIR) postgres "$(DB_URL)" version

swagger:
	swag init -g cmd/server/main.go --parseInternal
