# Статус функций

`README.md` описывает целевую архитектуру Liapoldus. Этот файл показывает, что уже реализовано в текущем первом срезе.

## Реализовано

| Область | Статус | Что проверяется |
| --- | --- | --- |
| Go backend | Готово | Запуск двух HTTP-серверов (admin `:8080`, client `:18080`) |
| Admin REST API | Готово | API из `docs/api-admin.md`: sites/pages/content/assets/routes/forms/snapshots |
| Client REST API | Готово | API из `docs/api-client.md`: content (merge по locale), assets, forms, `/runtime/*` |
| Edge routing | Готово | serve asset, redirect (включая группы `$1..$9` и keepQuery), render page → 404 |
| Site | Реализовано | Создание, чтение, список, обновление, удаление, `defaultLocale`, `hosts`, резолв по Host |
| Page | Реализовано | Создание, чтение, список, update tree, версии страницы |
| Component tree | Реализовано | `Container`, `Text`, `Image`, `Button`, props и children |
| Component validation | Реализовано | ID, тип компонента, глубина дерева |
| Page versioning | Реализовано | Новая версия при обновлении дерева |
| Snapshot | Реализовано | Фиксация актуальной версии каждой страницы, чтение, версии, удаление |
| Content | Реализовано | Поля + переводы по locale, merge на стороне клиента, batch, коллекция `strings` |
| Asset storage | Реализовано | Загрузка (multipart), метаданные с variants, выдача байтов с ETag, удаление |
| Routes | Реализовано | matcher (regex) + priority, действия serve-asset/redirect/render-page, валидация |
| Forms | Реализовано | Определение, серверная валидация (required/minLength/email), сабмиты |
| Auth (admin) | Реализовано | Bearer token через `LIAPOLDUS_ADMIN_TOKEN` (пустой = открыто) |
| Runtime health | Готово | `GET /healthz` на admin и client |
| Persistence | Реализовано | PostgreSQL adapter (миграции `001`, `002`) и in-memory adapter |

## Пока не реализовано

- ComponentDefinition, metadata и schema как отдельные объекты;
- Theme;
- Binding и runtime data sources;
- Operation, Provider и frontend SDK;
- прямые HTTP/GraphQL-интеграции из frontend;
- server-only operations;
- ESB extension layer (registry, metadata routing, `Call`/`Stream`);
- Plugin management и npm dependencies;
- версии всех объектов кроме Page;
- полноценный Snapshot со всеми типами объектов;
- Vite build worker и static artifacts;
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