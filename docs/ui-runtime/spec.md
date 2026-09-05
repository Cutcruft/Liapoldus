# ui-runtime — спецификация

`@liapoldus/ui-runtime` — библиотека-рантайм, которую используют компоненты сайта (и частично конструктор). Backend не генерирует код: он публикует **JSON-контракты**, а ui-runtime парсит их и превращает в живые запросы, подписки, дерево страницы, дизайн-токены, роутинг и локализацию.

Этот документ — проектирование (стадия «спек»). Реализации нет; код, описанный ниже, является контрактом, под который пишутся тесты (`docs/ui-runtime/test-spec.md`).

---

## 1. Принципы

1. **Backend не генерирует код.** Все структуры, которые рантайм использует для работы (операции, провайдеры, эндпоинты, дерево, токены, роуты), приходят как плоские JSON-дескрипторы.
2. **Структура и данные разделены.** Изменение структуры страницы (добавили/убрали компонент, поменяли наполненность) → **пересборка дерева**. Изменение только контента/данных инфраструктуры → **синхронизация, без пересборки**.
3. **Компонент не знает протокола.** Компонент описывает только представление и ожидаемую структуру данных. Протокол (HTTP/WS/SSE/GraphQL) выбирает вычисление по дескриптору. Компонент — это шаблон: весь текст и данные приходят из контента.
4. **Контент и локализация — одно.** Компонент — шаблон; всё остальное — контент, описанный текстом с вариативностью по языкам (см. §7c). Отдельного механизма UI-строк нет.
5. **Перерисовки через zustand.** Данные рантайма живут в zustand-сторе; перерисовываются только подписанные части дерева компонентов.
6. **Browser-first, но с server-only исключением.** Всё, что отдано браузеру, — публично. Приватные операции (секреты, локальная БД, webhook, admin) — только через server-only endpoint на нашем backend.
7. **Поллинг — механизм по умолчанию.** Данные (контент) обновляются **short polling** — обычным HTTP, расписание — **cron-выражение** в контракте. Push (SSE/WS) — только там, где нужна мгновенная реакция (live-режим конструктора, двусторонние чаты). Уведомление о структурном изменении никогда не идёт по поллингу данных.
8. **Роутинг единый.** Одна таблица RouteDescriptor обслуживает и клиентскую навигацию (React), и серверные запросы (edge): отдача фронтенда, статических файлов/ассетов, редиректов (см. §9/§9a).

---

## 2. Слои пакета

```
ui-runtime/
├── package.json            name: @liapoldus/ui-runtime
├── src/
│   ├── index.ts            публичный экспорт
│   ├── types/
│   │   ├── descriptor.ts   типы JSON-дескрипторов (см. json-descriptors.md)
│   │   ├── tree.ts         TreeDeclaration / ComponentInstance / Binding
│   │   ├── route.ts        RouteDescriptor / RouteAction
│   │   ├── content.ts      ContentData / ContentLocaleMerge / StringsCollection
│   │   ├── asset.ts        AssetMeta / AssetVariant / AssetRef
│   │   ├── form.ts         FormDefinition / FieldSchema / ValidationResult
│   │   └── tokens.ts       ThemeDescriptor
│   ├── core/
│   │   ├── registry.ts     Registry: операции, провайдеры, эндпоинты
│   │   ├── boot.ts         загрузка контракта сайта и его регистрация
│   │   ├── transport/
│   │   │   ├── transport.ts   Transport interface + TransportFactory
│   │   │   ├── http.ts        HttpFetchTransport
│   │   │   ├── ws.ts          WebSocketTransport
│   │   │   ├── sse.ts         SseTransport
│   │   │   └── graphql.ts     GraphqlTransport
│   │   ├── builtin/
│   │   │   ├── content.ts     шаблонные запросы content (get/list/batch, ?locale, base+overlay)
│   │   │   ├── assets.ts      шаблонные запросы ассетов (get/list/batch, метаданные + варианты)
│   │   │   └── forms.ts       шаблонные запросы форм (get/submit)
│   │   ├── api-client.ts      единый клиент запросов (query/mutate/callEndpoint/builtin)
│   │   ├── poll-scheduler.ts  PollScheduler: cron → short polling по HTTP
│   │   ├── sync.ts            SyncEngine: поллинг(cron) + push(ws/sse) + подписки
│   │   ├── i18n.ts            I18n: locale-детекция, переводы из контента (коллекция strings)
│   │   ├── content.ts         ContentController: merge по locale, резолв AssetRef → URL
│   │   ├── assets.ts          AssetResolver: метаданные + варианты, url(ref)
│   │   ├── form.ts            FormRuntime: схема, клиентская валидация, submit
│   │   ├── tree.ts            TreeController: mount / rebuild / bindings
│   │   ├── routing.ts         Router: matcher → action (клиентская навигация)
│   │   ├── edge.ts            RequestRouter: серверное применение той же таблицы роутов
│   │   ├── tokens.ts          DesignTokens: применение CSS-переменных
│   │   └── store.ts           zustand-стор рантайма
│   ├── react/
│   │   ├── provider.ts        RuntimeProvider
│   │   ├── hooks/             useQuery/useMutation/useContent/useAsset/useDesignToken/useRoute/useTree/useLocale/useT/useForm
│   │   └── render/
│   │       ├── PageRenderer.tsx  рендер дерева из декларации
│   │       └── RouteOutlet.tsx   рендер по текущему роуту
│   └── errors.ts             ErrorCatalog
└── tests/unit/               спек тестов по слоям (vitest)
```

