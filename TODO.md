# TODO — Frontend Constructer: поэтапный план

Порядок идёт от документации к вертикальному срезу. Каждый этап заканчивается
работающим статусом и тестами. Дизайн-лог: `docs/design/frontend.md` (O1 — открыто).

> **Смена модели (редизайн админки)**: редактор дерева (Этап 6, M1-слайсы и page-store/
> tree-utils) и связанная структура данных **заменены** редизайном `docs/redesign/`
> (новая модель: роуты/Page-лист/Element, IDE-компоненты на tiptap, постраничная
> раздача). Пункты ниже выполнения 5–7 по старой модели считаются перенесёнными на
> R-слайсы и не выполняются по-старому; выполненная часть остаётся историей.

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
- [x] `/runtime/*`-дескрипторы: `/runtime/routes` — RouteDescriptor-форма boot-контракта (`toRouteDescriptors`); `/runtime/tree` — снапшот-пин item-дерево страницы (`pageId`/`routeId`/boot-эвристика, `environment`+`versionId` как у contract, страницы вне снапшота → 404); `/runtime/tokens` — дизайн-токенсет сайта как одна runtime-тема (`themeId` по умолчанию `default`). Хендлеры — `ContractHandler` в `internal/api/client/handler.go`; интеграционные тесты `runtime_descriptors_test.go`.
- [x] Снапшоты на git: `snapshot.Service` получил `GitCommitter` (`WithGit`) — при `Create` состояние сайта сериализуется в git-коммит (ветка dev) и `GitSHA` пишется в запись снапшота; проводка в `application.New` (`gitSnaps`); БД остаётся источником деревьев для build, restore/rollback из git — уже в `gitsnapshot` (R5). Unit: `TestSnapshotServiceCreateCommitsToGit`/`…GitCommitFails`; integration: `TestSnapshotServiceCreateRecordsGitCommit`.

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
- [x] Фаза 2 (слайс 1) — **subpath-импорты из site-кода**: сканирование definition-sources на bare-subpath спецификаторы объявленных вершинных deps (regex `from/import` без `x.from()`); отдельные бандлы `dist/_deps/<name>@<ver>/<subpath>.js` (scoped `%2F`, артефакт `dist/_deps/<base>/<subpath>.js`, fail-fast на нерезолвящемся и non-JS ассете); `manifest.deps` (ключ=спецификатор) + `manifest.externals` += subpath; unit + e2e.
- [x] Фаза 2 (слайс 2) — **subpath-импорты внутри вершинных deps**: общий сканер `build.ScanImportSpecifiers`/`SplitBareSpecifier` (layout+materializer); `scanDepSubpaths` по распакованным `node_modules/<dep>`; per-dep externals в dep-бандле; объединённый список subpath-бандлов; shared-корни (`react` и др.) остаются external, транзитивные/self — инлайн, subpath объявленного dep → отдельный артефакт + external; нерезолвящийся/non-JS subpath dep-кода → `DepBuildError` с hint; e2e: агент импортирует `helper/lib/math`, helper объявлен сайтом.
- [x] Фаза 2 (слайс 3) — **вложенные версионные layout**: `LockedDep{Hoisted, RequestedBy []string}` (аддитивно, omitempty); `ResolveLock` → экземплярный граф (BFS по граням `{name,spec,requestedBy}`, reuse-first-satisfying по порядку создания, Hoisted = первый экземпляр имени, Deps в BFS-порядке, top-сортировка, `ErrVersionConflict` больше не эмитится); раскладка: hoisted → корневой `node_modules/<name>`, несовместимая версия → вложена под каждым требующим родителем `node_modules/<parent>/node_modules/<name>` (walk-up esbuild резолвит каждому свою версию), legacy flat-lock без маркеров → весь root, guard'ы (unknown parent / site-requested → `DepBuildError`), единственный вложенный экземпляр не «уезжает» в root (legacy-детект лок-уровневый); unit (резолвер: nested/recursive; layout: место+бандл каждой версии, guards) + e2e (два вершинных deps → b@1 hoisted / b@2 nested, маркеры версий в бандлах, legacy flat-lock).
- [x] Фаза 2 (слайс 4) — **CSS/не-JS ассеты пакетов**: combined css соседним артефактом (`bundle()` + esbuild css-loader, `.css`-сосед `_deps/<name>@<ver>.css`, импорт вырезан из JS, `DepRef.CSSArtifact`); bare css-subpath из site-кода и из кода вершинного dep — `bundleCSS()` (css-бандл `_deps/<name>@<ver>/<subpath>.css` + JS-stub `…css.js` `export default undefined;`, import-map → stub, спецификатор в externals); `manifest.styles` (dedup+сортировка) как `<link>`-источник runtime-shell; `.css` убран из unsupported-ext, non-CSS ассеты (`png/woff/…`) → fail с hint, «No loader is configured» детектится в `depBuildError`; инверсия старых css-reject-тестов на png + unit-тесты трёх путей + e2e `TestDependencyBuildCSSArtifacts` (артефакты+stubs+styles+externals+нет css в JS-бандлах).
- [x] Фаза 2 (слайс 5) — **peer-политика (peer-fail)**: registry парсит `peerDependencies`/`peerDependenciesMeta` (packument abbr), `ResolvedVersion`/`DepPackage` несут их; после заморозки графа post-pass `checkPeers` по BFS-порядку верифицирует обязательные peers (optional из `PeerDependenciesMeta` и self-ссылки exempt) — удовлетворитель: экземпляр графа с версией в диапазоне peer ИЛИ фиксированный shared external (react 18.3.1/react-dom/react/jsx-runtime/@liapoldus/ui-runtime 0.1.0, `build/shared.Versions()`), иначе `ErrUnsatisfiedPeer` (422, подробное сообщение: потребитель@version, peer, range, доступные версии); `LockedDep` += `PeerDependencies`/`PeerDependenciesMeta` (omitempty, SBOM-прозрачность); unit: парсинг packument-фикстуры + 6 кейсов политики (граф/shared/version-mismatch/optional/self); integration `TestDependencyPeerPolicy` (shared-удовлетворён → снапшот, mismatch → 422-фейл).
- [x] Фаза 2 (слайс 6) — **allowlist scopes**: per-site allowlist в БД (`site_dependency_allowlist(site_id, entry)`, миграция 006) + admin API `GET/POST /api/sites/{id}/dependencies/allowlist`, `DELETE …/allowlist/{entry}` (scoped `%2F`); записи без версий: имя/`@scope/pkg`/`@scope/*`/`*`; пустой allowlist = allow-all (обратная совместимость); политика покрывает весь граф — проверка каждого edge в `ResolveLock` (`ErrDepNotAllowed`, 422) + ранняя отбраковка на POST `Add` после успешного probe (транзиентный probe скипается); нормализация TrimSpace+ToLower, невалидная → 400, дубль → 409, удаление отсутствующей → 404; unit: матчер/валидация, policy (vertex/transitive/star/empty/scope), admin CRUD + 422 на create; integration `TestDependencyAllowlist`.
- [x] Фаза 2 (слайс 7) — **кэш-лимиты/эвикция**: общий tarball-кэш + per-site настройка лимита в БД (`site_cache_config(site_id, max_deps_bytes)`) и last-access метки (`dep_tarball_access`, миграция 007); effective-лимит = max по сайтам, нет записей = безлимит; LRU-эвикция по last-access — удаляется только `.tgz` (`store.Delete`), метаданные (`dep_packages`/access) сохраняются; access-touch на чтении (`Layout.blob`); `deps.Service.EvictTarballs` + `StartCacheEviction` (периодический sweep, `LIAPOLDUS_DEPS_CACHE_EVICT_INTERVAL` дефолт 5 мин, автозапуск в `cmd/server`); admin API `GET/PUT /api/sites/{id}/cache-config` (вал `0 < max ≤ 8 GiB`, 400); unit: валидация/round-trip/effective-max/LRU/no-op/admin CRUD; integration `TestDependencyCacheEviction`.
- [x] Фаза 2 (слайс 7b) — **ручная эвикция**: `deps.Service.ManualEvictTarballs` (LRU до целевого размера или effective-лимита, только `.tgz`-блобы, метаданные БД сохраняются) + admin API `POST /api/sites/{id}/cache-config/evict` (body `targetDepsBytes` опционален, 0/пропущен → effective-лимит) → `{evicted, evictedBytes}`; unit: to-target/fallback-to-limit/no-limit-noop/admin; integration — расширен `TestDependencyCacheEviction`.
- [x] Фаза 3 (слайс 8a) — **`cmd/dependency-build generate/verify` (react-семья)**: автономный `internal/infra/build/sharedbuild` (BFS-резолвер react@18.3.1/react-dom@18.3.1/scheduler@0.23.2 из registry + fetch/unpack тарболлов в node_modules + esbuild-как-Go-библиотека); react бандл самодостаточен, react-dom/jsx-runtime держат react external; e2e против реального registry.npmjs.org — сгенерённые бандлы побайтово идентичны закоммиченным; `Verify` сверяет со `shared.go` Artifacts (артефакты вне области — пропускаются, ui-runtime → 8b); `writeBundles` пишет в embed без вложенного `embed/embed`; unit: generate-семья/детерминизм/materialize/verify-comparator; registry `fetchPackument` лимит 8→64 МБ (react-dom ~9.3 МБ).
- [x] Фаза 3 (слайс 8b) — **`@liapoldus/ui-runtime` через `builder.Options` (Go esbuild)**: `bundleUIRuntime` компилирует `ui-runtime/src/index.ts` (esbuild-как-Go-библиотека, jsx automatic, react/react-dom external — react/jsx-runtime экстернализуется автоматически, единый React из import-map); `Generator.WithUIRuntime(src)` + автообнаружение `ui-runtime/src` в `cmd/dependency-build` (walk-up от cwd); `Generate` теперь покрывает **всю таблицу Artifacts**; `Verify` — full-surface-гейт: артефакт вне скоупа генератора (напр. ui-runtime без src) → ошибка вместо skip. e2e против реального registry: react-семья побайтово идентична, ui-runtime пере-сгенерирован под текущий `ui-runtime/src` и закоммичен; `verify` ok. unit: full-surface generate (4 ключа, react external/self-contained, jsx-runtime shared), детерминизм с ui-runtime, verify-coverage-гейт.
- [ ] Фаза 3 (слайс 8c) — ликвидация shell-обёрток CI (`scripts/*.sh`).
- [ ] Фаза 3 (слайс 8d) — SBOM/audit-экспорт.

