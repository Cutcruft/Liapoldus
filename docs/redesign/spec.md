# Редизайн админки — спецификация (единый документ)

Единый документ редизайна: обзор и решения (R1–R19), доменная модель и API,
детальный интерфейс и план работ. Включает прежние `README.md`, `backend.md`,
`interface.md`, `todo.md`, `docs/api-admin.md`, `docs/api-client.md`. Связанные
операционные документы: `docs/postgres.md`, `docs/direct-integrations.md`,
`docs/backend/build-test-spec.md`, `docs/backend/dependency-service.md`,
`docs/ui-runtime/spec.md`.

- Статус: R0–R6 реализованы; R7–R13 — план (см. §12).
- Теги решений R* — из §1.2.

---

## 1. Обзор

### 1.1 Цель

Текущая админка — набор site-страниц (`/sites/:siteId/...`), куда каждая область ходит
своим маршрутом. Сквозные проблемы:

1. Нет единой «витрины»: первым экраном должна быть главная с карточками всех сайтов.
2. Редактирование страницы (дерево) и работа с контентом слабо разведены: редактор
   дерева насильно включает технику, а контент/медиа/формы разбросаны по разным роутам.
3. Компоненты не редактируются из админки (только через git), биндинги спрятаны
   в инспекторе, страница описывается вложенным деревом вместо простой последовательности.
4. Нет встроенного менеджера публикации (снапшоты → прод) и настроек сайта как сущности.

Редизайн приводит админку к формату «конструкторской платформы»:

```
Главная (карточки сайтов + создать + системные настройки)
  └── Таб сайта (URL-синхронная вкладка, шапка с тумблером режима)
        ├── Режим «Обслуживание» (по умолчанию): Контент (с языком), Формы+ответы, Медиа
        └── Режим «Редактор»: 5 разделов (вертикальная панель слева)
              ├── 1 Компоненты   — IDE: TSX в tiptap-редакторе, live-валидация, schema из TS
              ├── 2 Инфраструктура — операции/эндпоинты + формы
              ├── 3 Страницы     — URL-роуты (regex) + линейный лист компонентов + биндинги
              ├── 4 Менеджер состояний — снапшоты, выпуск в прод, откат
              └── 5 База сайта   — site-settings, темы/токены, шрифты/фавиконка, шаблоны
```

### 1.2 Согласованные решения

| # | Область | Решение |
| --- | --- | --- |
| R1 | Главная | Карточки всех сайтов + «создать сайт» + системные настройки `/settings`. Объединяем текущие Dashboard + Sites. |
| R2 | Табы | URL-синхронные вкладки: каждый сайт — свой route `/sites/:siteId/…`; список открытых вкладок в SPA-состоянии (закрытие, восстановление при рефреше); шапка-трей с крестиками. |
| R3 | Режимы | Таб сайта открывается **в режиме «Обслуживание»**; тумблер `[Обслуживание | Редактор]` в правом верхнем углу шапки таба; режим отражён в URL (`?view=editor`). |
| R4 | Обслуживание | Только контентная часть: **Контент** (с локалями), **Формы+ответы**, **Медиа**. Техника в обслуживание не входит. |
| R5 | Локализация | Контент и локализация — одна сущность: один и тот же контент добавляется на разных языках (селектор языка), работаем поверх существующего merge-механизма (базовый язык + overlay `Translations[locale]`). |
| R6 | Редактор компонентов | Источник истины компонента — **TSX в code-блоке tiptap**: подсветка синтаксиса, slash-палитра `/` (сниппеты, рефы на другие компоненты, binding-плейсхолдеры). Один файл на компонент; версия = git SHA. |
| R7 | Валидация TSX | **Live в браузере**: встроенный трансформер/проверка типов, ошибки подсвечиваются под строкой (полноценное IDE-ощущение); серверный esbuild — только при сборке. |
| R8 | Schema пропсов | **Авто-вывод из TS-интерфейса props** компонента → schema для биндинга/форм-редактора (плюс ручные уточнения поверх). |
| R9 | Страница | Страница = **чисто линейный лист компонентов** (вложенности нет); порядок важен, drag-drop. Повторное использование — сами компоненты, «фрагменты» не вводим. |
| R10 | Разделы 3+4 | «Страницы» и «Биндинг» **слиты в один раздел**: реестр URL-роутов (regex, priority) + композиция страницы (лист) + привязки (форма/операция/контент/query) + **передача regex-групп из пути в props**. *(Пересмотрено в R18: композиция и привязки переезжают в контент-редактор; «Страницы» остаются реестром маршрутов.)* |
| R11 | Инфраструктура | Раздел = реестр **операций/эндпоинтов** (провайдеры `http|ws|sse|graphql`, `query|mutation`, scope `public|server`) + **формы** (schema, `submit.target: endpoint.<id>|operation.<id>`). Работаем **поверх существующего механизма ui-runtime** (`OperationDescriptor`/`EndpointDescriptor`/`ApiClient`). |
| R12 | Внешние операции | При рендере фронтоп сам ходит по указанному адресу (браузер напрямую). Механизм в ui-runtime дорабатываем, секреты/среды на сервер не выносим. |
| R13 | Менеджер состояний | Снапшоты, **явный выпуск в прод** (кнопка «выпустить»), **откат** = пере-публикация старого снапшота; пиним активный прод-снапшот. |
| R14 | База сайта | Новая entity **`site-settings`** (title/description/lang/favicon→asset/meta); темы/токены наследуются от токен-редактора; шрифты/фавиконка — ассеты из Медиа; базовые шаблоны/заготовки. |
| R15 | Права | Одна роль пользователя (без RBAC-разведения); публикация в прод — отдельное явное действие, не авто-сейв. |
| R16 | Бэкенд | Полная перестройка слоёв разрешена: новая модель страниц/роутов, site-settings, реестр компонентов, сводки локалей. |
| R17 | Миграция | **Старые данные сбрасываем, прода нет**: удаляем дерево-модель, старый EditorPage/TreePanel/Inspector/tree-utils; конвертеры не нужны — чистый старт. |
| R18 | Контент-редактор | **Контент — главный дизайнерский workspace**: страница → упорядоченный лист элементов-шаблонов → предпросмотр-редактор (живой canvas на ui-runtime по листу) с контентом (внешние `Content`, биндинг `content.*`), переводами (вариативность), ассетами и биндингами источников. Пересборка структуры страницы (add/move/remove) и «назначение инфраструктуры» (operation/endpoint/form → `BindingSource`) — здесь же, для дизайнеров. «Страницы» сводятся к реестру маршрутов `Route→Page` (пересмотр R10). |
| R19 | Ассеты в схеме | Тип `asset`/`asset[]` в PropSchema (значение = `assetId`, kinds image/file/video); пикеры + превью во всех редакторах; URL через `AssetResolver`; usage «где используется» по `Element.props` + `Content.Fields` (все локали) + `Form.Definition` + `SiteSettings.faviconAssetId`. |

