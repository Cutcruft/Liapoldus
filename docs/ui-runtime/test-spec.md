# ui-runtime — спек тестов

Набор тестов — **контракт**, под который пишется вся реализация (`docs/ui-runtime/spec.md`). Каждый тест задаёт поведение; реализация считается готовой, когда все тесты зелёные.

## Общие требования

- Runner: **vitest**, TS strict.
- Слой `core` тестируется без React (чистые unit-тесты).
- Транспортах протоколов исполняются через заглушки (fake `fetch`, fake `WebSocket`, fake `EventSource`).
- Все ошибки проверяются через `code` (см. ErrorCatalog).
- Дескрипторы — фикстуры; один набор общих фикстур в `tests/unit/fixtures.ts`.

---

## 1. `registry.test.ts`

Фикстуры: 2 провайдера (`cms.http`, `push.ws`), 3 операции (`articles.list` query/http, `article.subscribe` query/ws, `contact.submit` mutation/http server-only), 1 endpoint.

1. `registerProviders` добавляет провайдеров; `getProvider` возвращает правильный дескриптор.
2. повторная регистрация провайдера с тем же `id` → `DuplicateRegistrationError`.
3. `registerOperations`; `getOperation` резолвит `provider` и `transport` по `providerId` (transport инстанс нужного протокола).
4. операция с `providerId`, отсутствующим в реестре → `UnknownProviderError` при регистрации.
5. `getOperation('unknown')` → `UnknownEntityError`.
6. `registerEndpoints`; `getEndpoint` возвращает дескриптор; повторный `id` → `DuplicateRegistrationError`.
7. `hasOperation` → true/false корректно.
8. `drop()` очищает реестр; после `drop` все `get*` → `UnknownEntityError`.
9. дескриптор с невалидным `protocol` (например `"ftp"`) → `DescriptorValidationError` с указанием `entityId`.
10. дескриптор операции с `type: "other"` → `DescriptorValidationError`.
11. endpoint `operationId`, ссылающийся на публичную (`scope: public`) операцию → `DescriptorValidationError`.

---

## 2. `descriptor.test.ts` (парсинг/валидация JSON)

1. полный provider-descriptor парсится с типизацией (protocol/baseUrl/defaults).
2. missing `id` → `DescriptorValidationError` (путь `$.id`).
3. `baseUrl` — не-URL строка → `DescriptorValidationError`.
4. operation-descriptor: отсутствует `type` → `DescriptorValidationError`.
5. operation `scope: "weird"` → `DescriptorValidationError`.
6. route-descriptor: matcher без якорей (`/articles/[0-9]+`) → `DescriptorValidationError`.
7. route-descriptor: `action.type: "frobnicate"` → `DescriptorValidationError`.
8. route-descriptor: redirect без `target` → `DescriptorValidationError`.
9. binding с `source.type: "database"` (не из списка) → `DescriptorValidationError`.
10. theme-descriptor: токен без `value` → `DescriptorValidationError`.
11. `parseDescriptors(json)` на битом JSON → `DescriptorValidationError`.
12. суррогатный кейс: пустой массив операций — валиден (пустая система).

---

## 3. `transport.test.ts` (фабрика)

1. `create({protocol:'http'})` → `HttpFetchTransport`.
2. `create({protocol:'ws'})` → `WebSocketTransport`.
3. `create({protocol:'sse'})` → `SseTransport`.
4. `create({protocol:'graphql'})` → `GraphqlTransport`.
5. `create({protocol:'unknown'})` → `DescriptorValidationError`.
6. `create(провайдер с default timeout)` — таймаут пробрасывается в transport-инстанс.

---

## 4. `transport-http.test.ts`

Заглушки: глобальный `fetch` (fake), `AbortController`.

1. GET: параметры `params.in=query` → в URL-search-параметры; тело пустое.
2. GET: `params.in=path` → подставляется в `path` (placeholder `:name` / `{name}`).
3. POST (mutation): `input` → в JSON-body, `Content-Type: application/json`.
4. дефолтные headers провайдера добавляются к запросу.
5. кастомные headers операций сливаются с default (операция побеждает).
6. таймаут: задержка ответа > timeout → `TransportError` (code `transport_error`), запрос отменён (abort signal).
7. успешный ответ → распарсенный JSON.
8. ответ не-JSON → `TransportError`.
9. HTTP 404 → `TransportError` c `cause.status = 404`.
10. HTTP 409 → `TransportError` c `cause.status = 409`.
11. HTTP 5xx → `TransportError`.
12. network failure (reject) → `TransportError`.
13. `baseUrl` отсутствует → используется относительный URL на базе host (не абсолютный).

