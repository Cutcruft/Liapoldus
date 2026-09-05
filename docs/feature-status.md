# Статус функций

`README.md` описывает целевую архитектуру Liapoldus. Этот файл показывает, что уже реализовано в текущем первом срезе.

## Реализовано

| Область | Статус | Что проверяется |
| --- | --- | --- |
| Go backend | Готово | Запуск модульного монолита |
| Site | Частично | Создание, чтение, уникальность slug |
| Page | Частично | Создание, чтение, список страниц |
| Component tree | Частично | `Container`, `Text`, `Image`, `Button`, props и children |
| Component validation | Частично | ID, тип компонента, глубина дерева |
| Page versioning | Частично | Новая версия при обновлении дерева |
| Snapshot | Частично | Фиксация актуальной версии каждой страницы |
| REST API | Частично | API из `docs/api.md` |
| Runtime health | Готово | `GET /healthz` |
| Persistence | Реализовано | PostgreSQL adapter и in-memory adapter |

## Пока не реализовано

- Route, matcher, priority и Redirect;
- ComponentDefinition, metadata и schema как отдельные объекты;
- Content и ContentTranslation;
- Asset storage и выдача ресурсов;
- Theme;
- Binding и runtime data sources;
- Operation, Provider и frontend SDK;
- прямые HTTP/GraphQL-интеграции из frontend;
- server-only operations;
- ESB extension layer (registry, metadata routing, `Call`/`Stream`);
- Forms и validation schema;
- Plugin management и npm dependencies;
- версии всех объектов кроме Page;
- полноценный Snapshot со всеми типами объектов;
- Vite build worker и static artifacts;
- Development/Production environments;
- runtime routing, publication и rollback;
- Redis/cache policies;
- production lifecycle и динамическая загрузка extension registry.

Наличие пункта в основном README не означает, что он уже доступен через API. Перед реализацией каждого следующего блока его нужно добавить в этот статус и покрыть тестами.