---

## 3. Жизненный цикл (Boot)

```
boot(siteId, environment, version?, locale?) →
  1. builtin-контракт:        GET /runtime/contract (провайдеры, операции, эндпоинты, роуты, токены)
  2. Registry.registerProviders / registerOperations / registerEndpoints
  3. DesignTokens.apply(theme)
  4. Router.addRoutes(routes)        — единая таблица (+RequestRouter на сервере использует ту же)
  5. I18n.detect(locale?)            — «localStorage → navigator.language → defaultLocale»
  6. TreeController.load(tree.get)   — первая декларация дерева
  7. SyncEngine.schedule(из контракта) — poll-расписания content (locale-aware, cron), без пересборки
  8. Store → ready
```

Загрузка привязана к версии/снапшоту: адрес контракта включает `siteId`, `environment` и опционально `versionId`. Если версия не указана — берётся текущая published Билда окружения.

### Пересборка vs данные

| Событие                                                          | Реакция                                                         |
| ---------------------------------------------------------------- | --------------------------------------------------------------- |
| Изменена структура дерева (страница пересобрана на backend)      | `TreeController.rebuild(nextDeclaration)` — ГЛОБАЛЬНАЯ пересборка |
| Опубликован новый роут                                            | `Router.addRoutes` / `Router.replaceRoutes`                     |
| Изменился дизайн-токен темы                                       | `DesignTokens.apply`                                            |
| Изменился content / локализация / данные инфраструктуры          | **short polling по cron** (HTTP) или push; ЗАДЕЙСТВУЕТСЯ только binding |
| В Dev: страница пересобрана в конструкторе                       | push (WS) новой декларации → rebuild                              |
| В Prod: опубликован новый снапшот → новая версия Build           | приложение перезагружает контракт при инвалидации               |

---

## 4. Ядро — Registry

```ts
type OperationKind = 'query' | 'mutation';

interface ResolvedOperation extends OperationDescriptor {
  provider: ProviderDescriptor;   // резолвится по providerId
  transport: Transport;           // инстанс по протоколу провайдера
}

class Registry {
  registerProviders(ds: ProviderDescriptor[]): void;
  registerOperations(ds: OperationDescriptor[]): void;
  registerEndpoints(ds: EndpointDescriptor[]): void;

  getProvider(id: string): ProviderDescriptor;
  getOperation(id: string): ResolvedOperation;
  getEndpoint(id: string): EndpointDescriptor;
  hasOperation(id: string): boolean;
  drop(): void;                     // полный сброс (смена сайта/версии)
}
```

Правила:
- дубликат `id` при регистрации → бросает `DuplicateRegistrationError`;
- обращение к неизвестному `id` → `UnknownEntityError`;
- операция с `providerId`, которого нет в реестре → ошибка при регистрации (`UnknownProviderError`).

---

## 5. Транспорт

Интерфейс единый для всех протоколов:

```ts
type TransportProtocol = 'http' | 'ws' | 'sse' | 'graphql';

interface TransportRequest<TOutput = unknown, TInput = unknown> {
  operation: OperationDescriptor;
  provider: ProviderDescriptor;
  input?: TInput;
}

interface Transport {
  readonly protocol: TransportProtocol;
  request <TOutput = unknown, TInput = unknown>(
    req: TransportRequest<TOutput, TInput>,
  ): Promise<TOutput>;

  /** опционально: push-подписка (ws/sse) */
  subscribe? <T>(
    req: TransportRequest<T>,
    onData: (data: T, meta: SubscriptionMeta) => void,
  ): () => void;
}

interface TransportFactory {
  create(provider: ProviderDescriptor): Transport;
}
```

