# Liapoldus backend

## Запуск

Из Ubuntu под WSL, из корня репозитория:

```bash
cd /mnt/c/Users/sh_ub/CutCraft/Liapoldus
export PATH="/usr/local/go/bin:$(go env GOPATH)/bin:${PATH}"
go run ./cmd/server
```

Адрес можно изменить переменной `LIAPOLDUS_ADDR`, например `LIAPOLDUS_ADDR=:9090`.

## Проверка

```bash
go test ./...
go vet ./...
curl http://localhost:8080/healthz
```

Полная локальная проверка качества кода:

```bash
bash scripts/lint.sh
```

Используется pinned-конфигурация в корневом `.golangci.yml`. Скрипт проверяет форматирование, `go vet` и набор статических линтеров.

GitHub Actions CI выполняет те же проверки и собирает серверный бинарник. Описание находится в [docs/ci.md](../docs/ci.md).

Для полного smoke-теста API из WSL, пока сервер запущен в другом окне:

```bash
bash scripts/smoke.sh
```

Сейчас persistence работает в памяти. Перезапуск процесса очищает данные намеренно: это позволяет разрабатывать API и frontend до подключения PostgreSQL.