### 1.3 Терминология

| Термин | Значение |
| --- | --- |
| Сайт (site) | Один конструкторный проект; карточка на главной, одна вкладка. |
| Таб | Вкладка сайта в трее; URL-синхронная. |
| Режим | «Обслуживание» или «Редактор» — способ работы с сайтом. |
| Раздел | Пункт вертикальной панели в режиме «Редактор» (5 разделов). |
| Компонент (definition) | TSX-шаблон с schema props; версия = git SHA; один файл. |
| Инфраструктура | Сущности окружения: операции/эндпоинты (запросы), формы-сборы. |
| Страница | URL-роут (regex, priority) + линейный лист компонентов с привязками. |
| Элемент | Один пункт листа страницы: компонент + props + bindings. |
| Биндинг | Привязка данных: форма/операция/контент/query/группа роут-регулярки → props. |
| Снапшот | Зафиксированный состав сайта (страницы, сущности, deps-lock) для сборки/выпуска. |
| site-settings | Сущность настроек сайта (метаданные, favicon, lang). |
| Контент-редактор | Главный дизайнерский workspace (R18): страницы → лист элементов → предпросмотр-редактор с контентом/переводами/ассетами/биндингами. |

---

## 2. Доменная модель

### 2.1 Компонент (по сути как сейчас, уточняется schema)

```ts
ComponentDefinition {
  id: string;          // уникальный id (пространство имён сайта)
  source: string;      // TSX (один файл, R6)
  version: string;     // git SHA коммита
  schema: PropSchema;  // авто-вывод из TS props (R8) + ручные уточнения
}
```

- Хранение/версии — через существующий git-механизм (компоненты — git-модель, этап 2).
- Новое: **серверная история/реестр** для UI (list, detail, history, где используется).

### 2.2 Инфраструктура

Переиспользуем домен дескрипторов `ui-runtime` (`docs/ui-runtime/spec.md`):

```ts
OperationDescriptor { kind:'operation'; id; provider: 'http'|'ws'|'sse'|'graphql';
                      typeOp: 'query'|'mutation'; method; path; scope: 'public'|'server'; ... }
EndpointDescriptor  { kind:'endpoint'; id; method; operationId; ... }
FormDefinition      { id; schema: FormFields[]; submit: { target: 'endpoint.<id>'|'operation.<id>' } }
```

- CRUD этих сущностей — admin-операции (раньше только декларативные в контракте).
- Внешние адреса frontend ходит сам (R12), без серверного прокси и секретов.

### 2.3 Страница и роут (новая модель, R9/R10)

```ts
Route {
  id; siteId; name;
  matcher: string;     // regex URL (якорные ^…$)
  priority: number;
  action: { type: 'page'; pageId: string } | { type: 'redirect'; target; status; keepQuery };
}

Element {
  id;                    // стабильный (draggable, bindings ссылаются стабильно)
  componentId: string;
  props: Record<string, { kind: 'literal'; value: unknown }
                     | { kind: 'binding'; source: BindingSource }>;
  bindings?: BindingSource[];  // дубль для явного списка (форма/операция/контент/query)
}

BindingSource =
  | { kind: 'content';    contentId; field }
  | { kind: 'form';       formId }
  | { kind: 'operation';  operationId }
  | { kind: 'query';      param }
  | { kind: 'routeGroup'; index: number }   // группы regex-роута → props (R10)

Page {
  id; siteId; name;
  list: Element[];       // чисто линейный, порядок значим (R9)
}
```

Wire-форма (реализована, К4/К5):
- `PUT /api/pages/{pageID}` — тело `{name, list}` где `list: []domain.Element`;
- `POST /api/sites/{siteId}/pages` — тело `{name, slug, list}`;
- `domain.Element {id, componentId, props, bindings}` (backend/internal/domain/model.go).

- Вложенного дерева нет — убрана сущность `ComponentNode` и её обработчики (tree, tree-utils).
- «Страницы» в UI = связка `Route → Page` (реестр роутов + композиция).

### 2.3.1 Схема компонента: контент-слоты и ассеты (R13/R18-R19)

```ts
PropSchemaEntry {
  name; label?;
  type: 'string' | 'text' | 'number' | 'boolean' | 'select' |
        'asset' | 'asset[]' | 'object' | 'array';
  kinds?: ('image' | 'file' | 'video')[];  // только для type asset/asset[]
  role?: 'content' | 'config';             // content = дизайнерский слот (значение по binding)
  required?; default?;
  defaultSource?: BindingSource;           // дефолт биндинга при создании элемента (обычно content.<auto>.field)
}
```

- `role: 'config'` — значение литерал ставится один раз (dev); `role: 'content'` — дизайнерский
  слот, редактируется через контент-редактор и является полем внешнего `Content`.
- Значение ассета — строка `assetId` (как в `Content.Fields`); runtime резолвит URL через `AssetResolver`.
- `createContentFromElement`: авто-создание `Content`-рекорда с `collectionId = componentId`,
  fields по контент-слотам схемы; контент-слоты элемента стартуют binding-ом `content.<contentId>.<field>`.

### 2.4 site-settings (новая сущность, R14)

```ts
SiteSettings {
  siteId: string;
  title: string; description: string; lang: string;
  faviconAssetId?: string;
  meta: Record<string, string>;   // og:, twitter:, доп. теги
  defaultPageId?: string;         // стартовая страница (берется из initialTree контракта)
}
```

