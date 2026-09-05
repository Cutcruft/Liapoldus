# ui-runtime — JSON-дескрипторы

Flat JSON, которым backend описывает контракт рантайма. Publishing не генерирует код — только эти объекты. ui-runtime парсит их и строит типизированный API (`docs/ui-runtime/spec.md`).

Все дескрипторы имеют общий формат ошибок валидации (`DescriptorValidationError`).

---

## 1. Провайдер

```jsonc
{
  "id": "cms.http",                 // уникально в рамках сайта
  "protocol": "http",               // "http" | "ws" | "sse" | "graphql"
  "baseUrl": "https://api.example.com",
  "defaults": {
    "headers": { "Accept": "application/json" },   // без секретов
    "timeoutMs": 5000
  }
}
```

- `baseUrl` может отсутствовать → тогда это «родной» backend (builtin). Используется базовый URL хоста.
- Для `graphql` допускается `baseUrl` + опциональный `wsUrl` (subscription).

---

## 2. Операция

```jsonc
{
  "id": "articles.list",
  "type": "query",                  // "query" | "mutation"
  "providerId": "cms.http",
  "method": "GET",                  // для http
  "path": "/api/articles",
  "params": [                       // path/query параметры
    { "name": "page", "in": "query", "type": "number" }
  ],
  "input": {                        // schema входных данных (ничего не говорит о протоколе)
    "articleId": { "type": "string", "required": true }
  },
  "output": {                       // schema результата
    "items": { "type": "array", "items": "object" }
  },
  "cache": { "policy": "ttl", "ttlSeconds": 60 },   // "disabled" | "ttl" | "immutable" | "custom"
  "scope": "public",               // "public" | "server"
  "poll": {                        // опционально: SHORT POLLING по cron
    "schedule": "*/5 * * * * *"    // 6-полевый cron (sec min hour DOM month DOW)
  },
  "push": false                    // опционально: явный push (ws/sse). false = по умолчанию поллинг
}
```

Поля по протоколу:

| protocol | дополнительные поля                                          |
| -------- | ------------------------------------------------------------- |
| `http`   | `method`, `path`, `params[]`                                  |
| `ws`     | `eventName` (сообщение в socket)                              |
| `sse`    | `eventName` (имя EventSource-события; default `message`)       |
| `graphql`| `query` (строковое GraphQL-операция), `variablesMode: "input"\|"params"` |

`scope: "server"` — серверный endpoint; исполняется только через `ApiClient.callEndpoint` и НЕ доступен из компонента напрямую (проверка в early runtime).

---

## 3. Endpoint (server-only)

```jsonc
{
  "id": "contact.submit",           // уникально
  "operationId": "contact.submit.op",  // ссылка на server-operation
  "path": "/api/endpoints/contact",    // путь на нашем backend
  "method": "POST",
  "input": { ... },
  "output": { ... },
  "auth": "none"                    // будет дополнено политиками
}
```

Endpoint доступен только через `ApiClient.callEndpoint`; не является универсальным прокси.

---

## 4. Route

Единая таблица роутов для клиента (React) и сервера (edge). Actions:

```jsonc
// renderPage — SPA-страница
{
  "id": "route.article",
  "matcher": "^/articles/([0-9]+)$",      // полное regex, якоря обязательны
  "priority": 10,                          // больше = раньше
  "action": { "type": "renderPage", "pageId": "page.article" }
}

// serveAsset — статический файл (Asset) по «красивому» пути,
// напр. положили robots.txt и назначили ему роут
{
  "id": "route.robots",
  "matcher": "^/robots\\.txt$",
  "priority": 100,
  "action": { "type": "serveAsset", "assetId": "asset.robots" }
}

// redirect — HTTP-редирект (клиент просто переходит к target)
{
  "id": "route.old",
  "matcher": "^/old$",
  "priority": 5,
  "action": { "type": "redirect", "target": "/new", "status": 301 }
}

// redirect с сохранением query и перемещением группы regex:
{
  "id": "route.legacy",
  "matcher": "^/legacy/(.*)$",
  "priority": 5,
  "action": { "type": "redirect", "target": "/modern/$1", "status": 308, "keepQuery": true }
}
```