---

## 5. `transport-ws.test.ts`

Заглушки: fake `WebSocket` (класс с `send`, `close`, `onopen/onmessage/onerror/onclose`).

1. `request` открывает соединение и отправляет сообщение с `correlationId`.
2. ответ с совпадающим `correlationId` → resolve.
3. ошибка от сервера (поле `error`) → `TransportError`.
4. соединение не открылось (onerror) → `TransportError`, без зависания.
5. timeout ожидания ответа → `TransportError`.
6. `subscribe` — подписка на события `eventName`, отписка по возвращённой функции.
7. повторный `request` переиспользует открытый сокет (не открывает второй).
8. `close()` — корректное закрытие и отмена всех ожидающих запросов.

---

## 6. `transport-sse.test.ts`

Заглушки: fake `EventSource`.

1. `subscribe` открывает EventSource на URL (baseUrl + path/eventName).
2. событие с `data` (JSON) → `onData` с распарсенными данными.
3. событие с не-JSON `data` → `SubscriptionError` (не падает процесс).
4. `onerror` EventSource → `SubscriptionError` (канал недоступен).
5. `request` (unary) не поддерживается — бросает `SubscriptionError`/`TransportError` с кодом.
6. отписка → `close()` вызывается ровно один раз.

---

## 7. `transport-graphql.test.ts`

1. `request` шлёт POST на `baseUrl` с телом `{ query, variables }` и `Content-Type: application/json`.
2. `variables` берутся из `input` при `variablesMode: "input"`.
3. при `variablesMode: "params"` — из `params`.
4. ответ `data` → resolve.
5. ответ с `errors[]` (GraphQL errors) → `TransportError` с `cause.graphqlErrors`.
6. дефолтные headers провайдера применяются.
7. NaN-проверка: `query` обязателен (дескриптор без `query` → `DescriptorValidationError`).
8. серверная ошибка HTTP (5xx) → `TransportError`.

---

## 8. `builtin.test.ts`

Бэкенд: content (get/list/batch, base + overlay + серверный фолбэк), assets (get/list/batch), forms (get/submit).

1. `content.get(id, {locale:'ru'})` → возвращает `ContentData`, смёрженный под локаль: перевод приоритетнее base.
2. `content.get(id, {locale})` с переводом, покрывающим не все поля → поля без перевода остаются из base (серверный фолбэк).
3. `content.get(id, {locale:'xx'})` без перевода вообще → base (base = готовый текст, фолбэк на сервере).
4. `content.get` без `locale` → base (defaultLocale).
5. `content.get` — правильный URL с `?locale=ru` (`/api/contents/{contentId}?locale=ru`).
6. `content.get` отсутствующего контента → `TransportError`.
7. `content.list(siteId, {locale})` → массив `{id, collectionId, fields}` под локалью.
8. `content.list(siteId, {collectionId:'strings', locale})` → UI-строки (контент коллекции `strings`).
9. `content.batch(siteId, {ids:[...]}, {locale})` → `{[id]: ContentData}` под локалью; отсутствующий id пропускается.
10. `asset.get(assetId)` → `AssetMeta` (мастер + `variants[]`); `asset.get` неизвестного id → `AssetNotFoundError`.
11. `asset.list(siteId)` → массив `AssetMeta`; `asset.batch(ids)` → `{[id]: AssetMeta}`.
12. `form.get(formId)` → `FormDefinition` (схема полей/валидация/submit-конфиг).
13. `form.submit` — server-only: `callEndpoint` только, из компонента → `ScopeError`.
14. builtin-операции регистрируются автоматически при boot (см. boot.test.ts) и не могут быть перезаписаны дубликатом.
15. `builtin`-звонки можно замокать (операция «переопределяется» для теста).

---

## 9. `api-client.test.ts`

