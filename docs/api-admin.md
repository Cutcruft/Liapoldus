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

## Компоненты

Каталог компонентов редактора. `GET /api/sites/{siteId}/components` возвращает **объединённый каталог**: сначала платформенные builtin-компоненты (Container/Text/Image/Button), затем определённые на сайте. Каждая запись — `{type, label, container, schema}` (schema — JSON-Schema draft-07-подмножество для генератора форм).

```http
GET /api/sites/{siteId}/components
Accept: application/json

[
  {"type":"Container","label":"Контейнер","container":true,"schema":{"type":"object","properties":{...}}},
  {"type":"Text","label":"Текст","container":false,"schema":{...}},
  {"type":"MySection","label":"Моя секция","container":false,"schema":{...}}   // определённый на сайте
]
```

- `container: true` — разрешено вкладывать детей (у текущих builtin — только `Container`; пользовательские всегда `container: false`).
- Полный CRUD определений:

```http
POST   /api/sites/{siteId}/components            // create {id,name,kind,source,schema,metadata}
GET    /api/sites/{siteId}/components            // каталог (builtin + определённые)
GET    /api/sites/{siteId}/components/{id}       // определение
PUT    /api/sites/{siteId}/components/{id}       // update {name,schema,metadata}
DELETE /api/sites/{siteId}/components/{id}       // 204
```

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

## Build

Публикация снапшота в окружение. Сборка **синхронная**: `POST …/builds` возвращает готовый Build (`queued→building→ready|failed`); повторная публикация той же пары snapshot+environment — no-op и возвращает существующий ready-Build. Окружение — одно из `development|production` (staging нет).

```http
POST /api/sites/{siteId}/builds
Content-Type: application/json

{"snapshotId":"snap1","environment":"production"}
```

```http
GET /api/sites/{siteId}/builds     // список Build сайта, по возрастанию created_at
GET /api/builds/{buildId}
```

Build:

```json
{"id":"b1","siteId":"s1","snapshotId":"snap1","environment":"production",
 "status":"ready","log":["..."],"artifactDir":"build/s1/production/snap1",
 "createdAt":"…","startedAt":"…","finishedAt":"…"}
```

Артефакты раздаются публично на `/build/…` (без Bearer). Текущий на prod = новейшая `ready`-сборка `production`; «откат» — повторный `POST …/builds` для старого снапшота.

### Живой статус сборок (WS)

```http
GET /api/builds/ws?siteId={siteId}&token={adminToken}
```

WebSocket событий жизненного цикла сборки. Событие:

```json
{"siteId":"s1","environment":"production","snapshotId":"snap1",
 "status":"building","artifactDir":"…","updatedAt":"…"}
```

События приходят на каждом переходе: `building`, `ready`, `failed` (с `error`). Поле `updatedAt` уникально идентифицирует событие — повторная доставка того же события игнорируется.

- Браузерные `WebSocket` не умеют слать HTTP-заголовки, поэтому **токен передаётся query-параметром** `?token=` (валидируется constant-time до апгрейда, как BearerAuth). Пустой `LIAPOLDUS_ADMIN_TOKEN` = открыто.
- Роут смонтирован на внешнем mux (до BearerAuth) и работает только если сервер собран с build-событиями.
- Фильтрация по `siteId` на стороне сервера; поля `environment`/`snapshotId`/`artifactDir`/`error` — только для контекста UI.

## Аутентификация

- Если `LIAPOLDUS_ADMIN_TOKEN` пуст — admin открыт (режим разработки).
- Иначе каждый запрос admin требует `Authorization: Bearer <токен>` (сравнение constant-time); без него — `401 {"error":"unauthorized"}`.