## Этап 5 — Boot и рендеринг на живой сборке

- [x] index.html shell + boot/mount(siteId, environment) против реального Build-контракта. *(shell в `internal/infra/build/shell` — import-map + dep `<link>` + `#root` + `entry.js`, пишется в Build и dev-rebuilder; `mount()` — boot + RuntimeProvider + PageRenderer, baseUrl = origin; entry регистрирует определения через `ComponentRegistry.registerDefinition`.)*
- [x] ComponentRegistry из бандла сайта; binding (props ← контент/route/query/операция/form). *(резолв полный в tree.ts; builtin-компоненты Container/Text/Image/Button регистрируются при загрузке модуля.)*
- [x] Пересборка структуры vs синк данных (poll/WS) на живой сборке; кэш-политики. *(mount подписан на route/content/operationResults/forms → `TreeController.refresh()` без полной пересборки; кэш-политики `/build/`: development → `no-cache`, production/`_shared` → `public, max-age=31536000, immutable`; E2E живой сборки в `tests/integration/live_build_test.go` — publish dev+prod → оболочка/entry/shared/manifest отдаются с корректными заголовками.)*
- [x] Постраничная раздача (loadable): build-time per-page чанки — materializer генерит per-page чанк `src/pages/<pageId>.tsx` (статические импорты определений страницы + `ComponentRegistry.registerDefinition` + `registerPage(pageId, tree)`), esbuild (Splitting:true, outbase=src) выдаёт `dist/pages/<pageId>.js` только с definition-ами этой страницы, корневой `entry.js` = shell+boot+роуты+import-map (только `mount`); `manifest.json` несёт `pages[]` (`pageId/chunk/definitions`) + `homePage` (эвристика «самый специфичный renderPage-роут» = та же, что у initialTree), чанк стартовой страницы тэгается `<link rel=modulepreload>` в index.html (первый экран без round-trip); `ui-runtime`: статический реестр `registerPage`/`getPageTree` + `PageLoader` (manifest → dynamic-import чанка с кэшем) в `mount()`, при навигации Router/PageRenderer догружает чанк `pageId` по требованию (placeholder, пока нет первого дерева; старый контент не моргает).
  <br>16.05b *(сверено: home-tree по-прежнему отдаётся в контракте как boot-подсказка для мгновенного первого пейнта; чанки — источник def-ов и деревьев навигации)*