### Встроенные реализации

| Transport             | protocol  | Назначение                                                |
| --------------------- | --------- | -------------------------------------------------------- |
| `HttpFetchTransport`  | http      | базовый fetch: method/path + query/body + headers/timeout |
| `WebSocketTransport`  | ws        | двусторонний поток: запросы по `method`, ответы по `correlationId` |
| `SseTransport`        | sse       | `EventSource` для длинных потоков событий                 |
| `GraphqlTransport`    | graphql   | операция из поля `query`/`variables` дескриптора          |

Поллинг выполняется через `HttpFetchTransport` (обычный HTTP); WS/SSE — только для явных push-подписок. Общая семантика ошибок: транспорт выбрасывает `TransportError` с `code` (см. `errors.ts`). HTTP-статусы доступны через `cause`.

### Шаблонные запросы к нашему backend (builtin)

Провайдер `liapoldus.builtin` регистрируется автоматически при boot. Это операции с готовыми дескрипторами:

| id                     | method/path                          | Назначение                        |
| ---------------------- | ------------------------------------ | --------------------------------- |
| `content.get`          | GET `/api/contents/{contentId}?locale=ru` | данные контента для локали (base + overlay, серверный фолбэк) |
| `content.list`         | GET `/api/sites/{siteId}/contents?locale=ru` | список контентов сайта под локалью |
| `content.batch`        | POST `/api/sites/{siteId}/contents/batch?locale=ru` | массив id → `{[id]: ContentData}` под локалью |
| `asset.get`            | GET `/api/assets/{assetId}`          | метаданные ассета (мастер + варианты) |
| `asset.list`           | GET `/api/sites/{siteId}/assets`     | список метаданных ассетов сайта    |
| `asset.batch`          | POST `/api/sites/{siteId}/assets/batch` | массив id → `{[id]: AssetMeta}`   |
| `form.get`             | GET `/api/forms/{formId}`            | определение формы (схема/валидация/submit-конфиг) |
| `form.submit`          | server-only POST `/api/forms/{formId}/submissions` | запись **raw JSON** в таблицу форм при submit |
| `tree.get`             | GET `/runtime/tree`                  | декларация дерева страницы       |
| `tokens.get`           | GET `/runtime/tokens`                | дизайн-токены темы               |
| `routes.get`           | GET `/runtime/routes`                | единая таблица роутов (клиент + edge) |

Все builtin-операции type-bound: `content.get` всегда возвращает `ContentData` (уже смёрженные под локаль поля), `asset.get` — `AssetMeta`, `form.get` — `FormDefinition`, `form.submit` принимает на вход любую `ContentData`-структуру (raw JSON) и не валидирует ничего сверх обязательного `{ formId }` на уровне транспорта.

Операции, предназначенные для поллинга (`content.get`, `content.list`, `content.batch`), могут нести `poll: { schedule }` — расписание cron указывается в контракте. Дизайн-дескриптор `poll` описан в `json-descriptors.md` §2. Ассеты — имutable: грузятся on-demand и кэшируются, поллинга нет. UI-строки — обычный контент (резервная коллекция `strings`), поллятся как контент.

---

## 6. Единый API-клиент

```ts
class ApiClient {
  query <TInput, TOutput>(operationId: string, input?: TInput): Promise<TOutput>;
  mutate<TInput, TOutput>(operationId: string, input?: TInput): Promise<TOutput>;

  callEndpoint<TInput, TOutput>(endpointId: string, input?: TInput): Promise<TOutput>;

  builtin: BuiltinClient;   // typed: content.get/list/batch, asset.get/list, form.get/submit

  subscribe <T>(operationId: string, onData: (d: T) => void): () => void;
  poll <T>(operationId: string, onData: (d: T) => void, opts?: SyncOptions): () => void;
}
```

`query` vs `mutate` — проверка против `type` операции из реестра: `mutate('x')` для `type: query` → `OperationKindMismatchError`.

---

## 7. Синхронизация данных (SyncEngine)

Слой отделяет **данные** (могут поллиться) от **структуры** (только rebuild). Push-уведомления не попадают в поллинг.

