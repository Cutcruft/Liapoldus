# Liapoldus — make targets
# `make dev` = postgres (docker) + backend (air, hot-reload) + admin (vite).
# Backend env comes from backend/.env (see backend/.env.example).

BACKEND_ENV := backend/.env

.PHONY: build dev dev-native test lint clean docker-up docker-down

# Build all artifacts
build:
	cd admin && npm install && npm run build
	cd backend && go build -o ./bin/liapoldus ./cmd/server

# Dev mode: postgres in docker, backend with hot-reload, admin vite
dev:
	docker compose up -d postgres
	set -a; . $(BACKEND_ENV); set +a; cd backend && air -c .air.toml & \
	cd admin && npm run dev

# Dev mode native (no docker for postgres)
dev-native:
	set -a; . $(BACKEND_ENV); set +a; go run ./backend/cmd/server & \
	cd admin && npm run dev

# Run all tests
test:
	cd backend && go test ./...
	npm test

# Lint/typecheck all
lint:
	cd backend && go vet ./...
	npm run typecheck
	cd admin && npx eslint src/ 2>/dev/null || true

# Remove generated artifacts
clean:
	rm -rf backend/tmp backend/bin backend/build admin/dist data/
	docker compose down -v

# Full stack in docker (postgres + backend + admin)
docker-up:
	docker compose up -d --build

docker-down:
	docker compose down -v