- Значения текущих site-полей (имя/slug) не дублируем — только UI/metdata-уровень.
- Шрифты/фавиконка — ассеты (Медиа), в settings — ссылки `assetId`.

### 2.5 Контент и локали (R5)

- Сущность контента без изменений в ядре: `Content{ Base Fields; Translations: {locale → fields} }`, фоллбек `merged()`.
- Новое на API/UI: селектор языка, список локалей сайта, сводка «переведено/нет», отсутствующие переводы маскируются базовыми.

### 2.6 Снапшот и публикация

```ts
Snapshot { id; siteId; name; pages: SnapshotPage[]; depsLock; createdAt }   // как есть
Deployment { siteId; environment: 'development'|'production'; snapshotId }   // активный пин прод (R13)
```

- Добавляем пин активного прода (`Deployment`), откат = смена `snapshotId` и пере-публикация артефакта из artifactstore (уже идемпотентна).

---

## 3. API

### 3.1 Общие конвенции

- **Admin API** — редакторский интерфейс конструктора: CRUD всех управляемых сущностей
  (сайты, страницы, контент, переводы, ассеты, роуты, формы, снапшоты).
  - Сервер: отдельный HTTP-порт `LIAPOLDUS_ADMIN_ADDR` (по умолчанию `:8080`).
  - Всё тело JSON, `Content-Type: application/json` (кроме upload байтов ассета — `multipart/form-data`).
  - Аутентификация — `Authorization: Bearer <токен>` при заданном `LIAPOLDUS_ADMIN_TOKEN`; пустой токен = открытый доступ (dev).
  - CORS: `*`, методы `GET,POST,PUT,DELETE,OPTIONS`, заголовки `Content-Type, Authorization`.
- **Client API** — публичный runtime-интерфейс сайта: контент, ассеты, формы, runtime-контракт и единый edge-роутинг.
  - Сервер: отдельный HTTP-порт `LIAPOLDUS_CLIENT_ADDR` (по умолчанию `:18080`).
  - Без аутентификации; CORS `*`.
  - Выбор сайта: **по `Host`** из `site.hosts` (точное совпадение или дик `*.domain`), иначе — сайт по `LIAPOLDUS_CLIENT_DEFAULT_SLUG`, иначе `404`.
  - Ручки, помеченные `{siteId}`, принимают явный siteId в пути (для программного доступа/ui-runtime-паритета) и обслуживают любой существующий сайт.
  - В этом раунде client отдаёт **актуальные данные напрямую** (pipeline Environments/Build/Publication — отдельный этап); `environment`/`versionId` в контракте принимаются, но не фильтруют.
- Ошибки: единый формат `{"error":"..."}`. Status: admin `400` (invalid), `401` (unauthorized), `404` (not found), `409` (already exists), `405` (method), `500` (internal); client `400/404/500` (+`405`).

### 3.2 Admin API — сущности

| Сущность | Назначение |
| --- | --- |
| `Site` | изолированный контейнер сайта: `name`, `slug`, `defaultLocale`, `hosts[]` (домены клиентского порта), `createdAt` |
| `Page` | страница с линейным листом элементов и версионированием |
| `Content` | контент: `fields` (base, язык по умолчанию) + `translations[locale]` (overlay-переопределения) |
| `Asset` | метаданные файла (имя, mime, size, `variants`), байты на диске (`LIAPOLDUS_ASSET_DIR`) |
| `Route` | единый маршрут для клиентской навигации и edge: `matcher`, `priority`, `action` |
| `Form` | определение формы (схема/валидация); сабмиты пишутся raw JSON в таблицу |
| `Snapshot` | фиксация актуальных версий страниц сайта |

### 3.3 Admin API — Sites

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

### 3.4 Admin API — Pages (новая модель)

```http
POST /api/sites/{siteId}/pages
Content-Type: application/json

{"name":"Home","slug":"home","list":[]}
```

Ответ `201` — `Page` (`version: 1`). Обновление листа выпускает новую версию:

```http
GET /api/sites/{siteId}/pages
GET /api/pages/{pageId}
PUT /api/pages/{pageId}        // {"name":..., "list":[{id,componentId,props,bindings}]} → version++
GET /api/pages/{pageId}/versions
GET /api/pages/{pageId}/versions/{versionId}
DELETE /api/pages/{pageId}     // 204
```

Элемент:

```json
{
  "id":"el_1","componentId":"cmp.Text",
  "props":{
    "text":{"kind":"binding","source":{"kind":"content","contentId":"hero.title","field":"text"}},
    "link":{"kind":"literal","value":null}
  },
  "bindings":[{"kind":"content","contentId":"hero.title","field":"text"}]
}
```

### 3.5 Admin API — Content

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

#### Переводы (overlay)

```http
PUT /api/sites/{siteId}/contents/{contentId}/translations/{locale}
Content-Type: application/json

{"fields":{"title":"Привет"}}
```

```http
GET /api/sites/{siteId}/contents/{contentId}/translations    // {ru:{...}, en:{...}}
DELETE /api/sites/{siteId}/contents/{contentId}/translations/{locale} // 204
```

Правила слияния для клиентского чтения: `fields = base.fields`, затем поля из `translations[locale]` поверх; перевод, покрывающий не все поля, «дополняет» base (см. §3.10).

### 3.6 Admin API — Ассеты

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

### 3.7 Admin API — Компоненты

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

### 3.8 Admin API — Роуты

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

### 3.9 Admin API — Формы и снашпоты

Форма (определение по схеме `docs/ui-runtime/spec.md`):

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
GET /api/sites/{siteId}/forms/{formId}/submissions   // [{"id","formId","siteId","payload","createdAt"}]
```

Снапшот:

```http
POST /api/sites/{siteId}/snapshots        // {"name":"Release 1"}
GET /api/sites/{siteId}/snapshots
GET /api/snapshots/{snapshotId}
DELETE /api/snapshots/{snapshotId}        // 204
```

### 3.10 Client API

#### Выбор сайта и записи

| Параметр | Значение |
| --- | --- |
| `Host` | матчится по `site.hosts` (точное или `*.domain`) |
| `LIAPOLDUS_CLIENT_DEFAULT_SLUG` | фолбэк-сайт, если Host не совпал |

#### Контент (base + overlay, серверный фолбэк)

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

UI-строки — контент коллекции `strings`; ид = ключ. Батч одним запросом (потерянные id пропускаются):

```http
POST /api/sites/{siteId}/contents/batch?locale=ru
Content-Type: application/json