1. `query('articles.list', input)` → запрос проходит через нужный транспорт и возвращает результат.
2. `query('unknown')` → `UnknownEntityError`.
3. `mutate('articles.list')` (query-операция) → `OperationKindMismatchError`.
4. `query('contact.submit')` (mutation) → `OperationKindMismatchError`.
5. `callEndpoint('contact.submit', input)` → возвращает результат server-operation.
6. `callEndpoint('articles.list')` (не endpoint) → `UnknownEntityError`.
7. `callEndpoint('contact.submit')` из компонента запрещён: проверяемый код, бросающийся `ScopeError` (см. ErrorCatalog) при вызове из public-контекста.
8. `builtin.content.batch(...)`, `builtin.asset.get(...)`, `builtin.form.get(...)` — делегируют в соответствующие операции.
9. `builtin.form.submit(...)` → `ScopeError` вне server-контекста.
10. `query` c кэш-police `immutable` — второй вызов с тем же input возвращает кэшированный результат (не делает второй запрос к транспорту).
11. `query` с `cache: disabled` — каждый вызов делает запрос.
12. `poll('content.get', handler, {schedule, input:{id, locale}})` стартует short polling (тики по cron, запросы с `?locale`) и отписывается по возвращённой функции.
13. `subscribe('article.subscribe', handler)` с `push:true` → push; возвращённая функция отписывает.

---

## 10. `sync.test.ts` (SyncEngine + подписки)

Поллинг CRON через `PollScheduler`; push (ws/sse) — точечно.

1. `poll` для http-провайдера → поллинг по `schedule` из дескриптора (cron), каждый тик — отдельный HTTP-запрос.
2. `poll` без `poll.schedule` в дескрипторе, но с `opts.schedule` → работает по `opts.schedule`.
3. `poll` без расписания вовсе → `DescriptorValidationError`.
4. `subscribe` для ws-провайдера с `push: true` → push через `transport.subscribe`.
5. `subscribe` для ws-операции без `push: true` → **поллинг** (не ws).
6. первый тик при `immediate: true` (default) — выполняется сразу, до ожидания интервала.
7. поллинг: тик с `TransportError` → подписка продолжает (не падает), ошибка в `errorHandler`.
8. перекрытие тиков (долгий запрос) не накапливается: пока tick выполняется, новый не стартует.
9. отписка: канселит cron-job и push-канал.
10. подписка/поллинг **не трогают** `store.tree` (поле остаётся прежним).
11. `content.list({collectionId:'strings', locale})` (поллинг) обновляет `store.content` для строк под текущим locale.

---

## 10a. `poll-scheduler.test.ts` (cron → short polling)

FakeTimers (витус `vi.useFakeTimers`).

1. валидный cron `*/5 * * * * *` → тик каждые 5 секунд (по часам fake).
2. cron с `0 * * * * *` → тик раз в минуту.
3. невалидный cron (`'* * *'`) → `CronParseError`.
4. `add(schedule, tick, {immediate:true})` → tick вызван сразу.
5. `cancel()` → дальнейшие тики не происходят.
6. `clear()` → все джобы остановлены, `size()` = 0.
7. в 6-полевый cron `day-of-week` = `*` по умолчанию; `5 * * * * MON` корректно планируется один раз в минуту по понедельникам (прогон по нескольким неделям).
8. долгий tick (обещание) не даёт наложения: второй не стартует, пока первый не завершился.

---

## 10b. `i18n.test.ts`

Хранилище — fake localStorage, детективное окружение — fake `navigator.language`. Строки — контент коллекции `strings`.

1. `detect()`: localStorage → navigator.language → defaultLocale.
2. localStorage пустой, `navigator.language='en-US'` → locale `en-US` → normalize `en`.
3. localStorage отсутствует и navigator пуст → defaultLocale.
4. `setLocale('ru-RU')` → локализовано `ru`, записано в localStorage, стop обновлён, триггерит переспрос контента под ru.
5. `resolveLocale('en-GB')` при `supportedLocales: ['en','ru']` → `en`.
6. неизвестная локаль при пустых supportedLocales → принимается, как есть.
7. неизвестная локаль при strict + supportedLocales → `LocaleUnsupportedError`.
8. `t('nav.home')` читает из контента коллекции `strings` под текущим locale (серверный фолбэк уже применён).
9. `t` для ключа, отсутствующего и в base → сырой ключ (не падает).
10. `t('hello', {name:'Liapoldus'})` — подстановка параметров.
11. `onLocaleChange` срабатывает при `setLocale`.
12. словари-строки приходят как content: `content.list({collectionId:'strings', locale})` → `store.content`; при смене локали строки перечитываются поллингом.
13. поллинг строк (контент) не вызывает `store.tree` пересборку.

---

## 10c. `asset.test.ts` (AssetResolver)

Бэкенд: `asset.get` → метаданные (мастер + варианты); контент хранит `{ assetId, variant }`.

