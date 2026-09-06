# Статус функций

`README.md` описывает целевую архитектуру Liapoldus. Этот файл показывает, что уже реализовано в текущем первом срезе.

## Реализовано

| Область | Статус | Что проверяется |
| --- | --- | --- |
| Go backend | Готово | Запуск двух HTTP-серверов (admin, client) из переменных окружения |
| Admin REST API | Готово | API из `docs/api-admin.md`: sites/pages/content/assets/routes/forms/snapshots |
| Client REST API | Готово | API из `docs/api-client.md`: content (merge по locale), assets, forms, `/runtime/*` |
| Edge routing | Готово | serve asset, redirect (включая группы `$1..$9` и keepQuery), render page → 404 |
| Site | Реализовано | Создание, чтение, список, обновление, удаление, `defaultLocale`, `hosts`, резолв по Host |
| Page | Реализовано | Создание, чтение, список, update tree, версии страницы |
| Component tree | Реализовано | `Container`, `Text`, `Image`, `Button`, props и children |
| Component validation | Реализовано | ID, тип компонента, глубина дерева |
| ComponentDefinition | Реализовано | Реестр по сайту: id, schema (JSON Schema), metadata; дерево ссылается на `definitionId` и валидируется жёстко по schema (R4) |
| Component versioning (git) | Реализовано | Bare-репо на сайт (go-git, `internal/infra/git`); `Release` = коммит (`src/definition.tsx` + `schema.json` + `metadata.json`); история версий, checkout по sha, rollback |
| Page versioning | Реализовано | Новая версия при обновлении дерева |
| Snapshot | Реализовано | Фиксация актуальной версии каждой страницы, чтение, версии, удаление |
| Content | Реализовано | Поля + переводы по locale, merge на стороне клиента, batch, коллекция `strings` |
| Asset storage | Реализовано | Загрузка (multipart), метаданные с variants, выдача байтов с ETag, удаление |
| Routes | Реализовано | matcher (regex) + priority, действия serve-asset/redirect/render-page, валидация |
| Forms | Реализовано | Определение, серверная валидация (required/minLength/email), сабмиты |
| Auth (admin) | Реализовано | Bearer token через `LIAPOLDUS_ADMIN_TOKEN` (пустой = открыто) |
| Runtime health | Готово | `GET /healthz` на admin и client |
| Persistence | Реализовано | PostgreSQL adapter (миграции `001`–`004`) и in-memory adapter |
| Build (synchronous, Этот 3) | Реализовано | `POST /api/sites/{id}/builds` {snapshotId, environment} → синхронная сборка (queued→ready/failed), no-op повторной публикации; материализация (workspace: `src/entry.tsx` + `src/definitions/*.tsx` + `manifest.json`), esbuild как библиотека Go, артефакты `build/<site>/<env>/<snapshot>/`; статика отдаётся публично на `/build/...` |
| Dev-пересборщик (Этап 3 §7) | Реализовано | `internal/infra/build/rebuilder`: fsnotify → esbuild `Context` (Incremental) → publish + бродкаст `DevRebuildEvent`; `Hub` с relay `Current`; WS-канал `GET /dev/build/ws` (фильтр `?siteId=`, failed не заменяет последний удачный артефакт); привязка к boot-рантайму — Этап 5 |
| Shared-бандлы (Этап 3 §8) | Реализовано | Реальные ESM-бандлы (react/react-dom/jsx-runtime 18.3.1 + ui-runtime 0.1.0) из `scripts/build-shared`, go:embed → `/build/_shared/<key>/<version>.js`; манифест содержит import-map `shared`; `Externals` += `react/jsx-runtime`. CDN fetch+lock — R9 |
| Runtime-контракт (Этап 4) | Частично | **Boot-контракт реализован**: `GET /runtime/contract` (снапшот-пиннинг через `versionId` | последний ready-билд окружения) на `application/runtime.Service`; JSON совпадает с `parseDescriptors`/`extractTree` ui-runtime; автономный boot-скрипт (`tests/integration/fixtures/ui-runtime-core.mjs` + `boot.mjs`, реальный `boot()` в integration-тесте). Осталось: `/runtime/tree`, `/runtime/routes`, `/runtime/tokens` + API компонент-реестра (`ComponentRegistry` для entry-шаблона — сегодня только `RuntimeRegistry`) |
| Сторонние npm-пакеты | Фаза 1 реализована | `deps.Service` (CRUD+`ResolveLock` flat-graph, exact+sha512), registry-фетчер, диск-стор tarball-блобов (`<data>/deps`), материализация (`node_modules` из кэша) + esbuild per-dep бандлы `dist/_deps/<name>@<version>.js` (scoped `%2F`), `manifest.deps`/`manifest.externals`, `DepBuildError` для несовместимых, e2e (httptest-registry + реальный esbuild). Осталось (Фаза 2/3): полный subpath, вложенные версионные layout, CSS-ассеты, `cmd/dependency-build`. Спека `docs/backend/dependency-service.md` |
| Admin SPA (Этап 6, M0) | Реализовано | Монорепо (`ui-runtime`/`ui-kit`/`admin`, root workspaces). Спека `docs/editor-spec.md`. ui-runtime: публичные экспорты + обобщённый slice-store (zustand-обёртка `createSliceStore`/`useSelector`, админка zustand не импортирует). `@liapoldus/ui-kit`: layout-примитивы Tailwind v4 (Box/Stack/Inline/Columns/Grid/Spacer/Divider/Frame/Sidebar/SplitPane) + 25 тестов. `admin`: Vite+React+TS+Tailwind v4+React Router, proxy `/api`→:8080 и `/runtime|/build|/dev`(ws)→:18080, AppShell+admin-context+роуты-заглушки. admin-runtime: AdminApi (fetch, Bearer-токен, норм. ошибки 400/401/4xx/transport), реестр операций `OPERATIONS` + `runOperation`, token-store на slice-store, i18n строки; 20 тестов на mock-fetch. M1-слайс «Список+CRUD»: ops listPages/deletePage, `useOperation`, ConfirmButton (без window.confirm), Field/EntityTable, страницы Sites/SiteHome/SitePages/SiteRoutes + роуты + EditorPlaceholder; 13 тестов страниц (админка 33/33, корень 285/285) |