{"ids":["a1","a2"]}
```

Ответ: `{"a1":{...},"a2":{...}}`.

#### Ассеты

```http
GET /api/assets/{assetId}
GET /api/sites/{siteId}/assets
GET /api/assets/{assetId}/file?variant=master
```

Ответ отдаёт `Content-Type` по mime, `ETag`, `Cache-Control: public, max-age=31536000, immutable` и `Accept-Ranges`. Неизвестный variant → `400`; нет байтов → `404`.

#### Формы

```http
GET /api/forms/{formId}
POST /api/forms/{formId}/submissions
Content-Type: application/json

{"formId":"form.contact","locale":"ru","submittedAt":"2026-01-01T00:00:00Z","values":{"email":"a@b.c"}}
```

Ответ `201`: `{"submissionId":"subm_x","status":"ok"}`. Сервер проверяет: `formId` в теле совпадает с путём; значения валидируются по определению формы (`required`, `minLength`, `type: email`) → `400 {"error":"..."}` при несоответствии.

#### Runtime-контракт

```http
GET /runtime/contract?siteId={siteId}&environment={environment}&versionId={versionId}
```

Ответ (текущее состояние сайта):

```json
{
  "siteId":"site_x",
  "defaultLocale":"ru",
  "routes":[...],
  "forms":[{"id":"form.contact","fields":[...],"submit":{...}}],
  "operations":[],
  "endpoints":[],
  "environments":[],
  "theme":null
}
```

Дополнительно: `GET /runtime/contract` отдаёт `siteSettings` и route-группы в дескрипторах роутов (в развитие контракта — R14). Прежние поля контракта, связанные с деревом (`initialTree`), заменяются на **лист** (`initialPage`/`list`).

Сокращённый список маршрутов: `GET /runtime/routes?siteId={siteId}`.

#### Edge-роутинг (единая таблица маршрутов)

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

### 3.11 Admin API — Build

Публикация снапшота в окружение. Сборка **синхронная**: `POST …/builds` возвращает готовый Build (`queued→building→ready|failed`); повторная публикация той же пары snapshot+environment — no-op и возвращает существующий ready-Build. Окружение — одно из `development|production` (staging нет).

```http
POST /api/sites/{siteId}/builds
Content-Type: application/json

{"snapshotId":"snap1","environment":"production"}

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

#### Живой статус сборок (WS)

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

#### Аутентификация

- Если `LIAPOLDUS_ADMIN_TOKEN` пуст — admin открыт (режим разработки).
- Иначе каждый запрос admin требует `Authorization: Bearer <токен>` (сравнение constant-time); без него — `401 {"error":"unauthorized"}`.

### 3.12 Новые админ-ресурсы (редизайн)

| Ресурс | Операции | Комментарий |
| --- | --- | --- |
| `/components` | list, get, getHistory, delete | реестр компонентов (R6/R8); source и schema |
| `/settings` (site) | get, update | site-settings CRUD (R14) |
| `/locales` | list, getSummary, updateTranslation | сводка/редактирование переводов (R5) |
| `/operations`, `/endpoints`, `/forms` | CRUD | инфраструктура как управляемая модель (R11) |
| `/pages`, `/routes` | CRUD (лист) + element-мутации `addElement/moveElement/removeElement/updateElement`, `createContentFromElement` | вместо tree-операций (R13) |
| `/deployments` | pin, rollback | активный прод-снапшот, откат (R13) |

### 3.13 Упрощаются/удаляются

- tree-операции (`/tree`), ComponentNode-моделей не будет.
- Старый редактор/превью-флоу `page-store`/`tree-utils` удаляются.
- Скрытые страницы контента/форм перекладываются на consolidated ресурсы.
- Использование ассетов «где используется» распространяется на `Element.props` (литералы asset
  и любые значения), `Content.Fields` всех локалей, `Form.Definition`, `SiteSettings.faviconAssetId` (R13).

### 3.14 Влияние на ui-runtime и сборку

- **Рендер**: `PageRenderer`/`RouteOutlet` рендерят **лист** (порядок элементов), вместо рекурсии по дереву; элемент → компонент + props (литерал или приведённый binding). Механизм `InstanceNode` заменяется на `ElementNode`. *(Реализовано, К4: ui-runtime 262/262.)*
- **Route-группы**: при матче роута сохраняем regex-группы → доступ `{{route.$n}}` и `BindingSource.routeGroup` (R10).
- **Операции/формы**: `ApiClient` (query / callEndpoint) уже есть — расширяем декларативными операциями/формами из раздела «Инфраструктура»; формы `submit.target` уже поддержан (type form.ts).
- **Контент-биндинги**: `merged()`-механизм и `AssetResolver` не меняются.
- **Live-канвас (R13)**: `PageRenderer`/`ElementNode` по листу рендерится и в админке поверх
  чернового стора (content-drafts + активная локаль); операции/формы в превью не выполняются —
  плейсхолдер или «превью-данные».
- **Schema**: дескриптор дополняется `asset`/`asset[]` и `role`/`defaultSource` для типизированных
  контролов и превью ассетов в контент-редакторе.
- **Контракт/типы**: расширяем `descriptor.ts` новыми полями (siteSettings, page list, routeGroups) — правки в `docs/ui-runtime/spec.md`.
- **Materializer**: определения компонентов собираются по **используемым page.list** (перечисляем `componentId` элементов страниц снапшота вместо обхода дерева).
- **entry.tsx** генерится как сейчас (register + boot). Метаданные site-settings (title/favicon/og) подмешиваются в `index.html` и контракт.
- **Постраничная раздача** (per-page chunks): линейная модель листа подходит напрямую — чанк страницы = её элементы.