```ts
type Channel = 'poll' | 'sse' | 'ws';

interface SubscriptionMeta {
  timestamp: number;
  channel: Channel;
}

interface SyncOptions {
  schedule?: string;      // cron-выражение (см. PollScheduler), если нужен poll
  immediate?: boolean;    // выполнить первый тик сразу (default true)
  channel?: Channel;      // принудительный канал
}

class SyncEngine {
  poll<T>(operationId: string, onData: (d: T, meta) => void, opts?: SyncOptions): () => void;
  subscribe<T>(operationId: string, onData: (d: T, meta) => void, opts?: SyncOptions): () => void;
}
```

Выбор канала:

1. задан `opts.channel` → используем только его (poll/sse/ws);
2. у провайдера есть `subscribe` (ws/sse), и в этом провайдере/операции объявлен push (`push: true`) → push;
3. иначе → **short polling** по `schedule` из дескриптора (`poll.schedule`), при отсутствии — по `opts.schedule`.

Поллинг — это обычный HTTP-запрос каждого тика; расписание — cron-выражение. Результат кладётся в стор (content/operationResults), запросы идут с текущим `locale`; **`store.tree` поллинг не трогает никогда**.

### PollScheduler

```ts
interface PollJob {
  readonly id: string;
  cancel(): void;
}

class PollScheduler {
  add(schedule: string, tick: () => void, opts?: {
    immediate?: boolean;         // default true
    errorHandler?: (e: Error) => void;
  }): PollJob;
  size(): number;
  clear(): void;
}
```

- Формат cron: **6 полей** `second minute hour day-of-month month day-of-week` (секунды нужны для коротких интервалов, например `*/5 * * * * *` — каждые 5 секунд).
- Каждый тик — единичное планирование следующего запуска (без дрейфа); перекрытие с прошлым, если tick долгий, игнорируется (нет накопления).
- Невалидное выражение → `CronParseError`.
- `add(..., {immediate:true})` выполняет первый tick немедленно.

---

## 7a. I18n — локализация

Язык — runtime-состояние, не часть роутера. Источники: **localStorage** (персистентный) → **авто-определение** (`navigator.language` / `Accept-Language`) → **defaultLocale**.

Локализация — часть контента: UI-строки живут в резервной коллекции контента `strings` (ид = ключ), а не в отдельной l10n-сущности. `t(key)` читает из локализованного контента (сервер уже применил фолбэк, §7c).

```ts
interface I18nOptions {
  defaultLocale: string;          // язык по умолчанию (из контракта/темы)
  supportedLocales?: string[];
  stringsCollection?: string;     // резервная коллекция UI-строк, default 'strings'
  storageKey?: string;            // default 'liapoldus.locale'
}

class I18n {
  constructor(opts: I18nOptions, store, { storage, navigatorLike });

  detect(): void;                 // localStorage → navigator.language → defaultLocale
  getLocale(): string;
  setLocale(locale: string): void;   // normalize + store + localStorage + notify + переспрос контента
  onLocaleChange(cb: (locale: string) => void): () => void;

  resolveLocale(candidate: string): string;   // 'ru-RU' → 'ru'; неизвестный → defaultLocale
  t(key: string, params?: Record<string, string|number>): string;  // из контента коллекции strings
  strings(): Record<string, string>;          // все строки текущей локали
}
```

- `normalize`/`resolveLocale`: `ru-RU` → `ru`, регистронезависимо; если `supportedLocales` не пуст и кандидата нет → `defaultLocale`.
- Параметры `t('hello, {name}', {name:'Liapoldus'})` — подстановка.
- Словарь наполняется из `content.list({ collectionId: 'strings', locale })` (поллингом по cron как обычный контент). Серверный фолбэк гарантирует: строка всегда готова к показу; клиентский fallback-цепочки нет.
- `setLocale` вызывает повторную синхронизацию локализованного контента (без пересборки дерева).
- Метод `detect` вызывается в boot; `setLocale` доступен как метод рантайма (переключение языка в рантайме).

---

## 7b. Контент и локализация (ContentController)

Компонент — шаблон; всё текстовое — контент с вариативностью по языкам. **Base + overlay translation**:

```text
Content (серверное хранилище)
    contentId: "hero.title"
    collectionId: "species"            // группа контента (статьи, баннеры, strings, ...)
    fields:  { "text": "Hello!" }      // базовый язык = defaultLocale
    translations:
        ru: { "text": "Привет!" }      // overlay: поля, переопределённые для ru
        de: { "text": "Hallo!" }
    → content.get({ id, locale:"ru" }) → { "text": "Привет!" }   // server merge: base + overlay
    → content.get({ id, locale:"fr" }) → { "text": "Hello!" }    // серверный фолбэк на base
```