1. `get('asset-1')` → `AssetMeta` с `variants[]` (минимум master); кэшируется в `store.assets`.
2. `get` во второй раз → берётся из кэша (нет повторного запроса).
3. `get('unknown')` → `AssetNotFoundError`.
4. `url({assetId:'asset-1', variant:'thumb'})` → url варианта `thumb`.
5. `url({assetId:'asset-1'})` без variant → url `master`.
6. `url({assetId:'unknown'})` → `undefined` (не бросает).
7. `url(ref)` с неизвестным variant → `undefined`.
8. `resolveDeep(data)` — поле `{ assetId, variant }` заменяется на строку URL (глубокий обход вложенных объектов/массивов).
9. `resolveDeep`: поле `{ assetId }` без variant → master URL.
10. `resolveDeep`: `{ assetId: 'unknown' }` → значение остаётся как есть (не ломает данные).
11. `resolveDeep`: обычные строки/числа не меняются.
12. после `resolveDeep` в `store.content` — готовые URL; `asset.get` не был вызван для уже закэшированного ассета.

---

## 10d. `form.test.ts`

Бэкенд: `form.get` (определение) + `form.submit` (raw JSON в таблицу форм).

1. `load('form.contact')` → возвращает `FormDefinition` из `form.get`.
2. `fields('form.contact')` → список `FieldSchema`.
3. `validate` корректных значений → `{valid:true, errors:{}}`.
4. `validate` пустого `required` поля → `{valid:false, errors:{email:[...]}}`.
5. `validate` правила `minLength:10` при коротком значении → ошибка.
6. cross-field rule `confirmMatch` при разных password → ошибка.
7. `validate` невалидной схемы поля (type: 'email' с разными строками) — по правилам email.
8. `submit` клиентски-невалидной формы → `FormValidationError`, запрос НЕ выполняется.

(Проверка: fetch не вызван.)

9. `submit` валидной формы → `ApiClient.callEndpoint(form.submit)`, payload = `{formId, locale, submittedAt, values}` (raw JSON).
10. `submit` → возвращает `{submissionId, status:'ok'}`; стор `forms[formId].status = 'success'`.
11. `reset` → состояние формы в сторе очищено (idle, пустые values/errors).
12. после submit сервером не хранится ничего сверх raw JSON: форма клиентская, поллинга форм нет.
13. `form.get` и `form.submit` регистрируются как builtin-операции при boot.

---

## 11. `tree.test.ts`

1. `load(declaration)` кладёт декларацию в стор; `root` доступен.
2. `rebuild` с другой декларацией → эмиттит `onRebuild`, стор `tree` обновлён.
3. `rebuild` с идентичной декларацией → no-op (нет события, нет перерисовки).
4. `updateBindings({i2: {...}})` → обновляет значение, НЕ создаёт событие `onRebuild`.
5. `updateBindings` для несуществующего `instanceId` → `UnknownEntityError`.
6. binding `content` резолвится: `contentId` + `path` из стора → значение в переданной декларации.
7. binding `routeParam` резолвится из текущего роута (params.articleId).
8. вложенные binding sources (`operation`, `form`) резолвятся по заданным правилам.
9. `rebuild` сохраняет unresolved bindings (НЕ ломает дерево при отсутствии данных в сторе).

---

## 12. `routing.test.ts`

Фикстуры: `route.article` (^/articles/([0-9]+)$, prio 10), `route.old` (redirect 301 → /new), `route.legacy` (redirect 308 → /modern/$1, keepQuery), `route.robots` (serveAsset asset.robots), `route.home`.

1. `match('/articles/42')` → `{route: route.article, params:{articleId:'42'}}`.
2. `match('/articles/abc')` → null (regex не совпал).
3. `match('/old')` → redirect-роут (status 301, target /new).
4. приоритет: два роута, более высокий `priority` выигрывает.
5. `navigate('/articles/42')` → обновляет `store.route` и эмитит `renderPage` с `pageId`.
6. `navigate('/old')` → redirect: клиентский переход к `/new` (стор меняет путь).
7. `navigate('/robots.txt')` → serveAsset: **полный переход** `location.assign('/robots.txt')` (SPA не рендерит).
8. `match('/legacy/foo')` → redirect с target `/modern/foo` (группа regex захвачена в target) и `keepQuery: true`.
9. нет совпадений: `match` → null, `navigate` → `RouteNotFoundError`.
10. валидация матчегора без якорей → `DescriptorValidationError`.
11. порядок: равенство priority — решается порядком регистрации (первый).
12. redirect: `status` вне {301,302,307,308} → `DescriptorValidationError`.

---

## 12a. `edge-routing.test.ts` (RequestRouter)