---

## 4. Что удаляем (R17)

- Дерево-модель: `ComponentNode`, `tree-utils`, tree-операции/таблицы.
- Старый редактор: `EditorPage`, `TreePanel`, `Inspector` (дерево-интерфейсы), дерево-превью флоу.
- `page-store`/`preview-store` в старом виде (заменяются новыми slice-сторами разделов).
- Старые данные страниц/деревьев — сбрасываются (прода нет; конвертеры не нужны).

*(Частично выполнено К5: admin-port перевёл редактор на лист; tree-utils → list-utils; EditorPage/TreePanel/Inspector переписаны под flat-list API.)*

---

## 5. Интерфейс

### 5.1 Глобальный слой

#### Шапка-трей табов

- Сверху — горизонтальный трей открытых вкладок. Вкладка = сайт + системные настройки.
- Вкладка сайта: favicon/логотип (если задан), имя сайта, индикатор режима, крестик закрытия.
- Справа в трее — кнопка «+» (новый сайт) и управление закрытием (средняя→закрыть).
- **Поведение**:
  - Открытие сайта с главной — добавляет вкладку и активирует её.
  - Закрытие — закрывает вкладку, активной становится предыдущая; если закрыт последний таб — переход на главную.
  - Рефреш/глубокая ссылка — админка восстанавливает вкладку из URL (одну, текущую); список открытых не персистится между сессиями (см. R2: список в SPA-состоянии).

#### Главная `/`

- **Секция «Системные настройки»** — отдельная ссылка/карта в углу страницы (ведёт в `/settings`, не является табом сайта).
- **Сетка карточек сайтов**: карточка = имя, slug, последний статус сборки, количество страниц, редактор (если есть) и меню. Клик по карточке — открыть сайт в новой вкладке (режим «Обслуживание»).
- **Карточка «Создать сайт»**: модал с именем + slug; после создания сайт открывается в новой вкладке.

### 5.2 Таб сайта: режимы

#### Шапка таба

```
[ favicon ] Имя сайта        [ Обслуживание | Редактор ]   [ × ]
```

- Сегмент-тумблер справа вверху (R3): два состояния.
- Режим отражён в URL: `?view=editor` для редактора; отсутствие параметра = обслуживание.
- При открытии сайта всегда активен режим «Обслуживание».

#### Режим «Обслуживание»

Вертикальная навигация слева (узкая полоса подписей): **Контент · Формы+ответы · Медиа**.
Технические разделы здесь недоступны — они живут в редакторе.

#### Режим «Редактор» (`?view=editor`)

Вертикальная панель слева — 5 кнопок с иконкой и подписью (R-разделы):

| Кнопка | Раздел | Суть |
| --- | --- | --- |
| 1 | Компоненты | IDE: tiptap-редактор TSX, реестр компонентов, история |
| 2 | Инфраструктура | операции/эндпоинты + формы |
| 3 | Страницы | роуты + композиция страниц + биндинги |
| 4 | Менеджер состояний | снапшоты, выпуск, откат |
| 5 | База сайта | site-settings, темы/токены, шрифты, шаблоны |

Справа от панели — рабочая область раздела.

### 5.3 Режим «Обслуживание» — экраны

#### Контент

Общий экран контента (R5: контент и локализация — одна сущность).

- **Список**: фильтры по контент-типу и **языку**; колонки: заголовок, тип, язык, статус перевода, дата изменения.
- **Селектор языка** в шапке списка и в редакторе: базовый язык + языки, на которых уже есть переводы.
- **Редактор контента**:
  - Поля по schema типа (как текущий контент-редактор + rich-text через tiptap).
  - **Вкладка/блок «Переводы»**: список языков, статус (переведено / не переведено), переход на редактирование перевода.
  - Сохранение перевода — overlay `Translations[locale]` поверх базовых полей (существующий `merged()` на бэке).
- Поведение: смена языка не теряет введённое (draft в slice-хранилище), пометка «непереведённое поле — показывается базовое».

#### Формы + ответы

- **Список форм** (описание форм тоже управляется в разделе «Инфраструктура» — здесь только просмотр и ответы).
- **Ответы**: по выбранной форме — таблица сабмиссий (значения полей, время, статус), экспорт в CSV, действия (просмотр деталей, удаление).
- Достопные данные: GET сабмиссий формы (существующие form-/submissions-операции).

#### Медиа

- Грид ассетов (картинки/файлы): превью, имя, тип, размер; поиск/фильтр по типу.
- Загрузка файла (drag-drop + кнопка), контекст-меню: скачать, скопировать URL, удалить.
- Панель «используется на страницах/в контенте» (по биндингам ассетов), выбранный как favicon — бейдж «favicon».

### 5.4 Режим «Редактор» — разделы

#### Компоненты (IDE)

**Реестр (слева — список):** каждый компонент: имя/id, версия (git SHA), изменён ли (dirty), где используется (счётчик). Действия: создать, открыть, удалить (с подтверждением, если используется), история.

**Редактор компонента (основная область):**
- **Вкладка «Код»** — tiptap-редактор с code-блоком TSX (R6):
  - Подсветка синтаксиса TSX внутри блока.
  - Slash-палитра `/`: сниппеты примитивов, **ссылки на другие компоненты** (вставка рефа), binding-плейсхолдеры (`{{content.<id>}}`, `{{form.<id>.field}}`, `{{route.param}}`), одиночные комментарии/разделители.
  - Обычный текст вокруг code-блока — заметки/разделители (не влияют на сборку).
- **Live-валидация** (R7): транспилер/проверка типов в браузере; ошибки — маркером на строке + список в нижней панели; профиль кода «валиден/ошибка».
- **Панель Schema** (R8): авто-вывод из TS props → таблица пропсов (имя, тип, обязательность, лейбл); ручные уточнения (лейбл, группа, дефолт). Отсюда — дескриптор для биндинга и формы.
- **Сохранение**: явная кнопка «коммит» → git-коммит (новый SHA); пересборка не обязательна до публикации.
- **Предпросмотр компонента**: мини-рендер с текущими пропсами.

#### Инфраструктура

