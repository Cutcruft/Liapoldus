# REST API 1.0.0

По умолчанию сервер запускается на `http://localhost:8080`.

## Management transport

Управляющий API можно запустить в одном из двух режимов:

```bash
LIAPOLDUS_MANAGEMENT_TRANSPORT=rest  # по умолчанию
LIAPOLDUS_MANAGEMENT_TRANSPORT=grpc
```

Оба transport используют одни и те же доменные сервисы и операции. Меняется только внешний протокол. REST и gRPC не запускаются одновременно на одном адресе.

Protobuf-контракт находится в [proto/liapoldus/management/v1/management.proto](../proto/liapoldus/management/v1/management.proto), сгенерированный Go-код — в `backend/gen`.

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

В gRPC эти ошибки преобразуются в стандартные коды:

```text
400 → InvalidArgument
404 → NotFound
409 → AlreadyExists
500 → Internal
```
