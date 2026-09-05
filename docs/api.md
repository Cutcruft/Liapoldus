# REST API — навигация

Liapoldus разделён на два публичных API (см. `README.md` §26/§29):

| API | Назначение | Порт | Авторизация | Документация |
| --- | --- | --- | --- | --- |
| **Admin** | редактор: CRUD сайтов, страниц, контента, ассетов, роутов, форм, снапшотов | `LIAPOLDUS_ADMIN_ADDR` (`:8080`) | Bearer (`LIAPOLDUS_ADMIN_TOKEN`) | [`docs/api-admin.md`](api-admin.md) |
| **Client** | публичный runtime сайта: контент, ассеты, формы, runtime-контракт, edge-роутинг | `LIAPOLDUS_CLIENT_ADDR` (`:18080`) | без | [`docs/api-client.md`](api-client.md) |

## Общие соглашения

- Тело запросов/ответов — JSON, `Content-Type: application/json` (кроме upload ассета — multipart).
- Ошибки — единый формат `{"error":"..."}`. Статусы: `400` (invalid request), `401` (unauthorized, admin), `404` (not found), `409` (already exists), `405`, `500`.
- Поля: camelCase; времена — RFC3339 (`createdAt`, `updatedAt`).
- Идентификаторы: префикс `<entity>_<hex>` через `domain.NewID`, либо произвольный ключ там, где это разрешено (например `id` контента `nav.home`).

## Health

Оба сервера отдают `GET /healthz` → `{"status":"ok"}`.

## Логика, общая для обоих API

- **Контент и локализация — одно**: `fields` (base) + overlay `translations[locale]`; клиентское чтение всегда со слиянием `?locale`, серверный фолбэк на base. UI-строки — контент коллекции `strings`.
- **Ассеты**: в контенте — ссылки `{ assetId, variant }` (не URL); метаданные из `GET /api/assets/{id}`, байты — `GET /api/assets/{id}/file`. Выбор варианта — явно в контенте.
- **Единый роутинг**: одна таблица маршрутов сайта применяется и в клиенте (React, `ui-runtime`), и на edge (serveAsset/redirect; renderPage — заглушка до Builds).

Полные описания эндпоинтов — в [`docs/api-admin.md`](api-admin.md) и [`docs/api-client.md`](api-client.md).