Подразделы внутреннего таба: **Операции** · **Эндпоинты** · **Формы**.

- **Операции** (R12, поверх `OperationDescriptor`): id, провайдер (`http|ws|sse|graphql`), typeOp (`query|mutation`), method, path, scope (`public|server`), привязка типа результата; среда «предпросмотр» — безопасный вызов для проверки.
- **Эндпоинты** (`EndpointDescriptor`): id, method, operationId; используются формами (`submit.target`) и компонентами.
- **Формы**: schema полей (тип/обязательность/лейбл/валидация) + `submit.target` (endpoint/operation). Ответы просматриваются в обслуживании (§5.3).
- Принцип: описываются **снаружи кода компонентов**, фронт ходит сам по адресу; секреты/окружение внешних систем не храним (R12).

#### Страницы

Раздел сведён к **реестру маршрутов** (R18, пересмотр R10): композиция, биндинги и
редактирование содержимого переехали в **Контент-редактор** (§5.6).

**Список роутов (слева):** таблица: matcher (URL-регулярка), priority, статус, страница
(id/название), тип действия (`page|redirect`). Действия: создать роут, создать страницу, связать.

- **Редактор роута**: regex, приоритет, опции (keepQuery, redirect status/target), связка роут→страница.
- **Route-группы → props**: редактируемая таблица «группа regex → пропс»: `$1 → :slug` и т.п.;
  на превью подставляются реальные значения из URL.
- **Переход в контент**: клик по связанной странице открывает её в контент-редакторе (§5.6).

#### Контент-редактор (дизайнерский workspace, R18/R19)

Главный экран для дизайнеров. Трёхколоночная раскладка в режиме «Редактор».

- **Слева — страницы**: список страниц (название, связанные роуты, количество элементов). Открытие
  страницы → её **лист элементов-шаблонов** (порядок = рендер; drag = пересборка структуры;
  удалить/добавить элемент).
- **Центр — live canvas**: `PageRenderer` (ui-runtime, по листу R11) рендерит черновик страницы:
  контент — черновики + активная локаль; роут страницы — default/выбранный; операции/формы в превью
  не выполняются (плейсхолдер/«превью-данные»).
- **Справа — инспектор выбранного элемента**:
  - **Контент** — по схеме компонента (schema): типизированные контролы текста/надписи/списка/файла;
    выбор или **создание Content-рекорда из элемента** (collectionId = componentId), редактирование
    переводов (base + locale = «вариативность»).
  - **Ассеты** — пикер + загрузка + превью (тип `asset`/`asset[]`).
  - **Биндинги / инфраструктура** — per-слот literal | биндинг; источник выбирается из контента
    (поле записи), инфраструктуры (operation/endpoint/form из R6) или query/route-групп.
- Пересборка структуры и «назначение инфры» — здесь же: `addElement/moveElement/removeElement/updateElement`,
  `createContentFromElement` (бэкенд R13).

**Режим «Обслуживание» → «Контент»** (§5.3) остаётся сырым JSON-представлением тех же записей
(фильтры, локали, drafts); для уверенной схемы контролов используются типы из schema.

#### Менеджер состояний

- **Список снапшотов**: dev/prod, время, статус (готов/сборка/ошибка), автор; текущие активные.
- **«Создать снапшот»**: имя + окружение → сборка.
- **«Выпустить в прод»** (R13): явная кнопка на снапшоте с подтверждением; пином становится активный прод-снапшот.
- **Откат**: выбрать старый снапшот → «откатить» → пере-выпуск его в прод (пере-публикация артефакта).
- Индикация несоответствия: «есть незакоммиченные правки компонентов/страниц» → подсказка создать новый снапшот.

#### База сайта

- **Site-settings** (R14, новая entity): название, description, lang, favicon (ассет из Медиа), метаданные (доп. теги). Эти поля идут в контракт и в `index.html`.
- **Темы и токены**: наследуем существующий токен-редактор (collection/моды/токены).
- **Шрифты**: ассеты (TFF/WOFF2) + регистрация font-family для тем.
- **Базовые шаблоны/заготовки**: стартовые наборы компонентов и страниц (импорт в новый сайт).
- **Прочее**: правила пермалинков, favicon/og-изображение по умолчанию.

### 5.5 Общие UX-правила

- **Несохранённые изменения**: бейдж на вкладке/разделе; подтверждение закрытия таба при dirty-состоянии.
- **Деструктивные действия** (удалить компонент/контент, выпуск в прод, откат) — подтверждающий диалог с текстом последствий.
- **Загрузки/ошибки**: скелетоны; ошибки операций — тостеры (как в текущем admin).
- **Поиск**: глобальный по компонентам/страницам/контенту/операциям; переходы «где используется» из любого контекста.
- **Клавиатура**: `/` — slash-меню в tiptap; быстрые переходы между разделами редактора.

---

## 6. Карта навигации

```
/                            Главная: карточки сайтов, «создать сайт», «системные настройки»
/settings                    Системные настройки (вкладка, не сайт)

/sites/:siteId               Таб сайта, режим «Обслуживание» (умолчание)
/sites/:siteId?view=editor   Тот же таб, режим «Редактор»
  └─ разделы редактора по ?section=... (или вложенным path):
      компоненты   инфраструктура   страницы   состояния   база-сайта

Сохранение активных разделов/режима — в URL, открытые табы — в SPA-состоянии.
```

Точное кодирование разделов редактора в URL: `?view=editor&section=<key>` (query-параметры, единый стиль с режимом). Подразделы «Обслуживания» — `?mode=content|forms|media`.

---

## 7. Риски и открытые вопросы

### 7.1 Открытые технические решения

1. **tiptap как код-редактор TSX**: видеть код в code-блоке с полноценным синтаксисом
   vs обёртка над code-editor (Monaco-lite) внутри tiptap. *(Решено в R5: lowlight-подсветка + sucrase.)*