Правила:
- **Серверный фолбэк**: запрос с `?locale` всегда возвращает готовый к показу текст — сервер склеивает `fields` + `translations[locale]` (поля без перевода остаются как в base). Клиент не гоняет fallback-цепочки.
- `translations[locale]` хранит только переопределения (overlay), не полные копии.
- Без `?locale` сервер возвращает base (defaultLocale).
- Поля могут содержать вложенные ссылки на ассеты: `{ "image": { "assetId": "...", "variant": "thumb" } }` — резолвятся в URL (см. §7d).
- Порядок локалей в runtime: строки интерфейса (`strings`) и данные контента запрашиваются под один текущий locale.

```ts
class ContentController {
  get(contentId: string, opts?: { locale?: string }): Promise<ContentData>;
  list(opts?: { collectionId?: string; locale?: string }): Promise<ContentData[]>;
  batch(ids: string[], opts?: { locale?: string }): Promise<Record<string, ContentData>>;

  // резолв AssetRef → URL внутри смёрженных данных (по схеме полей/глубоким обходом)
  content: () => Record<string, ContentData>;
}
```

- Результат кладётся в `store.content` (ключ — `contentId`), значения — уже смёрженные под текущий locale.
- Смена локали → переспрос локализованного контента через синхронизацию; **дерево не пересобирается**.
- Поля со ссылкой `{ assetId }` при обращении к компоненту резолвятся в строковые URL (см. §7d).

---

## 7c. Ассеты (AssetResolver)

`Asset` — статический ресурс (изображение, SVG, шрифт, видео, документ). В контенте хранится ссылка `{ assetId, variant? }` (**не URL**); URL определяется рантаймом.

```ts
interface AssetRef { assetId: string; variant?: string; }     // поле внутри ContentData

interface AssetVariant {
  name: string;                  // 'master' | 'thumb' | 'full' | ...
  url: string;                   // публичный URL байтов файла
  mime: string;
  size: number;
  width?: number;
  height?: number;
}

interface AssetMeta {
  id: string;
  variants: AssetVariant[];      // минимум один (master)
  mime: string;                  // совпадает с master
  size: number;
}

class AssetResolver {
  get(assetId: string, opts?: { force?: boolean }): Promise<AssetMeta>;   // asset.get + кэш
  url(ref: AssetRef): string | undefined;     // выбор variant (default 'master')
  resolveDeep(data: ContentData): ContentData;  // глубокий обход: { assetId } → URL
}
```

- `asset.get` отдаёт метаданные (мастер + варианты) — не байты. Байты отдаёт сервер по `variants[].url` (и по route-правилу `serveAsset`, см. §9a).
- Вариант выбирается **явно в контенте** (`image: { assetId: "a1", variant: "thumb" }`); компонент получает строку URL.
- `resolveDeep` делает глубокий обход смёрженного контента: любой объект вида `{ assetId, variant? }` заменяется на url соответствующего варианта (default `master`). Неизвестный assetId → значение остаётся как есть (ссылка не резолвится, ошибка не бросается).
- Ассеты имutable (cache policy `immutable`): грузятся on-demand, кэшируются в `store.assets`, поллинга нет.

---

## 7d. Формы

Форма — спец-`ComponentDefinition`. Поведение:

- **Схема/поля/валидация** — дизайн-тайм объект, runtime получает через `form.get`.
- **Валидация** происходит **только на клиенте** (по схеме из `form.get`).
- **Submit** — единственная запись на сервер: `form.submit` (server-only) пишет **raw JSON** в отдельную таблицу форм. Никакого серверного состояния формы/поллинга формы нет.
- Break привязок: `BindingSource { type: 'form' }` читает текущее значение поля формы в компонент.

```ts
interface FieldSchema {
  name: string;
  type: 'text' | 'email' | 'password' | 'number' | 'select' | 'checkbox' | 'textarea' | 'custom';
  required?: boolean;
  rules?: Rule[];           // minLength/maxLength/min/max/pattern/custom(fn id)
  options?: string[];       // для select
}

interface FormDefinition {
  id: string;
  fields: FieldSchema[];
  validation?: ValidationRule[];           // cross-field
  submit: { target: OperationOrEndpointId; providerId?: string };
}

interface ValidationResult { valid: boolean; errors: Record<string, string[]>; }

class FormRuntime {
  load(formId: string): Promise<FormDefinition>;
  fields(formId: string): FieldSchema[];
  validate(formId: string, values: Record<string, unknown>): ValidationResult;
  submit(formId: string, values: Record<string, unknown>, ctx?: RuntimeContext): Promise<SubmissionResult>;
  reset(formId: string): void;
}
```

