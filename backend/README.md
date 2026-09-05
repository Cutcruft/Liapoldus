# Liapoldus backend

## Запуск

Из Ubuntu под WSL, из директории `backend` репозитория:

```bash
cd /mnt/c/Users/sh_ub/CutCraft/Liapoldus/backend
export PATH="/usr/local/go/bin:$(go env GOPATH)/bin:${PATH}"
go run ./cmd/server
```

Сервер запускает два HTTP-сервиса: admin (редакторский API) и client (публичный runtime). Конфигурация приходит только из переменных окружения (`LIAPOLDUS_*`) и не имеет дефолтов — образец полного набора в `backend/.env.example`. Ключевые переменные: `LIAPOLDUS_ADMIN_ADDR`, `LIAPOLDUS_CLIENT_ADDR` (адреса слушателей), `LIAPOLDUS_STORAGE` (`memory` | `postgres`) с `LIAPOLDUS_DATABASE_URL`, `LIAPOLDUS_ASSET_DIR` (каталог файлов ассетов), `LIAPOLDUS_ADMIN_TOKEN` (Bearer для admin; пустой = открыто) и `LIAPOLDUS_CLIENT_DEFAULT_SLUG` (запасной сайт по умолчанию).

## Проверка

```bash
go test ./...
go vet ./...
curl http://localhost:8080/healthz
curl http://localhost:18080/healthz
```

Полная локальная проверка качества кода:

```bash
bash scripts/lint.sh
```

Используется pinned-конфигурация в `.golangci.yml`. Скрипт проверяет форматирование, `go vet` и набор статических линтеров.

GitHub Actions CI выполняет те же проверки и собирает серверный бинарник. Описание находится в [docs/ci.md](../docs/ci.md).

Для полного smoke-теста API из WSL, пока сервер запущен в другом окне:

```bash
bash scripts/test.sh
```

Для PostgreSQL через Docker Compose см. [docs/postgres.md](../docs/postgres.md).

Режим `memory` доступен для unit-тестов и локальной разработки. В этом режиме перезапуск процесса очищает данные; режим `postgres` хранит данные в базе.