2. **Live-валидация**: объём трансплера в браузере. *(Решено на R5: sucrase-транспайл + маркеры ошибок; полные типы — серверная сборка.)*
3. **Формат Element сериализации**: стабильный internal id vs индекс. *(Решено: стабильный внутренний id.)*
4. **Live-канвас в админке (R13)**: способ собрать TSX компонентов в браузере —
   sucrase-транспайл (уже в R5) + import-map на `@liapoldus/ui-kit`/`@liapoldus/ui-runtime`
   vs esbuild-wasm. Решается на R13.

### 7.2 Технические риски

1. **Live-валидация TSX в браузере**: размер вендор-чанка (TS-транспилер). Вариант — лёгкий транспилер + линт-эвристики, полная проверка типов на бэке при сборке.
2. **Route-группы и priority** при «страница = лист»: совместимость со сквозным кэшем маршрутов (client edge) — проверяем на R8.
3. **Инфраструктура как JSON-модель**: перенос дескрипторов из статики контракта в CRUD — синхронизация deployed-контракта с изменениями.
4. **Постраничная раздача** увязывается с новой линейной моделью — не блокирует, но учитываем при проектировании чанков на R7/R11.

---

## 8. План работ (слайсы)

Принципы слайсов:
- Сборка вертикальная: слайс даёт видимый результат в админке.
- Backend-слайсы (R*) дают API, frontend-слайсы садят их в UI.
- Старая модель удаляется только в R12 (после переезда на новую).
- Данные сбрасываем (R17): миграций/конвертеров не пишем.

### Слайс R0 — Сброс и зачистка (подготовка)

- [x] Сбросить dev-данные (страницы/деревья/снапшоты).
- [x] Убрать из TODO mvp-пунктов, связанных с старой моделью дерева (пометка «перенесено в редизайн»).
- [x] Зафиксировать регрессионные тесты на то, что остаётся (auth, sites, forms, assets, контент, билды).

### Слайс R1 — Каркас: главная + табы + оболочка

- [x] Объединить Dashboard и Sites в единую главную: карточки сайтов + «создать сайт» + ссылка «системные настройки».
- [x] Таб-трей: открытие сайта → вкладка `/sites/:siteId`; закрытие; восстановление активной из URL; «+» для создания.
- [x] Системные настройки как отдельная вкладка `/settings`.
- [x] Тесты: карточки, создание, открытие/закрытие табов, рефреш.

### Слайс R2 — Режимы таба сайта и шапка

- [x] Шапка таба: имя сайта + сегмент-тумблер `[Обслуживание | Редактор]` (R3).
- [x] `/sites/:siteId` = режим «Обслуживание» (по умолчанию); `?view=editor` = редактор.
- [x] Разметка рабочей области редактора: вертикальная панель 5 разделов + переходы (R-разделы).
- [x] Тесты: умолчание, тумблер, URL-синхронизация, восстановление режима (SiteTab.test.tsx).

### Слайс R3 — Обслуживание: Контент с локалями

Backend + Frontend (R4/R5).

- [x] B: модели локалей поверх `Content{Fields, Translations}`; операции: `GET/update` перевода, сводка переведённости.
- [x] F: список контента с фильтром по типу и языку; редактор с селектором языка; блок «Переводы» со статусами (R5).
- [x] F: draft в slice-сторе (потеря ввода при смене языка исключена).
- [x] Тесты: merge/overlay, сводка, редактор локали.

### Слайс R4 — Обслуживание: Формы+ответы и Медиа

Frontend (данные API уже есть), возможны API-доработки.

- [x] F: экран список форм + ответы (таблица сабмиссий, детали, экспорт).
- [x] F: экран Медиа: грид ассетов, загрузка, удаление, «где используется» (бейдж favicon — в R10).
- [x] B: DELETE сабмиссии (`/api/sites/{siteID}/forms/{id}/submissions/{sid}`); `GET /api/sites/{siteID}/assets/{assetID}/usage`.
- [ ] B (по необходимости): пагинация сабмиссий, поиск ассетов (сейчас поиск/фильтр — клиентские).
- [x] Тесты: формы+ответы, медиа, CSV-билдер в form-utils.

### Слайс R5 — IDE компонентов (тикап-редактор)

Прототип + ключевые решения (R6/R7/R8).

- [x] F: реестр компонентов слева (list, version, dirty, «где используется»).
- [x] F: tiptap-редактор TSX: code-блок с подсветкой (lowlight); slash-палитра `/`.
- [x] B: реестр-ресурс `/components` (get/history/delete) + git-коммит из UI.
- [x] F: live-валидация в браузере: sucrase-транспайл + маркер ошибки под строкой, debounce.
- [x] F: панель Schema: авто-вывод из TS props + usage панель (страницы).
- [x] Тесты: реестр, detail, правка исходника → PUT, валидация (tsx-validate), schema-вывод.

### Слайс R6 — Инфраструктура (операции/эндпоинты/формы)

Средняя модель infra как CRUD (R11/R12).

- [x] B: миграция дескрипторов (Operation/Endpoint/Form) в домен + admin-CRUD ресурсы.
- [x] F: раздел «Инфраструктура»: подтабы операции/эндпоинты/формы; безопасный «предпросмотр» вызова операции (R12).
- [x] B/F: контракт runtime отдаёт актуальные дескрипторы (не зашитые статикой).
- [x] Тесты: CRUD (admin_infra_test + operations.test), submit-таргеты форм, предпросмотр операции (InfraSection.test).

### Слайс R7 — Новая модель страниц (данные)

Сердце перестройки (R9/R16). *(Контракт-часть выполнена К4/К5: `Page.list`, `ElementNode`,
`BindingSource`, `PUT /api/pages/{pageID}` body `{name, list}`, `POST /api/sites/{id}/pages`
body `{name, slug, list}`; admin-редактор переведён на лист.)*

- [ ] B: сущности `Route`, `Page{list: Element[]}`, `Element`, `BindingSource` (page/route). *(частично — плоский Page/Element реализован)*
- [ ] B: API: CRUD страниц/роутов; валидация matcher-регулярок; группы regex сохраняются в Element.
- [ ] B: перестройка обработчиков содержимого страниц (PageVersion/снапшотные ссылки → list).
- [ ] B: снапшот/материализация: список элементов → используемые componentId (вместо обхода дерева).
- [ ] Тесты: CRUD, порядок, regex-группы, снапшот-сборка по листу.