- `submit` идёт через `ApiClient.callEndpoint` (endpoint `form.submit`), payload — `{ formId, locale?, values }` (raw JSON, без серверной структуры сверх этого).
- Идентичные формы на разных страницах — один `formId`, общее определение.

---

## 8. Дерево страницы

```ts
interface BindingSource {
  type: 'content' | 'routeParam' | 'routeQuery' | 'operation' | 'form' | 'props' | 'runtime';
  // составы см. в json-descriptors.md → Binding
}

class TreeController {
  constructor(store, renderer);              // renderer — React-слой

  load(declaration: TreeDeclaration): void;  // первичная загрузка
  rebuild(next: TreeDeclaration): void;      // ТОЛЬКО структурное изменение
  updateBindings(patch: Record<string, unknown>): void;  // данные, без rebuild
  get root(): ComponentInstance;
  onRebuild(cb: (d: TreeDeclaration) => void): () => void;
}
```

- `rebuild` сравнивает декларации; если дерево идентично → no-op (короткое замыкание).
- `updateBindings` — обновление по `instanceId`, задействует только затронутый подграф.
- Поллинг/подписки никогда не вызывают `rebuild`: значения кладутся в стор, binding пересчитывается через `updateBindings`.

---

## 9. Роутинг (единая таблица для клиента и сервера)

Одна и та же таблица `RouteDescriptor[]` используется **двумя** системами:

- **Router (клиент, React)** — навигация в браузере;
- **RequestRouter (edge/backend)** — обработка входящего HTTP-запроса.

Actions:

```ts
type RouteAction =
  | { type: 'renderPage'; pageId: string }                       // SPA: рендер страницы
  | { type: 'serveAsset'; assetId: string }                      // статический файл по роуту
  | { type: 'redirect'; target: string; status?: 301|302|307|308; keepQuery?: boolean };
                                                               // default status: 301, keepQuery: false

interface ResolvedRoute {
  route: RouteDescriptor;
  params: Record<string, string>;   // захваченные группы regex / query
}

class Router {
  addRoutes(ds: RouteDescriptor[]): void;
  replaceRoutes(ds: RouteDescriptor[]): void;
  match(path: string): ResolvedRoute | null;   // по priority, затем по порядку
  navigate(to: string): void;                  // клиентская навигация
}
```

- matcher = полное регулярное выражение (якоря `^...$`);
- несколько совпадений → берётся наивысший `priority`, при равенстве — порядок регистрации;
- совпадений нет → `RouteNotFoundError`.

### Клиентское поведение Router.navigate

| Action      | Клиент (React)                                                    |
| ----------- | ----------------------------------------------------------------- |
| renderPage  | SPA-рендер `pageId` через `RouteOutlet` (history API)             |
| serveAsset  | **Полный переход** `window.location.assign(path)` — браузер сам браузит сервер, файл отдаёт edge |
| redirect    | Клиентский переход на target; если финальный матч — serveAsset → полный переход |

`status`/`keepQuery` — характеристики HTTP-редиректа на сервере; клиент просто переходит к target (query сохраняет по `keepQuery` только смысл для сервера).

Роутер отвечает **только за навигацию/маршрутизацию**: locale и контент — вне его (через I18n/ContentController). `routeParam`/`routeQuery` используются bindings.

---

## 9a. Серверный роутинг (RequestRouter / edge)

Go-backend применяет ту же таблицу роутов к входящему HTTP-запросу. Почтение очереди:

1. Site ⇨ Environment ⇨ Snapshot ⇨ Build (изоляция §29 README);
2. `match(path)` по единой таблице;
3. выполнение action.

```ts
class RequestRouter {
  match(path: string): ResolvedRoute | null;      // та же таблица, те же правила priority
  handle(req: HttpRequest): HttpResponse;         // dispatch по action
}

type HttpResponse =
  | { status: 200; body: HtmlShell; contentType: 'text/html' }          // renderPage
  | { status: 200; body: AssetBytes; contentType: string; cache: CacheHeaders; etag?: string } // serveAsset
  | { status: 301|302|307|308; location: string }                        // redirect
  | { status: 404 }                                                      // нет матча
```

| Action      | Сервер (edge)                                                                         |
| ----------- | ------------------------------------------------------------------------------------- |
| renderPage  | Отдаёт HTML-оболочку (index.html сборки Environment/Build) + cache-заголовки; контент и дерево грузятся клиентом из `/runtime/*` и `/api/*` |
| serveAsset  | Отдаёт байты ассета (content-type, Cache-Control, ETag); файл назначен роуту — доступен по «красивому» пути |
| redirect    | HTTP-редирект: `Location: target` + `keepQuery` дописывает query к target             |