| Action      | Формат                         | Сервер (edge)                                          | Клиент (React)                  |
| ----------- | ------------------------------ | ------------------------------------------------------ | ------------------------------- |
| renderPage  | `{ type, pageId }`             | отдаёт HTML-оболочку Build                             | SPA-рендер pageId               |
| serveAsset  | `{ type, assetId }`            | отдаёт байты ассета (content-type/cache/ETag)          | полный переход (`location.assign`) |
| redirect    | `{ type, target, status?, keepQuery? }` | 301/302/307/308 + `Location` (+query) | клиентский переход к target     |

---

## 5. Дерево (TreeDeclaration)

```jsonc
{
  "snapshotId": "snap_abc",
  "versionId": "ver_123",
  "root": {
    "instanceId": "i1",
    "definitionId": "layout.main",
    "props": { "variant": "wide" },
    "bindings": [],
    "children": [
      {
        "instanceId": "i2",
        "definitionId": "article.card",
        "props": {},
        "bindings": [
          {
            "property": "title",
            "source": { "type": "content", "contentId": "c1", "path": "title" }
          }
        ],
        "children": []
      }
    ]
  }
}
```

- `definitionId` → зарегистрированный ComponentDefinition (см. `spec.md` §15);
- дерево описывает только структуру, данные — отдельно (bindings/подписки).

---

## 6. Binding sources

```jsonc
// content
{ "type": "content", "contentId": "c1", "path": "fields.title" }

// параметры роута (захваченные группы regex / query)
{ "type": "routeParam", "name": "articleId" }
{ "type": "routeQuery", "name": "page" }

// результат операции (Query/Mutation по data-path)
{ "type": "operation", "operationId": "articles.list", "path": "items.[].title" }

// данные формы
{ "type": "form", "formId": "f1", "path": "email" }

// props родителя
{ "type": "props", "path": "title" }

// runtime-источник (разрешён через рантайм)
{ "type": "runtime", "source": "navigator.language" }
```

---

## 7. Content (контент и локализация — одно)

Контент — это текст с вариативностью по языкам. Хранение — **base + overlay translations**:

```jsonc
// серверное хранилище Content
{
  "id": "c1",
  "collectionId": "col.articles",        // группа контента; резервная "strings" = UI-строки
  "schemaId": "schema.article.card",     // компонентная schema
  "fields": {                            // базовый язык (defaultLocale)
    "title": "Hello",
    "description": "Description",
    "image": { "assetId": "asset-1", "variant": "thumb" }   // AssetRef, не URL
  },
  "translations": {                      // overlay: только переопределения
    "ru": { "title": "Привет", "description": "Описание" },
    "de": { "title": "Hallo" }
  }
}
```

Чтение — всегда с `?locale` (или без → базовый язык). Сервер отдаёт **смёрженный** результат `fields` + `translations[locale]`:

```jsonc
// GET /api/contents/c1?locale=ru  →  готовый к показу текст
{
  "id": "c1",
  "collectionId": "col.articles",
  "fields": {
    "title": "Привет",                 // перевод
    "description": "Description",      // фолбэк на base
    "image": { "assetId": "asset-1", "variant": "thumb" }
  }
}
```

Правила:

- **Серверный фолбэк**: для локали без перевода возвращается base. Клиент не гоняет fallback-цепочку.
- `translations[locale]` хранит overlay (переопределения), не полные копии.
- UI-строки — тоже контент: резервная коллекция `strings`, `id` = ключ (`nav.home`), значение — смёрженное поле. `I18n.t(key)` читает его (см. `spec.md` §7a).
- Поля со ссылкой на ассет `{ "assetId": "...", "variant": "..." }` (AssetRef) при обращении к компоненту резолвятся рантаймом в URL (см. §9).

---

## 8. Дизайн-токены/тема

```jsonc
{
  "themeId": "theme.default",
  "tokens": {
    "--color-primary": { "value": "#1a73e8" },
    "--space-unit":    { "value": "8px" }
  },
  "fonts": ["Inter", "Roboto"],           // подключаемые шрифты
  "assets": ["asset-1"]                  // статические ресурсы темы
}
```

Значения — строковые; рациональность (дозволить `value` и `ref`) — деталь реализации, но `ref` на другой токен должен резолвиться перед применением.

---

## 9. Ассеты (Asset-метаданные)

Content хранит ссылку `{ assetId, variant? }`, а не URL. URL определяет рантайм из метаданных. `asset.get`/`asset.batch` отдают **метаданные** (не байты):

