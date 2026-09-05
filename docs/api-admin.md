# Admin REST API

Admin API — **редакторский** интерфейс конструктора: CRUD всех управляемых сущностей (сайты, страницы, контент, переводы, ассеты, роуты, формы, снапшоты).

- Сервер: отдельный HTTP-порт `LIAPOLDUS_ADMIN_ADDR` (по умолчанию `:8080`).
- Всё тело JSON, `Content-Type: application/json` (кроме upload байтов ассета — `multipart/form-data`).
- Аутентификация — `Authorization: Bearer <токен>` при заданном `LIAPOLDUS_ADMIN_TOKEN`; пустой токен = открытый доступ (dev).
- CORS: `*`, методы `GET,POST,PUT,DELETE,OPTIONS`, заголовки `Content-Type, Authorization`.

## Сущности

| Сущность | Назначение |
| --- | --- |
| `Site` | изолированный контейнер сайта: `name`, `slug`, `defaultLocale`, `hosts[]` (домены клиентского порта), `createdAt` |
| `Page` | страница с корневым `ComponentNode` и версионированием дерева |
| `Content` | контент: `fields` (base, язык по умолчанию) + `translations[locale]` (overlay-переопределения) |
| `Asset` | метаданные файла (имя, mime, size, `variants`), байты на диске (`LIAPOLDUS_ASSET_DIR`) |
| `Route` | единый маршрут для клиентской навигации и edge: `matcher`, `priority`, `action` |
| `Form` | определение формы (схема/валидация); сабмиты пишутся raw JSON в таблицу |
| `Snapshot` | фиксация актуальных версий страниц сайта |

## Ошибки

Единый формат `{"error":"..."}`; статусы `400` (invalid request), `401` (unauthorized), `404` (not found), `409` (already exists), `405` (метод не поддерживается), `500` (internal).

## Sites

```http
POST /api/sites
Content-Type: application/json

{"name":"Demo","slug":"demo","defaultLocale":"ru","hosts":["demo.example.com","www.demo.example.com"]}
```

Ответ `201`:

```json
{"id":"site_x","name":"Demo","slug":"demo","defaultLocale":"ru","hosts":["demo.example.com","www.demo.example.com"],"createdAt":"..."}
```

```http
GET /api/sites
GET /api/sites/{siteId}
PUT /api/sites/{siteId}        // частичное обновление {name?, slug?, defaultLocale?, hosts?}
DELETE /api/sites/{siteId}     // 204, каскадное удаление всех объектов сайта
```

## Pages

```http
POST /api/sites/{siteId}/pages
Content-Type: application/json

{"name":"Home","slug":"home","root":{"id":"root","type":"Container","children":[]}}
```

Ответ `201` — `Page` (`version: 1`). Обновление дерева выпускает новую версию:

```http
GET /api/sites/{siteId}/pages
GET /api/pages/{pageId}
PUT /api/pages/{pageId}/tree     // {"root":{...}} → version++
GET /api/pages/{pageId}/versions
GET /api/pages/{pageId}/versions/{versionId}
DELETE /api/pages/{pageId}       // 204
```

## Content

Создание (или пересоздание base-полей). `id` опционален (произвольный ключ, например `nav.home` для коллекции `strings`); без него генерируется:

```http
POST /api/sites/{siteId}/contents
Content-Type: application/json

{"collectionId":"col.articles","id":"a1","fields":{"title":"Hello","image":{"assetId":"asset_x","variant":"thumb"}}}
```

```http
GET /api/sites/{siteId}/contents               // список (соответствие {id, collectionId, fields})
GET /api/sites/{siteId}/contents?collectionId=strings
GET /api/sites/{siteId}/contents/{contentId}   // base + translations (для редактора)
PUT /api/sites/{siteId}/contents/{contentId}   // обновить base {fields}
DELETE /api/sites/{siteId}/contents/{contentId} // 204
```

### Переводы (overlay)

```http
PUT /api/sites/{siteId}/contents/{contentId}/translations/{locale}
Content-Type: application/json

{"fields":{"title":"Привет"}}
```