Статическая сборка не перестраивается на каждый запрос: shell/asset кэшируются, версии — через снапшот (инвалидация при новой публикации).

Примеры стратегий роутов:

```text
^/articles/([0-9]+)$   → renderPage: page.article        # SPA-страница
^/robots.txt$          → serveAsset: asset.robots         # обычный txt, назначен роуту
^/assets/(.+)$         → serveAsset: парится по пути из матча (динамический assetId)
^/old$                 → redirect: { target: "/new", status: 301 }
^/legacy/(.*)$         → redirect: { target: "/modern/$1", keepQuery: true, status: 308 }
```

---

## 10. Дизайн-токены

```ts
class DesignTokens {
  apply(theme: ThemeDescriptor): void;   // пишет CSS-переменные в target scope
  get(name: string): string | undefined;
}
```

- Токены пишутся как `--<name>` в `:root` (глобально) или в `data-theme`/scoped-контейнер;
- одна тема на объект: повторный `apply` заменяет полностью (не смешивает).

---

## 11. Store (zustand)

```ts
interface RuntimeState {
  ready: boolean;
  siteId: string;
  environment: string;
  versionId: string | null;

  tree: TreeDeclaration | null;
  routes: RouteDescriptor[];
  route: ResolvedRoute | null;

  tokens: ThemeDescriptor | null;
  content: Record<contentId, ContentData>;          // уже смёрженные под текущий locale
  assets: Record<assetId, AssetMeta>;               // метаданные ассетов (кэш)
  operationResults: Record<operationId, { status; data?; error? }>;

  locale: string;
  forms: Record<formId, { definition?: FormDefinition; state: Record<string, unknown>; errors: Record<string,string[]>; status: 'idle'|'submitting'|'success'|'error' }>;

  // actions
  setReady(): void;
  setTree(d): void;
  setRoute(r): void;
  applyTokens(t): void;
  setContent(id, data): void;
  setAssets(meta: AssetMeta[]): void;
  setOperationResult(id, res): void;
  setLocale(locale): void;
  setFormState(formId, patch): void;
}
```

Стор один и общий для всех вкладок модуля; хуки выбирают только свой срез → точечные перерисовки.

---

## 12. React-слой

```tsx
<RuntimeProvider runtime={runtime}>      // контекст + готовность
  <RouteOutlet />
</RuntimeProvider>
```

Хуки:

```ts
function useQuery<TInput, TOutput>(operationId, input?, opts?)   // подписка на результат
function useMutation<TInput, TOutput>(operationId)               // возвращает { mutate, status }
function useContent(id): ContentData | undefined             // смёрженный под locale контент
function useAsset(ref: AssetRef | string): string | undefined // URL варианта ассета
function useCurrentLocale(): string
function useT(): (key, params?) => string
function useDesignToken(name): string | undefined
function useRoute(): ResolvedRoute | null
function useTree(): TreeDeclaration | null
function useForm(formId): UseFormReturn          // { values, errors, register, handleSubmit, status }
function useReady(): boolean
```

- `useQuery` при изменении `input` перезапрашивает; при `opts.schedule` — short polling по cron.
- `useContent`/`useAsset` подписаны на стор; контент обновляется поллингом (с текущим locale) без пересборки дерева; смена locale → перезапрос контента.
- `useT` читает строки из контента (коллекция `strings`) под текущим locale.
- `useForm` — тонкая обёртка над `FormRuntime` (client-side validation + submit).

---

## 12a. ComponentRegistry (определения компонентов)

```ts
class ComponentRegistry {
  registerDefinition(id: string, component: ComponentDefinition): void;
  getDefinition(id: string): ComponentDefinition;
  hasDefinition(id: string): boolean;
}
```

- `ComponentDefinition` — React-компонент (исходники `.tsx`), который знает только как рендерить; `metadata/schema` — отдельные дескрипторы, в рантайм не зашиты.
- `PageRenderer` по `instanceId → definitionId` забирает компонент из реестра и передаёт `props` + резолвенные binding-значения.
- неизвестный `definitionId` → `ComponentNotFoundError` (рендер placeholder, не падение всего дерева).

---

## 13. Ошибки (ErrorCatalog)

