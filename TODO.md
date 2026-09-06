# TODO — Frontend Constructer: поэтапный план

Порядок идёт от документации к вертикальному срезу. Каждый этап заканчивается
работающим статусом и тестами. Дизайн-лог: `docs/design/frontend.md` (O1 — открыто).

## Этап 0 — Уборка терминологии: Vue → React

- [x] Переписать README: §6 ComponentDefinition (.vue → .tsx), §8 Component Types, §19 Forms, §20-21 Plugin, упоминания Vue/редактора.
- [x] Переписать `docs/ui-runtime/spec.md` и `test-spec.md`: убрать `.vue`/обёртки, зафиксировать React как единственный фреймворк.
- [x] Прогнать `rg` по репо на `Vue|\.vue|vue ` — ноль совпадений, кроме согласованных.

## Этап 1 — Пакет `ui-runtime/` (по `docs/ui-runtime/test-spec.md`)

- [x] `ui-runtime/`: `package.json` (npm-пакет, версии отдельные), vitest, TS strict.
- [x] Дескрипторы и registry (Provider/Operation/Endpoint; Duplicate/Unknown/DescriptorValidationError).
- [x] HTTP-транспорт на fetch, api-client, builtin-операции (/runtime/*), poll-scheduler (cron 6-полевый).
- [x] WS/SSE-транспорты (фейки в тестах).
- [x] Sync (структура vs данные), i18n/content, assets, tree, routing, design-tokens, store, boot.
- [x] React-слой: PageRenderer, RouteOutlet, useQuery/useMutation, test-spec зелёный.

> Итог Этапа 1: 23 тест-файла, 220 тестов зелёные, `tsc` чистый (коммиты до `9f70170`).

## Этап 2 — Git-модель компонентов (backend, go-git)

> Тест-контракт: `docs/backend/components-test-spec.md`.
> Срез согласован: component-модель (ComponentDefinition + schema + bindings) + базовый git-сервис (внутренний bare).
> Внешние remote/автопикинг и Dependency-сервис (R9) — отдельные заходы.

- [x] Компонент-модель: ComponentDefinition (id, schema JSON Schema, metadata), ComponentNode → `definitionId` + `bindings`, валидация дерева против реестра.
- [x] Git-сервис: репо на сайт (bare, go-git), init репо, `ComponentVersion` = коммит (`src/definition.tsx` + `schema.json` + `metadata.json`).
- [x] fetch/checkout по версии, rollback (checkout коммита), история версий.

> Итог Этапа 2: unit §1–§3 (registry/validation/assembly), §4 git-service (мок), §5 git-repo (реальный go-git, bare), admin API компонентов, e2e зелёный.
> go-git в `internal/infra/git`, порт `internal/application/git` (коммиты `5f496d4`…`a37b156`).

## Этап 3 — Сборщик в Go (esbuild)

> Тест-контракт: `docs/backend/build-test-spec.md`.
> Срез Этапа 3 готов: `queued→building→ready/failed` синхронно в запросе, `entry.tsx` + `definitions/*.tsx` + `manifest.json`, esbuild как Go-библиотека, статика на `/build/...` (`internal/infra/build/*`, порты в `internal/application/build`).

- [x] Build-модель (Build, BuildStatus, environment {development, production}) + BuildRepository (Postgres `004_builds.sql` + memory).
- [x] Build-сервис с синхронным пайплайном materialize → bundle → publish; no-op повторной публикации; failed → новый id; ошибки обёрнуты в `ErrBuildFailed`.
- [x] Materializer workspace: `src/entry.tsx` (статические импорты + `ComponentRegistry.register` + `boot`) + `src/definitions/*.tsx` (реестр по `Source`+`CurrentSHA`) + `manifest.json`; детерминированный вывод, чистка каталога на ошибке.
- [x] `esbuild.Build` (external на shared-библиотеки), артефакты `build/<site>/<env>/<snapshot>/` (write-and-rename, no-op, traversal-safe).
- [x] `LIAPOLDUS_BUILD_DIR` (default `./build`), admin API `POST /api/sites/{id}/builds`, `GET /api/builds/{id}`, статика клиентского сервера `/build/...`.
- [x] Тесты: unit §1/§2/§4/§5 (моки, t.TempDir + memory, реальный материализатор/артефакт-стор, Postgres-очередь), integration §3 (real esbuild + `Incremental` smoke), e2e §6 (10/10).
- [x] Dev-пересборщик §7: `internal/infra/build/rebuilder` (fsnotify по workspace → esbuild `Context` `Incremental` → publish + бродкаст `DevRebuildEvent` в `Hub`), WS-канал `GET /dev/build/ws` на client-сервере (стрим + relay `Current` + фильтр `?siteId=`); failed не заменяет последний удачный артефакт. Тесты unit+integration (real fsnotify/esbuild/WS). Привязка к boot-рантайму — Этап 5.
- [x] Shared-бандлы §8: `scripts/build-shared` (npm) → реальные ESM react/react-dom/jsx-runtime + ui-runtime в `internal/infra/build/shared/embed` (go:embed), `Install(BuildDir)` → `build/_shared/{key}/{version}.js`, манифест получает import-map `shared`, `Externals` += `react/jsx-runtime`. Найдено: `ComponentRegistry` в entry-шаблоне отсутствует в ui-runtime → API добавится с runtime-контрактом (Этап 4). Fetch+lock — R9.

## Этап 4 — Runtime-контракт (Go-endpoints)

- [x] **Boot-контракт (часть 1)**: `GET /runtime/contract` переписан на `internal/application/runtime.Service` — выдача снапшота для boot (siteId + environment + версии: `versionId` → точный снапшот; без версии → последний `ready`-билд окружения). JSON совпадает с `parseDescriptors`/`extractTree` ui-runtime; wire-дерево с всегда-присутствующими `props/bindings/children` (иначе `resolveInstance` падает); `providers/operations/endpoints/themes` пустые (builtin регистрируется автоматически; токены — часть 2).
- [x] **Автономный boot-скрипт**: фикстура `tests/integration/fixtures/ui-runtime-core.mjs` (esbuild-bандл `ui-runtime/src/core/boot.ts`, без react) + `fixtures/boot.mjs`; integration-тест `httptest` + реальный `node boot()` против контракта.
- [ ] `/runtime/*`-дескрипторы: `/runtime/tree`, `/runtime/routes`, `/runtime/tokens`.
- [ ] Переделать снапшоты на git

## Этап 4b — Dependency-сервис (zero-node npm-зависимости, spec: `docs/backend/dependency-service.md`)

- [x] Спека: дизайн confirmed решений (zero-node/zero-sh Go, npm-registry напрямую, резолв на publish снапшота, диапазон→lock exact+sha512, fail на несовместимых); план фаз §11, ограничения §12.
- [x] Фаза 1 — **domain+storage**: `domain.Dependency`/`LockedDep`/`SnapshotLock`/`DepPackage` + доменные ошибки; `snapshots.deps_lock` JSONB (миграция `005_dependencies.sql`); таблицы `site_dependencies`/`dep_packages` (диск-стор блобов — `internal/infra/deps/store`, БД не индексирует); постгрес+memory-репозитории. 🔑 (resolvedVersion — пассивный лучший-effort, не источник правды).
- [x] Фаза 1 — **registry-фетчер**: `internal/infra/deps/registry` (packument abbr `install-v2+json`, scoped `%2F`, max-semver по диапазону, skip yanked, integrity+tarball, кэш 5m, retries/backoff, 404→`ErrPackageNotFound`/unresolvable→`ErrUnresolvableSpec`) + `Masterminds/semver/v3`.
- [x] Фаза 1 — **`deps.Service`**: `Add` (валидация имени/спек+probe best-effort, upsert), `Remove`, `List`, `ResolveLock` (flat-graph BFS, reuse совместимой версии, `ErrVersionConflict`, сортировка, integrity-cache в `dep_packages`).
- [x] Фаза 1 — **lock в снапшот**: `snapshot.Service` variadic `LockResolver` → `snapshot.DepsLock` заполняется при `Create` (fail издания если граф не резолвится); `LIAPOLDUS_NPM_REGISTRY` (default public registry).
- [x] Фаза 1 — **admin API**: `GET/POST /api/sites/{id}/dependencies`, `DELETE .../{name}` (scoped `%2F`); маппинг ошибок (400/404/422).
- [x] Фаза 1 — **тесты**: unit (`deps_service`, `deps_registry` с httptest-фейком, admin deps+lock, снимок-lock 422) и integration (флоу memory+postgres с локальным registry-сервером).
- [x] Фаза 1 — **материализация+бандлинг**: диск-стор tarball-блобов (`internal/infra/deps/store`, `<data>/deps/<name>/<version>.tgz`; таблица `dep_blobs` убрана); `node_modules`-layout; per-dep esbuild `dist/_deps/<name>@<version>.js` (scoped `%2F`) с транзитивами-инлайном и `SharedExternals`-external; `manifest.deps`+`manifest.externals`; `DepBuildError` (браузерные builtins); e2e (httptest-registry + реальный esbuild). Починен `artifactstore.copyTree` (вложенные `dist/_deps/` не копировались).
- [x] Фаза 1 — финальные поверки (gofmt/vet/test, spec-обновление, коммит).
- [ ] **Фаза 2**: полное subpath-покрытие, вложенные версионные layout, CSS/ассеты пакетов, peer-fail, allowlist scopes.
- [ ] **Фаза 3**: `cmd/dependency-build` вместо `scripts/build-shared` (npm уходит полностью), ликвидация shell-обёрток, SBOM.

## Этап 5 — Boot и рендеринг на живой сборке

- [ ] index.html shell + boot(siteId, environment) против реального Build-контракта.
- [ ] ComponentRegistry из бандла сайта; binding (props ← контент/route/query/операция).
- [ ] Пересборка структуры vs синк данных (poll/WS) на живой сборке; кэш-политики.

## Этап 6 — Редактор `admin/` (React)

> Спека и пошаговый план: `docs/editor-spec.md` (M0–M4). M0 (монорепо, ui-kit, каркас admin+runtime) — выполнен.
> M1-слайс «Список+CRUD» (Sites/Pages/Routes списки, создание/удаление, placeholder редактора) — выполнен: 33 admin-теста, корень 285/285, typecheck+build зелёные.

- [x] `docs/editor-spec.md`: архитектура, решения E1–E11, монорепо `ui-runtime`+`ui-kit`+`admin`, admin-runtime (типы/операции/API), тест-спека.
- [x] ui-runtime: публичные экспорты (HttpTransport/типы) + обобщённый slice-store `createSliceStore`/`useSelector` (zustand-обёртка; админка zustand напрямую не импортирует).
- [x] `ui-kit` пакет: layout-примитивы на Tailwind v4 (Box/Stack/Inline/Columns/Grid/Spacer/Divider/Frame/Sidebar/SplitPane) + тесты.
- [x] `admin` SPA-каркас: Vite+React+TS+Tailwind v4+React Router, proxy `/api`→:8080 и `/runtime` `/build` `/dev`→:18080, AppShell+admin-context+роуты-заглушки.
- [x] admin-runtime: AdminApi (fetch+токен+ошибки), реестр операций, token-store (slice-store), i18n строки + тесты на mock-fetch.
- [x] M1 (слайс 1/3): списки+CRUD страниц/роутов: ops listPages/deletePage, use-operation, ConfirmButton/Field/EntityTable, Sites/SiteHome/SitePages/SiteRoutes страницы, роуты, EditorPlaceholder.
- [ ] M1 (слайс 2/3): редактор дерева (tree render/select/insert/delete), инспектор schema-форм, bindings, draft+автосейв, canvas-превью.
- [ ] Темы и токены; UI зависимостей «как package.json» (R9).
- [ ] Git-операции из UI: commit/push/tag, новая версия, снапшот.
- [ ] Публикация (снапшот → Build → environment), статусы Build, rollback.

## Этап 7 — Окружения и завершение

- [ ] Development/Production не мешают друг другу; перенос между ними только через снапшот.
- [ ] Site isolation: один сайт не зависит от состояния другого; общие объекты переиспользуются.
- [ ] Логирование, метрики Build, документация архитектуры обновлена.

---

### Зависимости

Этап 2 → 3 → 5. Этап 1 можно вести параллельно с Этапом 0..2.
О1 закрыт (R9). Все открытые вопросы дизайна закрыты — см. `docs/design/frontend.md`.
