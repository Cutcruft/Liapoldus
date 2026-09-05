# TODO — Frontend Constructer: поэтапный план

Порядок идёт от документации к вертикальному срезу. Каждый этап заканчивается
работающим статусом и тестами. Дизайн-лог: `docs/design/frontend.md` (O1 — открыто).

## Этап 0 — Уборка терминологии: Vue → React
- [ ] Переписать README: §6 ComponentDefinition (.vue → .tsx), §8 Component Types, §19 Forms, §20-21 Plugin, упоминания Vue/редактора.
- [ ] Переписать `docs/ui-runtime/spec.md` и `test-spec.md`: убрать `.vue`/обёртки, зафиксировать React как единственный фреймворк.
- [ ] Прогнать `rg` по репо на `Vue|\.vue|vue ` — ноль совпадений, кроме согласованных.

## Этап 1 — Пакет `ui-runtime/` (по `docs/ui-runtime/test-spec.md`)
- [ ] `ui-runtime/`: `package.json` (npm-пакет, версии отдельные), vitest, TS strict.
- [ ] Дескрипторы и registry (Provider/Operation/Endpoint; Duplicate/Unknown/DescriptorValidationError).
- [ ] HTTP-транспорт на fetch, api-client, builtin-операции (/runtime/*), poll-scheduler (cron 6-полевый).
- [ ] WS/SSE-транспорты (фейки в тестах).
- [ ] Sync (структура vs данные), i18n/content, assets, tree, routing, design-tokens, store, boot.
- [ ] React-слой: PageRenderer, RouteOutlet, useQuery/useMutation, test-spec зелёный.

## Этап 2 — Git-модель компонентов (backend, go-git)
- [ ] Git-сервис: репо на сайт (bare), привязка remote (GitHub/GitLab SSH/HTTPS), auth (ключи/токены).
- [ ] Версии: `ComponentVersion` = коммит; файлы `.tsx` + metadata + schema в коммите.
- [ ] Автопикинг новых версий: webhook + поллинг; создание ComponentVersion по коммиту.
- [ ] Dependency-сервис (R9): резолв `pkg@version`, fetch ESM с CDN/registry, `lockfile`+integrity, кэш-зеркало.
- [ ] Интеграционные тесты: init/fetch/checkout/rollback; e2e с локальным bare.

## Этап 3 — Сборщик в Go (esbuild)
- [ ] Materializer workspace: `entry.tsx` + `definitions/*.tsx` из checkout снапшота + `manifest.json`.
- [ ] `esbuild.Build` (external на shared-библиотеки), артефакты `build/<site>/<env>/<snapshot>/`.
- [ ] Статусы Build (`queued→installing→building→ready/failed`), логи, очередь в Postgres.
- [ ] Инкрементальные пересборки: `fsnotify` + `Incremental` + WS-push (dev-режим).
- [ ] Выдача статики + shared-бандлов react/react-dom/ui-runtime.
- [ ] Тесты: собирается корректный dist, манифест, повторная сборка без изменений — no-op.

## Этап 4 — Runtime-контракт (Go-endpoints)
- [ ] `/runtime/contract` (дескрипторы), `/runtime/tree`, `/runtime/routes`, `/runtime/tokens`.
- [ ] Выдача снапшота для boot (siteId + environment + версии).
- [ ] Интеграционные тесты контракта против автономного boot-скрипта.

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