## Этап 6 — Редактор `admin/` (React)

> Спека и пошаговый план: `docs/editor-spec.md` (M0–M4). M0 (монорепо, ui-kit, каркас admin+runtime) — выполнен.
> M1-слайс «Список+CRUD» (Sites/Pages/Routes списки, создание/удаление, placeholder редактора) — выполнен: 33 admin-теста, корень 285/285, typecheck+build зелёные.

- [x] `docs/editor-spec.md`: архитектура, решения E1–E11, монорепо `ui-runtime`+`ui-kit`+`admin`, admin-runtime (типы/операции/API), тест-спека.
- [x] ui-runtime: публичные экспорты (HttpTransport/типы) + обобщённый slice-store `createSliceStore`/`useSelector` (zustand-обёртка; админка zustand напрямую не импортирует).
- [x] `ui-kit` пакет: layout-примитивы на Tailwind v4 (Box/Stack/Inline/Columns/Grid/Spacer/Divider/Frame/Sidebar/SplitPane) + тесты.
- [x] `admin` SPA-каркас: Vite+React+TS+Tailwind v4+React Router, proxy `/api`→:8080 и `/runtime` `/build` `/dev`→:18080, AppShell+admin-context+роуты-заглушки.
- [x] admin-runtime: AdminApi (fetch+токен+ошибки), реестр операций, token-store (slice-store), i18n строки + тесты на mock-fetch.
- [x] M1 (слайс 1/3): списки+CRUD страниц/роутов: ops listPages/deletePage, use-operation, ConfirmButton/Field/EntityTable, Sites/SiteHome/SitePages/SiteRoutes страницы, роуты, EditorPlaceholder.
- [x] M1 (слайс 2/3): редактор дерева (tree render/select/insert/delete/undo-redo на slice-store page-store), инспектор schema-форм (SchemaForm: литералы+enum+числа, live-валидация R4), bindings (literal/content/route/query/operation/form), draft+автосейв (debounce 1500 → saveTree + индикатор «Сохранено, vN»), canvas-превью (design-mode: Container/Text/Image/Button, bindings как {{source:path}}), тесты 33→66.
- [x] M1 (слайс 3/3): canvas-превью собранной страницы — таб «Дизайн | Превью», еслиrame на `/build/{site}/{development}/{snapshot}/dist/index.html`, авто-сборка dev build после автосейва (createSnapshot «Dev preview» + createBuild development), ротация превью-снапшота, preview-store (coalescing/quedued) на slice-store + персист последнего снапшота в localStorage, WS `/dev/build/ws?siteId=` (DevRebuildEvent → refresh), ops createBuild/getBuild/deleteSnapshot, тесты 66→85.
- [x] M2 (слайс 1): контент — ops createContent/deleteContent/deleteTranslation; `JsonFieldsEditor` («ключ/тип/значение»: string/number/bool/json, инлайн-валидация), SiteContentsPage (список + фильтр-чипы коллекций + создание `{collectionId,id?,fields}` + удаление), ContentEditorPage (base=defaultLocale + переводы overlay: табы локлей, «Добавить локаль», PUT updateContent/putTranslation, DELETE deleteTranslation), роуты и навигация, тесты 85→103.
- [x] M2 (слайсы 2–5): ассеты (upload/list/preview/picker, AssetThumb, SiteAssetsPage), формы (form-utils: slugifyFormId/validateDefinition, SiteFormsPage/FormEditorPage, сабмиты), роуты (route-utils: isValidRegex/validateRouteFields/overlapWarnings, SiteRoutesPage/RouteEditorPage), rich-text Tiptap (html.ts/commands.ts/RichTextEditor, lazy-chunk), тесты 103→183.
- [x] M3 (слайс 1): публикация — страница `/sites/:siteId/builds`: снапшоты (создание/список/«Опубликовать на prod»/rollback с reveal-подтверждением PROD/удаление), сборки (env-badge, статусы queued/building/ready/failed, пересборка failed, лог, превью артефакта), админский WS `GET /api/builds/ws?siteId=&token=` (`build-ws.ts`), backend hub-события + List; тесты 183→196.
- [x] Слайс 6 (doc+cleanup): WS-клиенты dev/build объединены в `ws-client.ts` (общий `connectWs<T>` + фабрики для тестов); константы в `runtime/constants.ts` (`AUTOSAVE_DEBOUNCE_MS=1500`, ENV_DEV, пути WS); удалён мёртвый код (`ui/label.tsx`); builtin-каталог компонентов перемещён на бэкенд — `GET /api/sites/{id}/components` (union builtin + определённые на сайте, `application/component/builtin.go`), `schemas.ts` грузит его через `useComponentCatalog` (fallback на статику); config.go — задокументированы дефолты вместо «no code defaults»; удалён `docs/shadcn-migration.md`; обновлены api-admin/editor-spec/feature-status/dependency-service. Админка 224/224.
- [ ] Общие wire-типы admin↔ui-runtime: `AssetMeta`/`FormDefinition`/`Route` дублированы в `admin/src/runtime/types.ts` и `ui-runtime/src/types/` (дрейф при смене backend-контракта; низкий приоритет — не форсировать переиспользование всего runtime в админке).
- [ ] Темы и токены; UI зависимостей «как package.json» (R9).
- [ ] Git-операции из UI: commit/push/tag, новая версия, снапшот.
- [x] Публикация (снапшот → Build → environment), статусы Build, rollback. *(M3-слайс 1 «Публикация»)*

## Этап 7 — Окружения и завершение

- [ ] Development/Production не мешают друг другу; перенос между ними только через снапшот.
- [ ] Site isolation: один сайт не зависит от состояния другого; общие объекты переиспользуются.
- [ ] Логирование, метрики Build, документация архитектуры обновлена.

---

### Зависимости

Этап 2 → 3 → 5. Этап 1 можно вести параллельно с Этапом 0..2.
О1 закрыт (R9). Все открытые вопросы дизайна закрыты — см. `docs/design/frontend.md`.