## Пока не реализовано

- Binding и runtime data sources (props ← контент/route/query/операция);
- Theme;
- Operation, Provider и frontend SDK;
- прямые HTTP/GraphQL-интеграции из frontend;
- server-only operations;
- ESB extension layer (registry, metadata routing, `Call`/`Stream`);
- Plugin management и декларации зависимостей (fetch + lock с CDN/registry);
- версии всех объектов кроме Page;
- полноценный Snapshot со всеми типами объектов;
- shared-бандлы (`build/_shared`) с fetch+lock (R9);
- Development/Production environments (client работает с текущими данными напрямую);
- runtime publication и rollback;
- Redis/cache policies;
- production lifecycle и динамическая загрузка extension registry.

## Спроектировано (спек, реализация не начата)

| Область | Документация |
| --- | --- |
| ui-runtime: архитектура и слои (контент+локализация, ассеты, единый роутинг/edge) | `docs/ui-runtime/spec.md` |
| ui-runtime: JSON-дескрипторы (контракт для backend, вкл. поллинг/content-locale/ассеты/роуты) | `docs/ui-runtime/json-descriptors.md` |
| ui-runtime: полный спек тестов (контракт для реализации, вкл. asset/edge-тесты) | `docs/ui-runtime/test-spec.md` |
| Backend REST API (разделение admin/client, content с `?locale` и translations, ассеты, формы, runtime-роутинг) | `docs/api.md`, `docs/api-admin.md`, `docs/api-client.md` |

Наличие пункта в основном README не означает, что он уже доступен через API. Перед реализацией каждого следующего блока его нужно добавить в этот статус и покрыть тестами.