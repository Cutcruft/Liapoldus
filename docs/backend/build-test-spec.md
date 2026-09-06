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

## Решённые решения (подтверждены пользователем)

- **Источник исходников**: определения берутся из **реестра** (`Source`+`CurrentSHA`); git checkout на этапе сборки не вызывается (реестр зеркалит хэд).
- **Гранулярность бандла**: один `entry.tsx` → один **app-бандл** (SPA boot); «бандлы страниц» в design — будущий code-splitting (Этап 5).
- **Shared-бандлы**: `go:embed` предсобранных react/react-dom/ui-runtime по зафиксированной версии; fetch+кэш CDN — вместе с Dependency-сервисом (R9).
- **Async-модель**: сборка **синхронная в запросе** (CreateBuild → queued → built → ready|failed в том же ходе); отдельный worker — позже.
- **Environment**: строка из `{"development","production"}` в Build; сущность Environment — Этап 7.