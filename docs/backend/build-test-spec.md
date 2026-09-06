# Backend — Сборщик (esbuild) и Build-сервис. Тест-контракт (Этап 3)

Статус: **черновик** — уточняем до реализации.
Дизайн: `docs/design/frontend.md` (R2, R3, R6, R9), `README.md` §24–27, `docs/ui-runtime/spec.md` (boot).
Предыдущий этап: `docs/backend/components-test-spec.md` (git-модель, реализована).

## Границы этапа (что входит / не входит)

**Входит**

- Материализация workspace из снапшота: `src/entry.tsx` + `src/definitions/*.tsx` + `manifest.json`.
- Сборка esbuild как Go-библиотеки (`github.com/evanw/esbuild/pkg/api`), external на shared-библиотеки.
- Артефакты по пути `build/<site>/<environment>/<snapshot>/` + artifact-store хендлер (выдача dist).
- Build-модель и очередь: статусы `queued → building → ready | failed`, лог, привязка к (site, snapshot, environment).
- Очередь и хранение Build в Postgres (адаптер) + in-memory аналог.
- Idempotent-публикация: повторная сборка без изменений — no-op (тот же artifact_dir, тот же build).
- shared-бандлы `react`, `react-dom`, `@liapoldus/ui-runtime` как external (раздаются отдельно от dist сайта).

**Не входит (отдельные заходы/этапы)**

- `/runtime/*`-контракт и boot (Этап 4–5).
- Theme/design-tokens и содержимое `deps.json`/`lockfile.json` (R9, Dependency-сервис — отдельный заход).
- Dev-пересборщик: `fsnotify` + esbuild `Incremental` + WS-push — отдельный заход (в спеке только §3 smoke на `Incremental`).
- Environments как сущность (Этап 7): здесь `environment` — зафиксированная строка.

## Общие требования

- Go-тесты живут в `backend/tests/unit` и `backend/tests/integration` (в `internal/` — только нетривиальные интеграционные, по договорённости).
- Каждый § отдельный зелёный шаг, коммит по шагам.
- Новый Go-модуль esbuild добавляется как direct-зависимость; для тестов §3/§4 он **не мокается** (реальный esbuild.Build в памяти).
- Артефакты/материализация пишутся в `t.TempDir()`; реальный artifact-store-корень — из конфига (env `LIAPOLDUS_BUILD_DIR`, дефолт `./build`).
- Определения компонентов для сборки берутся из **реестра** (`ComponentDefinition.Source` + `CurrentSHA`), а не checkout'ом из git; git остаётся источником истины, реестр — зеркало актуального хэда (см. «Примечания»).
- Все статус-переходы выставляются через один метод службы (в памяти — атомарно; на Postgres — в транзакции).

## §1. `build_service_test.go` — Build-модель и очередь (моки builder+runner)

Моки: `WorkspaceBuilder` (материализует workspace) и `BundleRunner` (эмуляция esbuild). Оба — интерфейсы в `internal/application/build`, реализации — в infra.

- `Create(siteID, snapshotID, environment)` → `Build{ID build_*, SiteID, SnapshotID, Environment, Status: queued, ArtifactDir: build/<site>/<env>/<snapshot>/, Log: []string, CreatedAt}`; снапшот должен существовать и принадлежать сайту (`ErrNotFound` иначе на моке-реестре).
- Очередь: `Queue`/`Start` — статусы `queued → building → ready|failed`; переход на уже `ready`/`building`/`failed` из ква идентичен no-op.
- Idempotent/no-op: повторный `Create` для (site, snapshot, environment) с существующим `ready` Build возвращает **тот же** Build (тот же ID, ArtifactDir не пересоздаётся).
- Failure: эмуляция ошибки runner → статус `failed`, `Log` не пуст, `ArtifactDir` результата **отсутствует**; повторный `Create` уже не no-op (новая попытка → новый build).
- `Get(id)` / `BySite(siteID)`; `UpdateStatus` с валидацией перехода (например, `ready → queued` запрещён → `ErrInvalidRequest`).
- `Environment` ограничен множеством `{"development","production"}` (иначе `ErrInvalidRequest`).

Файлы: `internal/application/build/service.go` (+ `model/service` в domain: `Build`, `BuildStatus`) и моки в `tests/unit`.

## §2. `materializer_test.go` — материализация workspace из снапшота

`materializer.Materialize(siteID, snapshotID, environment, defs ComponentDefinitionRepository, snapshots SnapshotRepository, dir string) (Manifest, error)`