```jsonc
// GET /api/assets/asset-1  → AssetMeta
{
  "id": "asset-1",
  "mime": "image/jpeg",
  "size": 2048,
  "variants": [
    { "name": "master", "url": "/api/assets/asset-1/file", "mime": "image/jpeg", "size": 2048,
      "width": 1600, "height": 900 },
    { "name": "thumb",  "url": "/api/assets/asset-1/file?variant=thumb", "mime": "image/jpeg",
      "size": 256, "width": 320, "height": 180 }
  ]
}
```

Ссылка внутри контента (AssetRef):

```jsonc
{ "assetId": "asset-1", "variant": "thumb" }
```

Разрешение:

- рантайм кладёт метаданные в `store.assets` (immutable, кэш, on-demand);
- поле-объект вида `{ assetId, variant? }` при обращении к компоненту заменяется на URL выбранного варианта (`default: "master"`);
- неизвестный assetId → ссылка остаётся как есть (не ошибка на этапе декора).

Байты ассета отдаются сервером по `variants[].url` и/или через route-правило `serveAsset` (см. §4): у обычного `robots.txt` или картинки может быть «красивый» путь, назначенный роутом.

---

## 10. Формы

Определение формы (дизайн-тайм, отдаётся `form.get`):

```jsonc
{
  "id": "form.contact",
  "fields": [
    { "name": "email",  "type": "email",    "required": true },
    { "name": "theme",  "type": "select",   "required": true, "options": ["general", "bug"] },
    { "name": "message", "type": "textarea", "required": true, "rules": [{ "id": "minLength", "value": 10 }] }
  ],
  "validation": [                                // cross-field
    { "id": "confirmMatch", "fields": ["password", "password2"] }
  ],
  "submit": { "target": "endpoint.form.submit", "providerId": "liapoldus.builtin" }
}
```

Submit — server-only операция `form.submit` (raw JSON):

```jsonc
// POST /api/forms/{formId}/submissions  (server-only)
{
  "formId": "form.contact",
  "locale": "ru",
  "submittedAt": "2026-09-05T12:00:00Z",
  "values": {
    "email": "a@b.c",
    "theme": "general",
    "message": "Hello"
  }
}
```

Ответ: `{ "submissionId": "subm_1", "status": "ok" }`. Backend пишет **raw JSON** в отдельную таблицу форм; серверного состояния формы для поллинга нет.

---

## 11. Публикация (контракт — то, что отдаёт `/runtime/contract`)

```jsonc
{
  "siteId": "site-1",
  "environment": "production",            // "development" | "production" | ...
  "versionId": "ver_123",
  "snapshotId": "snap_abc",
  "defaultLocale": "ru",                  // язык по умолчанию
  "providers": [ /* ProviderDescriptor[] */ ],
  "operations": [ /* OperationDescriptor[] (вкл. poll-расписания) */ ],
  "endpoints":  [ /* EndpointDescriptor[] */ ],
  "routes":     [ /* RouteDescriptor[] */ ],
  "theme":      { /* ThemeDescriptor */ }
}
```

Дерево (`tree`) — отдельная операция `tree.get` (не в контракте): контракт маленький, структура обновляется отдельно (без полной пересборки контракта).

---

## 12. Валидация

| Правило                                                       | Ошибка                 |
| ------------------------------------------------------------- | ---------------------- |
| `id` — непустая строка, уникальна в своём реестре              | `duplicate_registration` |
| `protocol` ∈ {"http","ws","sse","graphql"}                     | `descriptor_validation` |
| `type` ∈ {"query","mutation"}                                  | `descriptor_validation` |
| `providerId` существует в реестре                               | `unknown_provider`     |
| `scope` ∈ {"public","server"}                                  | `descriptor_validation` |
| `poll.schedule` — валидный 6-полевый cron                       | `descriptor_validation` |
| `form.submit.target` указывает на endpoint/операцию из реестра  | `descriptor_validation` |
| matcher — валидный regex с якорями                              | `descriptor_validation` |
| `action.type` ∈ {"renderPage","serveAsset","redirect"}          | `descriptor_validation` |
| `renderPage.pageId` присутствует; `serveAsset.assetId` непустой | `descriptor_validation` |
| `redirect.target` непустой; `status` ∈ {301,302,307,308}        | `descriptor_validation` |
| binding `source.type` в списке source types                     | `descriptor_validation` |
| endpoint `operationId` ссылается на server-operation            | `descriptor_validation` |