Тот же набор роутов, что в §12, применяется к входящему HTTP-запросу.

1. `handle(GET /articles/42)` → renderPage: `{status:200, body: HtmlShell, contentType:'text/html'}`.
2. `handle(GET /articles/abc)` → no match → 404.
3. `handle(GET /robots.txt)` → serveAsset: `{status:200, body: AssetBytes, contentType:'text/plain'}` + cache-заголовки/ETag.
4. `handle(GET /old)` → redirect: `{status:301, location:'/new'}` (Location без query).
5. `handle(GET /legacy/x?a=1)` с keepQuery → `{status:308, location:'/modern/x?a=1'}`.
6. redirect без keepQuery → query не копируется.
7. match по той же таблице: priority/order идентичны клиентскому Router (одна функция).
8. renderPage отдаёт shell без пересборки фронтенда на каждый запрос (одна и та же HTML-оболочка, cache).
9. serveAsset отдаёт правильный content-type по mime ассета и корректный ETag.

---

## 13. `tokens.test.ts`

1. `apply(theme)` пишет CSS-переменные в target scope (`:root`).
2. токены записываются как `--<name>: <value>`.
3. `apply` дважды (разные темы) → полное замещение (нет остатков старых токенов).
4. `get(name)` возвращает значение; неизвестный → `undefined` (не бросает).
5. тема с `fonts` подключает шрифты (проверка через injected link управляемой тестовой средой).
6. тема с `ref`-токеном резолвится перед применением (референс → конечное значение).
7. тема применяет только валидные токены; невалидный токен → `DescriptorValidationError`.

---

## 14. `store.test.ts`

1. стор создаётся с дефолтным состоянием (`ready:false`, пустые дерево/роуты/токены).
2. `setReady()` → `ready: true`.
3. `setTree`/`setRoute`/`applyTokens`/`setContent`/`setAssets`/`setOperationResult`/`setLocale`/`setFormState` — каждое действие обновляет нужный срез, не трогая остальные (вспомогательная функция, берёт снапшот).
4. `subscribe` на срез `content` реагирует только на изменение content (не срабатывает на setTree).
5. два пользователя этого модуля: изменение `content.c1` не перерисовывает подписчика `tree`.
6. `setLocale` обновляет срез `locale`, не трогая `content`.
7. `setAssets` пишет метаданные ассетов в `store.assets`, не трогая `content`.

---

## 15. `errors.test.ts`

1. каждый класс Error имеет `code` из ErrorCatalog.
2. `RuntimeError` — базовый класс; instanceof-проверка для всех типов.
3. `TransportError` хранит `cause` (status/порт).
4. `toErrorCode(err)` маршаллит код для внешнего слоя.

---

## 16. `boot.test.ts`

1. `boot(siteId, environment)` вызывает `GET /runtime/contract` с правильными параметрами (siteId + environment + versionId опциональн).
2. контракт регистрируется (провайдеры/операции/эндпоинты), `ready: true`.
3. тема применяется через DesignTokens.
4. роуты загружаются через Router.
5. `defaultLocale` из контракта применяется через I18n.detect (ло­кальный детективный источник).
6. если `tree` приходит в контракте — `TreeController.load`.
7. poll-расписания из операций (проявляются в контракте) стартуют через SyncEngine (поллинг `content.get/list` по cron, запросы с текущим `locale`).
8. отсутствие контракта (404) → `TransportError`.
9. невалидный контракт (плохой cron в poll.schedule) → `DescriptorValidationError`.
10. `boot` с указанием `versionId` шлёт контракт с этой версией.
11. повторный boot (другая версия) → `Registry.drop()`, сброс scheduler + полная перерегистрация.
12. dev-mode push: `boot(environment: development)` подписывается на WS-канал DevTransport и на новую декларацию пересобирает дерево.

---

## 17. `react-hooks.test.tsx`

Используется `@testing-library/react` + zustand store с актуальным состоянием. Render через `RuntimeProvider`.

