# Frontend / Runtime Design (лог решений)

Статус: **черновик** — ведём при проектировании, обновляем инкрементально.
Связанные документы: `docs/ui-runtime/spec.md`, `README.md`.
Кодовая база: скоро появится в `ui-runtime/` (пакет) и `admin/` (редактор).

---

## Решённые решения

| # | Область | Решение | Обоснование |
| - | ------- | ------- | ----------- |
| R1 | Рендер и компоненты | Только **React**. Компонент — исходник `.tsx`. **Vue исключается**, все упоминания удаляются из README и `ui-runtime/` docs. | Единый фреймворк для рантайма, редактора и сборщика. |
| R2 | Движок сборки | **esbuild как Go-библиотека** (`github.com/evanw/esbuild/pkg/api`) линкуется в единственный Go-бинарник. Без Node. | esbuild написан на Go; транспиляция TS/TSX+JSX, бандлинг, minify, tree-shaking, инкрементальные пересборки. |
| R3 | Библиотеки в браузере | **Shared self-hosted prebuilt бандлы**: `react`, `react-dom`, `@liapoldus/ui-runtime` раздаются backend'ом как статика. Бандл сайта объявляет их `external`. | Один общий снапшот библиотек, изоляция версий сайтов, лёгкая выданя статики. |
| R4 | Валидация контента | **Live в редакторе + мягкая проверка формы на backend** (запись контента не отклоняется при невалидных значениях относительно schema). | Быстрая итерация контента; согласованность по формату всё равно обеспечивается. |
| R5 | Источник истины исходников | **Git**. `ComponentVersion` = коммит. В Postgres — контент, страницы, снапшоты и ссылки на версии. Rollback = checkout коммита. | История, ветки, коллаборация — из коробки. |
| R6 | Топология git | **Репозиторий на сайт**. Наборы/плагины — отдельные репозитории, подключаются как зависимости набора. | Своя история и права у каждого сайта. |
| R7 | Интеграция git | Поддержка **внешних GitHub/GitLab** (SSH/HTTPS-токен, webhook/поллинг) **и встроенных bare git**. `go-git` (чистый Go, единственный бинарник). | Привязка репо — штатная возможность. |
| R8 | Формат schema | Единый **JSON Schema** (проверка props и контента одним механизмом). Schema принадлежит ComponentDefinition, контент привязан через `schemaId`. | Одна валидация в рантайме, редакторе и backend. |
| R9 | Внешние пакеты компонентов | **Fetch + lock из CDN/registry** (jsDelivr/unpkg): UI редактора добавляет `pkg@version`; на Build Go скачивает предсобранный ESM-бандл, пишет `lockfile` (+integrity) в версию, кэширует в self-hosted-зеркало. esbuild бандлит против него. Сервер не ставит npm-пакеты (нет Node). | package.json-подобный UX без Node; версионирование и воспроизводимость через lockfile. |

## Открытые вопросы

— (нет)

## Зависимости компонентов (R9)

Структура манифеста версии (package.json-эквивалент):

```text
ComponentVersion (коммит)
    src/definition.tsx
    schema.json
    metadata.json
    deps.json        ← { "pkg@^1.2.3": ... } управляется из UI
    lockfile.json    ← реально запрошенные @version + integrity (генерирует backend)
```

Добавление пакета: UI → backend резолвит (`latest`/сем-версия) → получает ESM-бандл
с CDN → сохраняет в кэш-артефакт + запись `lockfile.json` → на Build esbuild бандлит
`definition.tsx` против кэшированных бандлов. Все загрузки pinned + integrity-check;
self-hosted-зеркало даёт оффлайн-режим.

## Целевой пайплайн сборки

```
Snapshot (ссылки на коммиты версий)
→ Workspace (temp-директория воркера:
     src/entry.tsx · src/definitions/*.tsx (checkout коммитов) · manifest.json)
→ esbuild.Build(...)   [Go, incremental]
→ dist/ + manifest.json (бандлы страниц, external на shared-библиотеки)
→ Build-артефакты по пути build/<site>/<environment>/<snapshot>/
→ Runtime отдаёт статику + /runtime/* контракт
```

`entry.tsx` (генерируется): статически импортирует все декларации из снапшота
и регистрирует их в `ComponentRegistry` до `boot()`.

Dev-режим (пересборщик): `fsnotify` по workspace → esbuild incremental
(`Incremental: true`) → WS-push новой декларации в рантайм (см. спеки boot).

## Новые компоненты backend (Go, в том же бинарнике)

- **Git-сервис** (`go-git`): init репо на сайт, привязка remote (GitHub/GitLab), fetch/checkout по версии, автопикинг webhook/поллинг, управление SSH-ключами/токенами.
- **Dependency-сервис** (R9): резолв `pkg@version`, fetch ESM-бандлов с CDN/registry, `lockfile` + integrity в версии, self-hosted кэш-зеркало.
- **Build-сервис**: материализация workspace из снапшота, esbuild-сборка, статусы `Build` (`queued → installing → building → ready/failed`), логи.
- **Runtime-контракт**: Go-endpoints `/runtime/contract`, `/runtime/tree`, `/runtime/routes`, `/runtime/tokens` (+ существующие admin-роуты).
- **Artifact store**: дисковый корень `build/<site>/<env>/<snapshot>/` (позже — S3).

## Нолог для редактора (admin)

- React-app `admin/`: дерево страниц, редактор props/content по schema (live-валидация), bindings, темы, зависимости (package.json-like UI), git-операции (commit/push/tag), публикация, статусы Build.