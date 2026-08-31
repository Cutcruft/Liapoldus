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

## Ошибки

Ошибки имеют единый формат:

```json
{"error":"resource not found"}
```

Используются статусы `400`, `404`, `409` и `500`.