### Слайс R8 — Раздел «Страницы»: реестр маршрутов (UI)

Frontend на модель R7 (R10). Композиция страницы (лист, drag, блок элемента, биндинги)
перенесена в R13 «Контент-редактор» (R18: структура — задача дизайнера, живёт в контенте).

- [ ] F: реестр роутов: list/get/update/delete; matcher (regex), priority, action (page | redirect + keepQuery), связка роут→страница.
- [ ] F: таблица «группы regex → props» (предпросмотр групп матчера).
- [ ] F: переход из роута в соответствующую страницу контент-редактора.
- [ ] Тесты: CRUD роутов, валидация матчера, связка роут→страница.

### Слайс R9 — Менеджер состояний

- [ ] B: пин активного прод-снапшота (`Deployment`), операции pin/rollback (R13).
- [ ] F: раздел: список снапшотов (dev/prod, статусы), «создать снапшот», «выпустить в прод» (подтверждение), «откатиться».
- [ ] F: индикация «есть незакоммиченные правки → создать снапшот».
- [ ] Тесты: выпуск, откат, idempotent публикация снапшота.

### Слайс R10 — База сайта (site-settings)

Backend + Frontend (R14).

- [ ] B: entity `SiteSettings` + CRUD; favicon→asset, lang, meta; фолбэки дефолтов.
- [ ] F: раздел: форма site-settings, интеграция фавиконки (из Медиа), og-meta.
- [ ] B: контракт runtime и index.html получают метаданные/фавиконку (R14).
- [ ] F: темы/токены (наследуемый токен-редактор), шрифты как ассеты, базовые шаблоны/заготовки (импорт).
- [ ] Тесты: CRUD settings, мета в контракте/HTML.

### Слайс R11 — ui-runtime: рендер по листу и route-группы

- [x] F(runtime): `PageRenderer`/`ElementNode` рендер по листу вместо дерева (backend.md §3). *(К4: ui-runtime 262/262.)*
- [ ] F(runtime): regex-группы роута → `{{route.$n}}`/BindingSource.routeGroup → props (R10).
- [ ] F(runtime): `RouteOutlet`/initialPage; типы контракта расширены (siteSettings, list).
- [ ] F(runtime): операции/формы — уже есть через ApiClient; синхронизация с CRUD-дескрипторами.
- [ ] Обновить `docs/ui-runtime/spec.md`; тесты рантайма.

### Слайс R12 — Удаление старой модели и e2e

- [x] Удалить: `EditorPage`, `TreePanel`, `Inspector`, `tree-utils`, tree-модель слоя данных, старые превью-сторы. *(К5: port выполнен; tree-utils → list-utils.)*
- [ ] Удалить tree-операции/таблицы бэка; контент/формы/ассеты остаются нетронутыми.
- [ ] e2e: полный сценарий «создать сайт → компонент → страница → выпуск → откат».
- [ ] Финал: состояние документации обновлено, README-раздел админки переписан.

### Слайс R13 — Контент-редактор (дизайнерский workspace) и ассеты в схеме

Центральный экран для дизайнеров (R18/R19).

- [ ] B: schema компонента расширяется контент-слотами: `role: 'content'|'config'`, типы значений
      + `asset`/`asset[]` (kinds), `label`/`required`/`default`, `defaultSource?: BindingSource`;
      авто-вывод из TSX (R5) + ручные уточнения; валидация и миграция модели схемы.
- [ ] B: admin-операции элементов: addElement/moveElement/removeElement/updateElement (поверх updatePage);
      createContentFromElement (авто-создание Content-рекорда, collectionId=componentId, биндинг
      content-слотов `<contentId>.<field>`).
- [ ] B: usage «где используется» расширяется: Element.props (литералы asset/значения), Content.Fields
      всех локалей, Form.Definition, SiteSettings.faviconAssetId.
- [ ] F: workspace «Контент»: слева страницы; внутри страницы — лист элементов (drag = структура);
      клик → предпросмотр-редактор.
- [ ] F: предпросмотр-редактор: live canvas (ui-runtime `PageRenderer` по листу, R11) + схема-зависимые
      контролы, выбор/создание Content-рекорда, переводы (base+locale = «вариативность»).
- [ ] F: биндинг-пикеры: per-слот literal | binding (content/operation/endpoint/form/query/routeGroup).
- [ ] F: ассеты: тип `asset` во всех редакторах (пикер+загрузка+превью), превью-URL и runtime-URL через AssetResolver.
- [ ] Тесты: schema-слоты/валидация, element-мутации, createContentFromElement, workspace
      (композиция/drag/контролы/превью), usage по ассетам.

---

## 9. Завершённые работы вне редизайна (перенесено из корневого TODO)

Задачи, выполненные ранее и закрытые здесь для полноты (перенесены из `TODO.md`/`todo-mvp.md`):
Этапы 0–4b (терминология Vue→React, ui-runtime-пакет, git-модель компонентов, esbuild-сборщик,
runtime-контракт, dependency-сервис), Этап 5 (boot/рендеринг/снапшоты/код-сплит), Этап 6 M0–M3
(монорепо, каркас admin, CRUD, редактор, контент, формы/роуты/ассеты, публикация).

## 10. Оставшиеся работы вне редизайна (из корневого TODO)

Эти пункты перенесены из старого корневого `TODO.md` (Этап 4b/6/7) и остаются актуальными
(не требуют редизайна, но доводят текущую систему):

- **Ликвидация shell-обёрток CI** (`scripts/*.sh`) — `cmd/dependency-build` покрывает react-семью (слайс 8c).
- **SBOM/audit-экспорт** зависимостей (слайс 8d).
- **Общие wire-типы admin↔ui-runtime**: `AssetMeta`/`FormDefinition`/`Route` дублированы в
  `admin/src/runtime/types.ts` и `ui-runtime/src/types/` (низкий приоритет).
- **Development/Production изоляция**: не мешают друг другу; перенос между ними только через снапшот.
- **Site isolation**: один сайт не зависит от состояния другого.
- **Логирование/метрики Build**, документация архитектуры обновлена.