1. `useContent(id)` → значение из стора; рендерит.
2. `useContent(id)` при отсутствии → undefined (компонент не падает).
3. `useQuery` при успехе → отображает результат; при ошибке — статус error.
4. `useQuery` с изменением input → повторный запрос (mock fetch засчитывает 2 вызова).
5. `useQuery` с `opts.schedule` → short polling: тики по cron обновляют результат без ручного re-рендера.
6. `useMutation` → `mutate()` вызывает запрос, статус переключается idle→pending→success.
7. `useDesignToken(name)` → css-переменная из стора.
8. `useRoute()` → текущий роут из стора; null до навигации.
9. `useTree()` → текущая декларация.
10. `useReady()` → true после boot.
11. `useCurrentLocale()` → текущий язык из стора; `useT()` читает строки из контента (`strings`) под текущим locale.
12. `useAsset({assetId:'asset-1', variant:'thumb'})` → URL варианта из `store.assets`; неизвестный assetId → undefined.
13. `useForm(formId)` → `{values, errors, register, handleSubmit, status}`; `handleSubmit` валидирует клиентски, submit-запрос при невалидной форме не выполняется.
14. подписка на content (opts.subscribe / poll) → событие в сторе обновляет компонент без remount.

---

## 18. `render.test.tsx` (PageRenderer / RouteOutlet)

1. `PageRenderer` строит дерево по декларации: root + children в правильном порядке.
2. каждый `instanceId` получает props из `props` + резолвенные bindings.
3. неизвестный `definitionId` → рендер placeholder/ошибку (не падение дерева), см. Decision Log.
4. `RouteOutlet` рендерит pageId по текущему роуту.
5. redirect-роут при навигации выполняет переход (страница не рендерится).
6. кедширование: одинаковые декларации не вызывают remount (сравнение по `instanceId`).

---

## 19. Интеграционные (один слой сквозной)

`integration.test.ts` — сквозной сценарий с fake-сервером инфраструктуры (Route → Page → Content через binding → fetch; ассеты → asset.get → resolveDeep; форма → form.submit).

1. boot загружает контракт, дерево рендерится, content из fake-сервера приходит через binding; поле `image: {assetId, variant}` отображается как URL (resolveDeep).
2. «поменяли контент в fake-сервере» → поллинг (cron) обновил content в сторе; **дерево НЕ пересобиралось** (счётчик `onRebuild` = 0, DOM list не изменился по key).
3. «поменяли структуру (новая декларация)» → `rebuild`; DOM обновился.
4. server-only endpoint из компонента → ошибка scope (см. §9.7).
5. форма: `form.get` → схема; пользователь заполняет → клиентская валидация → `form.submit` пишет raw JSON в fake-сервер.
6. переключение локали (`setLocale`) → контент и UI-строки перечитаны под новую локаль (серверный фолбэк); дерево НЕ пересобирается.
7. content без перевода в локали → показывается base (серверный фолбэк).
8. `serveAsset`-маршрут: клиент по такому пути делает полный переход; fake-edge отдаёт байты файла с content-type (как в §12a).
9. query + mutation + endpoint + builtin + поллинг + ассеты в одном флоу — все работают согласованно.

---

## Примечания от пользователя

- **Поллинг = default**: short polling — обычный HTTP, расписание — cron-выражение (спека §7/§10a/§10 подтверждают).
- Push (SSE/WS) — только там, где заявлен `push: true` (live-режим конструктора); поллинг/ws/sse **никогда** не влияют на `store.tree`.
- **Контент и локализация — одно**: компонент — шаблон, всё текстовое — контент с вариативностью по языкам (base + overlay, серверный фолбэк); UI-строки — контент коллекции `strings` (§8/§10b).
- **Ассеты**: in-content `{ assetId, variant }`, метаданные из `asset.get`, резолв в URL; выбор варианта — явно в контенте (§10c).
- **Единый роутинг**: одна таблица RouteDescriptor для клиента (React) и сервера (edge); actions renderPage/serveAsset/redirect(301-308+keepQuery) (§12/§12a).
- **Формы**: схема/валидация из `form.get`; submit — raw JSON в отдельную backend-таблицу; серверного состояния для поллинга нет (§7d).
- Backend отдаёт **только JSON-контракты**, никакой кодогенерации (валидация дескрипторов — §2).
- Пакет **ui-runtime** — самостоятельный npm-пакет; привязка и построение спеков под его API — как описано выше.

---

## Порядок внедрения (рекомендуемый)

1. `descriptor` + `errors` (фундамент).
2. `registry` + `transport-http`.
3. `api-client` + `builtin` (вкл. content?locale, assets).
4. `poll-scheduler` (cron) + `sync` (поллинг/push).
5. `i18n` + `content` (locale-merge, `strings`) + `assets` (resolveDeep) + `form`.
6. `store` + `tree`.
7. `routing` + `edge` + `tokens`.
8. `boot` + `react-*` + `render`.
9. `integration`.

Каждый этап зелёный независимо (можно коммитить по шагам).