# Client REST API

Client API — **публичный** runtime-интерфейс сайта: контент, ассеты, формы, runtime-контракт и единый edge-роутинг («красивые» пути сайта).

- Сервер: отдельный HTTP-порт `LIAPOLDUS_CLIENT_ADDR` (по умолчанию `:18080`).
- Без аутентификации; CORS `*`.
- Выбор сайта: **по `Host`** из `site.hosts` (точное совпадение или дик `*.domain`), иначе — сайт по `LIAPOLDUS_CLIENT_DEFAULT_SLUG`, иначе `404`.
- Ручки, помеченные `{siteId}`, принимают явный siteId в пути (для программного доступа/ui-runtime-паритета) и обслуживают любой существующий сайт.
- В этом раунде client отдаёт **актуальные данные напрямую** (pipeline Environments/Build/Publication — отдельный этап). `environment`/`versionId` в контракте принимаются, но не фильтруют.
- Ошибки: `{"error":"..."}`, статусы `400/404/500` (+`405`).

## Выбор сайта и записи

| Параметр | Значение |
| --- | --- |
| `Host` | матчится по `site.hosts` (точное или `*.domain`) |
| `LIAPOLDUS_CLIENT_DEFAULT_SLUG` | фолбэк-сайт, если Host не совпал |

## Контент (base + overlay, серверный фолбэк)

```http
GET /api/contents/{contentId}?locale=ru
```

Ответ — слияние на сервере: `fields = base.fields`, затем поля `translations[ru]` поверх. Если перевод для локали отсутствует — возвращается base (готовый текст).

```json
{
  "id":"a1","siteId":"site_x","collectionId":"col.articles","locale":"ru",
  "fields":{"title":"Привет","description":"Description","image":{"assetId":"asset_x","variant":"thumb"}}
}
```

```http
GET /api/sites/{siteId}/contents?collectionId=strings&locale=ru
GET /api/contents?collectionId=strings&locale=ru      // сайт по Host
```

UI-строки — контент коллекции `strings`; ид = ключ.

Батч одним запросом (потерянные id пропускаются):

```http
POST /api/sites/{siteId}/contents/batch?locale=ru
Content-Type: application/json

{"ids":["a1","a2"]}
```

Ответ: `{"a1":{...},"a2":{...}}`.

## Ассеты

Метаданные (и link-список для кэширования):

```http
GET /api/assets/{assetId}
GET /api/sites/{siteId}/assets
```

Ответ — метаданные с относительными путями байтов:

```json
{
  "id":"asset_x","siteId":"site_x","name":"hero.jpg","mime":"image/jpeg","size":2048,
  "variants":[{"name":"master","url":"/api/assets/asset_x/file","mime":"image/jpeg","size":2048}],
  "etag":"<sha1>"
}
```

Байты:

```http
GET /api/assets/{assetId}/file?variant=master
```

Ответ отдаёт `Content-Type` по mime, `ETag`, `Cache-Control: public, max-age=31536000, immutable` и `Accept-Ranges`. Неизвестный variant → `400`; нет байтов → `404`.

## Формы

Определение формы (для клиентской валидации и рендера):

```http
GET /api/forms/{formId}
```

Submit — raw JSON в отдельную таблицу сабмитов:

```http
POST /api/forms/{formId}/submissions
Content-Type: application/json

{"formId":"form.contact","locale":"ru","submittedAt":"2026-01-01T00:00:00Z","values":{"email":"a@b.c"}}
```

Ответ `201`:

```json
{"submissionId":"subm_x","status":"ok"}
```

Сервер проверяет: `formId` в теле совпадает с путём; значения валидируются по определению формы (`required`, `minLength`, `type: email`) → `400 {"error":"..."}` при несоответствии.

## Runtime-контракт

```http
GET /runtime/contract?siteId={siteId}&environment={environment}&versionId={versionId}
```

Ответ (текущее состояние сайта):

```json
{
  "siteId":"site_x",
  "defaultLocale":"ru",
  "routes":[
    {"id":"route_1","matcher":"^/articles/([0-9]+)$","priority":10,"action":{"type":"renderPage","pageId":"page_x"}},
    {"id":"route_2","matcher":"^/old$","priority":0,"action":{"type":"redirect","target":"/new","status":301,"keepQuery":false}},
    {"id":"route_3","matcher":"^/robots\\.txt$","priority":0,"action":{"type":"serveAsset","assetId":"asset_robots"}}
  ],
  "forms":[{"id":"form.contact","fields":[...],"submit":{...}}],
  "operations":[],
  "endpoints":[],
  "environments":[],
  "theme":null
}
```

Сокращённый список маршрутов (без остального):

```http
GET /runtime/routes?siteId={siteId}
```

## Edge-роутинг (единая таблица маршрутов)

Любой `GET`/`HEAD` на клиентском порте, не начинающийся с `/api/` и `/runtime/`, матчится по маршрутам выбранного по Host сайта (сортировка: `priority` по убыванию, затем порядок создания):

| Action | Поведение |
| --- | --- |
| `serveAsset` | байты ассета: `Content-Type` по mime, `ETag`, `Cache-Control: public, max-age=31536000, immutable` |
| `redirect` | статус (`301/302/307/308`, default 301) + `Location: target` (группы `$1..$9` из regex; `?query` копируется при `keepQuery`) |
| `renderPage` | `404 {"error":"render page not implemented"}` — заглушка до этапа Builds |

Примеры:

```text
^/robots\.txt$             → serveAsset: asset_robots      (обычный txt, назначен роуту)
^/old$                     → redirect: /new (301)
^/legacy/([0-9]+)$         → redirect: /modern/$1 (308, keepQuery)
^/articles/([0-9]+)$       → renderPage: page_x            (404 placeholder на edge)
```

Несовпадение с маршрутами → `404`.