```http
GET /api/sites/{siteId}/contents/{contentId}/translations    // {ru:{...}, en:{...}}
DELETE /api/sites/{siteId}/contents/{contentId}/translations/{locale} // 204
```

Правила слияния для клиентского чтения: `fields = base.fields`, затем поля из `translations[locale]` поверх; перевод, покрывающий не все поля, «дополняет» base (см. `docs/api-client.md`).

## Ассеты

Загрузка байтов (multipart, поле `file`, опционально `name`). Ассеты immutable: повторная загрузка создаёт новый id.

```http
POST /api/sites/{siteId}/assets
Content-Type: multipart/form-data

file=@hero.jpg
```

Ответ `201` — метаданные:

```json
{
  "id":"asset_x","siteId":"site_x","name":"hero.jpg","mime":"image/jpeg","size":2048,
  "variants":[{"name":"master","url":"/api/assets/asset_x/file","mime":"image/jpeg","size":2048}],
  "etag":"<sha1>","createdAt":"..."
}
```

```http
GET /api/sites/{siteId}/assets          // список метаданных
GET /api/assets/{assetId}               // метаданные
GET /api/assets/{assetId}/file          // байты (Content-Type, ETag, Cache-Control)
DELETE /api/assets/{assetId}            // 204 (удаляет и байты)
```

`variants[].url` — относительный путь клиентского порта (тот же, что отдаётся `client` API).

## Роуты

Единый маршрут. `matcher` — полное регулярное выражение пути; `priority` (больше = раньше); при равенстве — порядок создания.

```http
POST /api/sites/{siteId}/routes
Content-Type: application/json

{"matcher":"^/articles/([0-9]+)$","priority":10,"action":{"type":"renderPage","pageId":"page_x"}}
```

Варианты action:

| type | поля | поведение |
| --- | --- | --- |
| `renderPage` | `pageId` | клиентская отрисовка; на edge — заглушка 404 (до Builds) |
| `serveAsset` | `assetId` | отдать байты ассета (content-type/mime, ETag, Cache-Control) |
| `redirect` | `target`, `status?` (301/302/307/308, default 301), `keepQuery?` (bool, default false) | HTTP-редирект; в `target` поддерживаются группы regex `$1`..`$9` |

```http
GET /api/sites/{siteId}/routes
GET /api/sites/{siteId}/routes/{routeId}
PUT /api/sites/{siteId}/routes/{routeId}   // обновить {matcher?, priority?, action?}
DELETE /api/sites/{siteId}/routes/{routeId} // 204
```

Валидация: непустой валидный regex; `redirect.status ∈ {301,302,307,308}`; непустой `target`; `pageId`/`assetId` непустые для своих типов.

## Формы

Определение формы — JSON по схеме `docs/ui-runtime/json-descriptors.md` §10 (поля, валидация, конфигурация отправки).

```http
POST /api/sites/{siteId}/forms
Content-Type: application/json

{"name":"Contact","definition":{"id":"form.contact","fields":[{"name":"email","type":"email","required":true}],"submit":{"endpoint":"form.contact"}}}
```

```http
GET /api/sites/{siteId}/forms
GET /api/sites/{siteId}/forms/{formId}
PUT /api/sites/{siteId}/forms/{formId}      // {name?, definition?}
DELETE /api/sites/{siteId}/forms/{formId}   // 204
```

Сабмиты (чтение — только через admin):

```http
GET /api/sites/{siteId}/forms/{formId}/submissions
```

Ответ: `[{"id":"subm_x","formId":"form.contact","siteId":"site_x","payload":{...},"createdAt":"..."}]`.

## Snapshot

```http
POST /api/sites/{siteId}/snapshots
Content-Type: application/json

{"name":"Release 1"}
```

```http
GET /api/sites/{siteId}/snapshots
GET /api/snapshots/{snapshotId}
DELETE /api/snapshots/{snapshotId}   // 204
```

## Аутентификация

- Если `LIAPOLDUS_ADMIN_TOKEN` пуст — admin открыт (режим разработки).
- Иначе каждый запрос admin требует `Authorization: Bearer <токен>` (сравнение constant-time); без него — `401 {"error":"unauthorized"}`.