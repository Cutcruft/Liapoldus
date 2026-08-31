# PostgreSQL

## Локальный запуск

PostgreSQL и backend поднимаются через Docker Compose:

```bash
docker compose up --build
```

Compose запускает PostgreSQL и backend. Backend ждёт health-check базы, проверяет соединение и выполняет миграцию `backend/store/migrations/001_initial.sql`.

По умолчанию Compose запускает management API в режиме gRPC. Для REST можно изменить `LIAPOLDUS_MANAGEMENT_TRANSPORT` на `rest` в `docker-compose.yml`.

## Проверка в WSL Ubuntu

Из PowerShell можно выполнять команды внутри WSL так:

```bash
wsl.exe -d Ubuntu -- bash -lc "cd /mnt/c/Users/sh_ub/CutCraft/Liapoldus && docker compose up -d --build"
```

Проверка gRPC management API:

```bash
wsl.exe -d Ubuntu -- bash -lc "cd /mnt/c/Users/sh_ub/CutCraft/Liapoldus && GRPCURL_BIN=/home/sh_ub/go/bin/grpcurl bash scripts/test-grpc.sh"
```

Smoke-тест создает сайт и страницу с компонентным деревом, обновляет дерево, проверяет две версии страницы и создает snapshot. Данные остаются в volume `liapoldus-postgres`, поэтому перезапуск приложения не удаляет их:

```bash
wsl.exe -d Ubuntu -- docker compose -f /mnt/c/Users/sh_ub/CutCraft/Liapoldus/docker-compose.yml restart app
```

Состояние контейнеров и записи в базе можно посмотреть так:

```bash
wsl.exe -d Ubuntu -- docker compose -f /mnt/c/Users/sh_ub/CutCraft/Liapoldus/docker-compose.yml ps
wsl.exe -d Ubuntu -- docker compose -f /mnt/c/Users/sh_ub/CutCraft/Liapoldus/docker-compose.yml exec -T postgres psql -U liapoldus -d liapoldus -c "SELECT count(*) FROM sites;"
```

## Конфигурация

```bash
LIAPOLDUS_STORAGE=postgres
LIAPOLDUS_DATABASE_URL=postgres://user:password@host:5432/database?sslmode=disable
```

Для запуска без базы данных можно явно выбрать временное хранилище:

```bash
LIAPOLDUS_STORAGE=memory go run ./cmd/server
```

Memory storage предназначен только для тестов и локальной разработки. Данные в нём теряются после остановки процесса.

## Схема

Миграция создаёт таблицы `sites`, `pages`, `page_versions`, `snapshots` и `snapshot_pages`.

Дерево компонентов хранится в колонках `pages.root` и `page_versions.root` типа `JSONB`.