- Сбор definitionId: пройти `root` каждой версии страниц снапшота (узлы `ComponentNode`, см. page-сервис `Version`), собрать уникальные `definitionId`.
- Для каждого: прочитать def из реестра; неизвестный `definitionId` → `ErrNotFound`. Записать `src/definitions/<defId>.tsx` = `Source`. Записать `src/entry.tsx`, который статически импортирует **все** definitions и регистрирует каждую в `ComponentRegistry` до `boot()`.
- Записать `manifest.json`: `{siteId, snapshotId, environment, definitions: {defId: {file, sha: CurrentSHA}}, pages: [{pageId, route?}], externals: ["react","react-dom","@liapoldus/ui-runtime"]}`.
- Чистота: write-and-rename в `dir` (материализация не оставляет частичных файлов при ошибке — каталог удаляется).
- Детерминизм: два запуска с одним реестром → идентичные байты `entry.tsx`/`manifest.json` (сортируемые keys, фиксированный текст регистрации).
- Манифест валиден по структуре (парсится, все поля на месте; `sha` совпадает с `CurrentSHA`).
- entry.tsx инвариант: содержимое содержит `import ... from './definitions/<defId>'` и вызов регистрации для каждого defId (проверяется substring'ами на сгенерированном файле).

Файлы: `internal/infra/build/materializer` (infra), интерфейс в application; тесты в `tests/unit` (реальная запись в t.TempDir, defs на `storage.Memory`).

## §3. `esbuild_builder_test.go` — реальная сборка (качество dist)

`builder.Build(wsDir string) (ArtifactManifest, error)` — обёртка над `esbuild.Build`:

- `options`: platform node?**browser**, `format: esm`, `bundle: true`, `minify: true`, `target: es2020`, `external: ["react","react-dom","@liapoldus/ui-runtime"]`, `outdir: <ws>/dist`, `entryPoints: ["src/entry.tsx"]`, `logLevel: silent` (ошибки — через `errors` результата API).
- Valid-source тест: workspace из реального §2-материализатора с двумя простыми дефинициями (`Container`, `Text` с тривиальным ЕСМА-кодом без внешних модулей) → сборка успешна; в `dist` есть JS-файл; его содержимое **не содержит** `react`-кода как inlined (external соблюдён: в бандле остаётся `import ... from "react"`).
- Failed-source тест: синтаксическая ошибка в `definitions/x.tsx` → ошибка со `Text` (сообщение esbuild) и полным `Log`; `dist` не создан.
- Incremental smoke (в отдельном тесте, по договорённости не привязан к CI-флоу/dev): `esbuild.Build` с `Incremental: true`, `rebuild` после правки файла без пересоздания workspace.

Файлы: `internal/infra/build/builder` — infra; тесты в `tests/integration` (real esbuild).

## §4. `artifact_store_test.go` — артефакты на диске, no-op, выдача статики

`artifactstore.Store(root string)`:

- После успешной сборки: `build/<site>/<environment>/<snapshot>/` содержит `dist/` + `manifest.json` (build-манифест: файлы, размеры, sha256; версия контракта).
- Запись атомарна: пишем во временный каталог, `rename` в финальный; при совпадении существующего содержимого (no-op) — каталог не перезаписывается (`mtime` не меняется).
- Чтение файлов `/build/{site}/{environment}/{snapshot}/{path}`: `Open`/`Exists` — для выдачи статики (в §6 e2e через runtime/admin-хендлер, здесь только храниловая логика).
- Ошибки: отсутствующий снапшот путей → `ErrNotFound`; выход за пределы корня (path traversal) → `ErrInvalidRequest`.
- shared-бандлы: при первом «успешном» publish копируются в `build/_shared/{name}/{version}.js` (реакт/реакт-дом/ui-runtime, источники — см. «Примечания», тест — на файлах-стабах из t.TempDir).

Файлы: `internal/infra/build/artifactstore` — infra; тесты в `tests/unit` (t.TempDir).

## §5. `postgres_build_test.go` — очередь Build в Postgres (интеграция)

- Миграция `003_builds.sql`: таблица `builds` (id text pk, site_id, environment, snapshot_id, status, log jsonb, artifact_dir, created_at, started_at, finished_at; index (site_id, environment, snapshot_id, status)).
- Адаптер `storage.BuildStore` (реализация на `database/sql` + `memory.BuildStore` аналог): `Create`, `Get`, `BySite`, `UpdateStatus` (+лог append) — в транзакции, консистентность no-op (повторный Create существующего ready → тот же id, не новый).
- Интеграционный тест — по шаблону существующего `tests/integration/postgres_test.go`: skip при отсутствии `TEST_DATABASE_URL`, TemporaryTable, запросы без SQL-инъекций.

## §6. e2e — публикация сайта через API

vitest (`tests/e2e/build.test.ts`, шаблон `api.test.ts` с Bearer-auth): последовательность для сайта с двумя определёнными компонентами (`POST /api/sites/.../components`), страницей и снапшотом:

- `POST /api/sites/{siteID}/builds` `{snapshotId, environment: "development"}` → `201` build c `status: queued` (после завершения — `ready` по `GET /api/builds/{id}`).
- Повторная публикация того же снапшота → **тот же** build id (no-op).
- `GET /api/builds/{id}` → статус и лог (при failure — лог непустой).
- Artifact-статика: `GET /build/{site}/{env}/{snapshot}/dist/<bundle>.js` отдаёт 200 и корректный `content-type`; путь вне каталога → 404.

Порядок инъекции эндпоинтов: `internal/api/admin` — ручки `CreateBuild`, `GetBuild` (+ маршруты).

## Порядок внедрения (рекомендуемый)

1. domain: `Build`, `BuildStatus`, `Environment`; схема `003_builds.sql`; `storage.BuildStore` (memory+postgres). Тесты `storage` + §5.
2. `internal/application/build.Service` + интерфейсы `WorkspaceBuilder`/`BundleRunner` (§1 unit).
3. `internal/infra/build/materializer` (§2 unit) и прогон через §1-моки в service.
4. `internal/infra/build/builder` (real esbuild) + `internal/infra/build/artifactstore` (§3, §4).
5. admin API: `CreateBuild`/`GetBuild` (§6 e2e).
6. Обновить `docs/feature-status.md` и закрыть пункты Этапа 3 в `TODO.md`.
7. Dev-пересборщик (`fsnotify`+`Incremental`+WS) — §7 ниже.

Каждый шаг зелёный независимо.

## §7 Dev-пересборщик (fsnotify + Incremental + WS)

Реализовано (компонент + WS-канал; привязка к boot-рантайму — Этап 5).

- `internal/infra/build/rebuilder.Rebuilder`: один dev-workspace на сайт; `Start(ctx)` материализует workspace (или читает `manifest.json`), открывает **esbuild `Context`** (`Incremental`, те же `Options`, что у одноразовой сборки), смотрит `src/*.tsx` через `fsnotify` (дебаунс), на изменение → `Rebuild` → `Publish` в артефакты → бродкаст.
- `rebuilder.Hub`: fan-out `DevRebuildEvent` подписчикам (неблокирующая доставка, drop при медленном получателе) + «последнее событие по сайту» для relay при подключении.
- Контракт `build.DevRebuildEvent` (`siteId`, `environment`, `snapshotId`, `artifactDir`, `status` ready/failed, `error`, `updatedAt`) — в `internal/application/build` (порт `DevEventHub`).
- Важно: **failed-пересборка не заменяет последний удачный артефакт** — браузер продолжает показывать рабочий бандл, следующая успешная пересборка публикуется поверх.
- WS-канал: `GET /dev/build/ws` на client-сервере (`internal/api/client/dev.go`); опциональный `?siteId=` фильтр; при подключении первым сообщением отдаётся `Current(siteID)` (релей). Монтируется при непустом `App.DevHub`; в `main.go` хаб создаётся всегда.
- Решённое решение: контракт WS-канала отвязан от boot (Этап 5) — канал публикует сырые события `DevRebuildEvent`, а не «новую декларацию»; преобразование в hot-reload рантайма остаётся за Этапом 5. `InsecureSkipVerify` на Accept — dev-only (без сессии/авторизации).

Тесты: unit `hub` (broadcast/unsubscribe/current/slow-receiver), integration `rebuilder` (real fsnotify + esbuild context + артефакты: правка определения → новый бандл + событие; синтаксическая ошибка → failed), integration WS (`httptest` + `coder/websocket`: стрим, relay при подключении, фильтр по `siteId`).

## §8 Shared-бандлы (реальные ESM через npm + go:embed)

Реализовано: поставляется import-map из 4 зафиксированных **реальных** ESM-бандлов, собранных esbuild из npm-пакетов и реакторного `ui-runtime/src`.

- Сборочный скрипт `backend/scripts/build-shared/` (`npm ci && npm run build`): bundles `react`, `react-dom` (+ `createRoot`/`hydrateRoot` в один модуль), `react/jsx-runtime` и `@liapoldus/ui-runtime` в `backend/internal/infra/build/shared/embed/<key>/<version>.js`; статический smoke (parse ESM + маркеры поверхности API). Версия ui-runtime читается из его `package.json`; react/react-dom фиксированы `18.3.1` и **должны совпадать с таблицей `Artifacts`** в `internal/infra/build/shared/shared.go`.
- Артефакты коммитятся в репозиторий и вшиваются в бинарник через `go:embed`. `shared.Install(BuildDir)` раскладывает их в `build/_shared/<key>/<version>.js` (идемпотентно, без перезаписи), откуда их раздаёт существующий хендлер client-сервера `GET /build/_shared/...` (тот же `http.FileServer`, что и site-артефакты).
- Манифест сборки получает `shared` (import-map: bare-спецификатор → публичный URL); `Externals` пополнен `react/jsx-runtime`, чтобы automatic JSX-transform никогда не инлайнил React в site-бандл. Boot-шелл (Этап 5) превратит `manifest.shared` в `<script type="importmap">`.
- Fetch/cache по CDN и Dependency-сервис остаются за R9 (см. «Решённые решения»).
- Известный разрыв (Этап 4, runtime-контракт): шаблон `src/entry.tsx` (§3) импортирует `ComponentRegistry.register(id, def)`, а ui-runtime сегодня экспортирует `RuntimeRegistry.register(descriptor)` (другое API). esbuild не валидирует экстерналы, поэтому сборка проходит; API компонент-реестра будет добавлен в ui-runtime на Этапе 4 (runtime-контракт). `boot(siteId, environment)` уже совпадает сигнатурой.

Тесты: unit `shared` (все декларированные артефакты вшиты, URL/import-map, `Install` идемпотентен и не перезаписывает), unit materializer (import-map попадает в `manifest.json` только при наличии резолвера), integration `httptest` + FileServer (4 бандла отдаются 200 `text/javascript`, неизвестные версии — 404), e2e §8 (клиентский сервер отдаёт 3 shared-бандла).

## §9 Boot-контракт (Этап 4, часть 1) — выдача снапшота + автономный boot-скрипт

Реализовано: `GET /runtime/contract` переписан на `internal/application/runtime.Service` — дескриптор-контракт, который `ui-runtime` парсит „как есть" через `parseDescriptors`/`extractTree` (JSON-формы совпадают с `ContractDescriptor`/`RouteDescriptor`/`TreeDeclaration` в `ui-runtime/src/types`).

- Привязка к снапшоту (§3 спеки): `versionId` → точный снапшот (с проверкой siteID); без версии → последний `ready`-билд окружения (`build.Service.Published`) → его снапшот; `contract.version` = snapshot.ID. `environment` по умолчанию `production`, валидные только `development`/`production` (иначе 400); нет published-билда → 404.
- Дерево: boot-страница = самая приоритетная `renderPage`-страница снапшота (priority desc, затем created) через роут, иначе первая страница снапшота; нет страниц → `tree` отсутствует. `tree.versionId` = ID версии страницы.
- Wire-дерево (`TreeNode`): `props`/`bindings`/`children` **всегда** присутствуют. Это отдельное решение: `domain.ComponentNode` использует `omitempty`, а `resolveInstance()` в ui-runtime итерирует `bindings`/`children` напрямую (`for…of`) — без нормализации boot падал бы на листьях.
- Роуты мапятся в `RouteDescriptor` только с якорями `^…$` и компилируемым regex (иначе `parseDescriptors` бросил бы DescriptorValidationError и boot не стартовал). `providers/operations/endpoints` = `[]` (builtin из `registerBuiltin` регистрируется автоматически), `themes` = `[]` (токены — Этап 4, часть 2), `enabledChannels {ws,sse}`, `capabilities {formSubmissions, dev: env==development}`.
- Автономный boot-скрипт: `tests/integration/fixtures/ui-runtime-core.mjs` — self-contained ESM-бандл `ui-runtime/src/core/boot.ts` (без react: core не импортирует react) от `scripts/build-shared/build.mjs`; `fixtures/boot.mjs` — node-пробник `boot(siteId, env, {baseUrl, env:{fetch, storage}})` → JSON (ready, locale, hasContentOp, homePageId, tree root). Integration-тест гоняет `node boot.mjs` против httptest client-сервера (skip при отсутствии node).
- Решённое решение: производственный boot не открывает dev-WS (`dev: false`), поэтому пробник работает headless без `WebSocket`/`window`.

Тесты (integration `try runtime_contract_test.go`): pinned boot release (мета/роуты/дерево/capabilities), versionId override + чужой snapshot → 404, ошибки (bad env 400, unpublished 404), автономный boot-скрипт (реальный `boot()`: ready/hasContentOp/homePageId/tree root).

## Решённые решения (подтверждены пользователем)

- **Источник исходников**: определения берутся из **реестра** (`Source`+`CurrentSHA`); git checkout на этапе сборки не вызывается (реестр зеркалит хэд).
- **Гранулярность бандла**: один `entry.tsx` → один **app-бандл** (SPA boot); «бандлы страниц» в design — будущий code-splitting (Этап 5).
- **Shared-бандлы**: `go:embed` предсобранных react/react-dom/ui-runtime по зафиксированной версии; fetch+кэш CDN — вместе с Dependency-сервисом (R9).
- **Async-модель**: сборка **синхронная в запросе** (CreateBuild → queued → built → ready|failed в том же ходе); отдельный worker — позже.
- **Environment**: строка из `{"development","production"}` в Build; сущность Environment — Этап 7.