| Error                     | Код                    | Условие                                       |
| ------------------------- | ---------------------- | --------------------------------------------- |
| `DuplicateRegistrationError` | `duplicate_registration` | повторная регистрация id                     |
| `UnknownEntityError`      | `unknown_entity`       | обращение к несуществующему операции/провайдеру/эндпоинту |
| `UnknownProviderError`    | `unknown_provider`     | операция ссылается на несуществующий providerId |
| `OperationKindMismatchError` | `operation_kind_mismatch` | mutate для query-операции и наоборот        |
| `RouteNotFoundError`      | `route_not_found`      | ни один роут не совпал                        |
| `TransportError`          | `transport_error`      | сбой транспорта (код HTTP/WS в `cause`)      |
| `DescriptorValidationError` | `descriptor_validation` | JSON-дескриптор не прошёл валидацию схемы   |
| `ThemeNotFoundError`      | `theme_not_found`      | запрошен неизвестный токен/тема              |
| `SubscriptionError`       | `subscription_error`   | канал подписки недоступен                     |
| `ScopeError`              | `scope_error`          | server-only endpoint вызван из public-контекста компонента |
| `ComponentNotFoundError`  | `component_not_found`  | `definitionId` не зарегистрирован в ComponentRegistry |
| `CronParseError`          | `cron_parse`           | невалидное cron-выражение поллинга           |
| `FormValidationError`     | `form_validation`      | submit формы поверх клиентских ошибок        |
| `LocaleUnsupportedError`  | `locale_unsupported`   | `setLocale` неизвестной локали в strict-режиме |
| `AssetNotFoundError`      | `asset_not_found`      | `asset.get`/`url(ref)` для неизвестного assetId |

Все ошибки — класс на базе `RuntimeError extends Error` с полем `code`.

---

## 14. Валидация дескрипторов

Каждый дескриптор при регистрации проходит валидацию (структурная проверка + типы):

- `DescriptorValidationError` с указанием `entityId` и пути поля;
- неизвестный `protocol` → ошибка;
- `type` операции ∈ {query, mutation};
- **`poll.schedule` должен быть валидным cron** → иначе ошибка;
- `serveAsset.assetId` должен быть валидным id (непустым; наличие в реестре ассетов не проверяется на клиенте);
- `redirect.status` ∈ {301, **302, 307, 308**}; `target` — непустая строка;
- option знание путей на провайдеры, эндпоинты, страницы.

Валидация выполняется на этапе `register*`, до сохранения в реестр.

---

## 15. Связь с README

| README-объект                | ui-runtime слой                                              |
| ---------------------------- | ------------------------------------------------------------ |
| ComponentDefinition          | `ComponentRegistry`: `registerDefinition(id, component)` / `getDefinition(id)`; рендер по `definitionId`. Metadata/schema — отдельные JSON-дескрипторы. |
| ComponentInstance            | `TreeDeclaration` → `PageRenderer`                           |
| Binding                      | `BindingSource` → `TreeController.updateBindings`/синхронизация данных |
| Operation / Query / Mutation | `OperationDescriptor` → `Registry` → `ApiClient.query/mutate` |
| Provider                     | `ProviderDescriptor` → `TransportFactory` → встроенные Transport |
| Endpoint                     | `EndpointDescriptor` → `ApiClient.callEndpoint` (server-only) |
| Content / ContentTranslation | контент — текст с вариативностью по языкам (base + overlay, серверный фолбэк); builtin `content.get/list/batch` с `?locale`; переводы = часть контента (не отдельная сущность) |
| Asset                        | `AssetMeta` (мастер + варианты) → `AssetResolver.url(ref)`; в контенте — `{ assetId, variant }`; байты по `variants[].url` или через route `serveAsset` |
| Form                         | `FormDefinition` (схема/валидация) + `FormRuntime` (client-validation, submit) |
| Route                        | единая таблица `RouteDescriptor` → `Router` (клиент) + `RequestRouter` (edge). Actions: renderPage / serveAsset / redirect(status+keepQuery) |
| Theme                        | `ThemeDescriptor` → `DesignTokens`                           |
| Version / Snapshot / Build   | контракт привязан к `versionId`; обновление контракта на уровне boot |
| Environment                  | параметр `boot(environment)`; dev/prod пересборка контракта; edge отдаёт shell Build окружения |
| Cache                        | policy в `OperationDescriptor.cache`; живёт на стороне транспорта/браузера; ассеты — immutable |
| Localization                 | `I18n`: defaultLocale + авто-определение + localStorage; строки — контент коллекции `strings` под текущим locale |
| Plugin                       | npm-модуль, регистрирующий Provider/компоненты/metadata    |