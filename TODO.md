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
- [ ] Инкрементальные пересборки: `fsnotify` + `Incremental` + WS-push (dev-режим).
- [ ] Shared-бандлы react/react-dom/ui-runtime: `build/_shared/{name}/{version}.js` (go:embed/stub; fetch+lock — с Dependency-сервисом R9).

## Этап 4 — Runtime-контракт (Go-endpoints)

- [ ] `/runtime/contract` (дескрипторы), `/runtime/tree`, `/runtime/routes`, `/runtime/tokens`.
- [ ] Выдача снапшота для boot (siteId + environment + версии).
- [ ] Интеграционные тесты контракта против автономного boot-скрипта.
- [ ] Переделать снапшоты на git

## Этап 5 — Boot и рендеринг на живой сборке

- [ ] index.html shell + boot(siteId, environment) против реального Build-контракта.
- [ ] ComponentRegistry из бандла сайта; binding (props ← контент/route/query/операция).
- [ ] Пересборка структуры vs синк данных (poll/WS) на живой сборке; кэш-политики.

## Этап 6 — Редактор `admin/` (React)

- [ ] Дерево страниц (страницы/роуты), props/content по schema (live-валидация), bindings.
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
