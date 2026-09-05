# REST API 1.0.0

По умолчанию сервер запускается на `http://localhost:8080`.

## Health

```http
GET /healthz
```

Ответ:

```json
{"status":"ok"}
```

## Sites

```http
POST /api/sites
Content-Type: application/json

{"name":"Demo site","slug":"demo"}
```

```http
GET /api/sites/{siteId}
```

## Pages

```http
POST /api/sites/{siteId}/pages
Content-Type: application/json

{
  "name": "Home",
  "slug": "home",
  "root": {
    "id": "root",
    "type": "Container",
    "children": [
      {"id":"title","type":"Text","props":{"text":"Hello"}}
    ]
  }
}
```

```http
GET /api/sites/{siteId}/pages
GET /api/pages/{pageId}
GET /api/pages/{pageId}/versions
```

Обновление дерева выпускает новую версию:

```http
PUT /api/pages/{pageId}/tree
Content-Type: application/json

{"root":{"id":"root","type":"Container","children":[]}}
```

## Snapshots

```http
POST /api/sites/{siteId}/snapshots
Content-Type: application/json

{"name":"Initial snapshot"}
```

```http
GET /api/snapshots/{snapshotId}
```

## Content

Контент — текст с вариативностью по языкам (base + overlay translations). Чтение всегда с `?locale` (без него — базовый язык):

```http
GET /api/contents/{contentId}?locale=ru
```

Ответ — смёрженный сервером результат `fields` + `translations[ru]` (поля без перевода остаются из base):

```json
{
  "id": "c1",
  "collectionId": "col.articles",
  "fields": {
    "title": "Привет",
    "description": "Description",
    "image": { "assetId": "asset-1", "variant": "thumb" }
  }
}
```

Серверный фолбэк: для локали без перевода возвращается base.

```http
GET /api/sites/{siteId}/contents?locale=ru
GET /api/sites/{siteId}/contents?collectionId=strings&locale=ru   // UI-строки — тоже контент
```

Батч нескольких контентов под локалью одним запросом:

```http
POST /api/sites/{siteId}/contents/batch?locale=ru
Content-Type: application/json

{"ids":["c1","c2"]}
```

Ответ: `{"c1":{...},"c2":{...}}` (отсутствующие id пропускаются).

Создание/обновление базовых полей (и выпуск версии):

```http
PUT /api/contents/{contentId}
Content-Type: application/json

{"fields":{"title":"Hello"}}
```

### Переводы (overlay)

```http
PUT /api/contents/{contentId}/translations/{locale}
Content-Type: application/json

{"fields":{"title":"Привет"}}
```

```http
GET /api/contents/{contentId}/translations
```

## Ассеты (Assets)

Метаданные (не байты):

```http
GET /api/assets/{assetId}
```

Ответ:

```json
{
  "id": "asset-1",
  "mime": "image/jpeg",
  "size": 2048,
  "variants": [
    { "name": "master", "url": "/api/assets/asset-1/file", "mime": "image/jpeg", "size": 2048, "width": 1600, "height": 900 },
    { "name": "thumb",  "url": "/api/assets/asset-1/file?variant=thumb", "mime": "image/jpeg", "size": 256, "width": 320, "height": 180 }
  ]
}
```

Список и статик-файл:

```http
GET /api/sites/{siteId}/assets
GET /api/assets/{assetId}/file?variant=thumb
```

Байты ассета также доступны по «красивому» пути, если ему назначен route `serveAsset` (см. Runtime-роутинг ниже). Upload ассета — отдельная admin-ручка (не используется рантаймом).

## Формы

```http
GET /api/forms/{formId}
```

Ответ — определение формы: `{ "id", "fields", "validation", "submit" }` (см. `docs/ui-runtime/json-descriptors.md` §10).

Submit — server-only, пишет raw JSON в отдельную таблицу форм:

```http
POST /api/forms/{formId}/submissions
Content-Type: application/json

{"formId":"form.contact","locale":"ru","submittedAt":"...","values":{...}}
```

Ответ: `{"submissionId":"subm_1","status":"ok"}`.

## Runtime-контракт и роутинг (для ui-runtime)

```http
GET /runtime/contract?siteId={siteId}&environment={environment}&versionId={versionId}
```

Ответ включает провайдеры, операции (с poll-расписаниями), эндпоинты, роуты, тему и `defaultLocale`. Дерево отдаётся отдельно:

```http
GET /runtime/tree?siteId={siteId}&environment={environment}
GET /runtime/tokens
GET /runtime/routes
```

Все runtime-ручки привязаны к версии/снапшоту окружения.

Единая таблица роутов (`/runtime/routes`) применяется и к входящим HTTP-запросам (edge):

| Action      | Поведение сервера                                                |
| ----------- | ---------------------------------------------------------------- |
| renderPage  | отдаёт HTML-оболочку Build окружения (shell), контент/дерево — клиентом |
| serveAsset  | отдаёт байты ассета по пути матча: content-type, Cache-Control, ETag |
| redirect    | HTTP-редирект: статус (301/302/307/308) + `Location: target` (+query при keepQuery) |

Пример:

```text
^/robots\.txt$  → serveAsset: asset.robots   (обычный txt, назначен роуту)
^/old$          → redirect: /new (301)
```

## Ошибки

Ошибки имеют единый формат:

```json
{"error":"resource not found"}
```

Используются статусы `400`, `404`, `409` и `500`.
