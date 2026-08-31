# Liapoldus backend

## Запуск

Из Ubuntu под WSL, из корня репозитория:

```bash
cd /mnt/c/Users/sh_ub/CutCraft/Liapoldus
go run ./cmd/server
```

Адрес можно изменить переменной `LIAPOLDUS_ADDR`, например `LIAPOLDUS_ADDR=:9090`.

## Проверка

```bash
go test ./...
curl http://localhost:8080/healthz
```

Для полного smoke-теста API из WSL, пока сервер запущен в другом окне:

```bash
bash scripts/smoke.sh
```

Сейчас persistence работает в памяти. Перезапуск процесса очищает данные намеренно: это позволяет разрабатывать API и frontend до подключения PostgreSQL.
