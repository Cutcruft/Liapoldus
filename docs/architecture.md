# Архитектура первой версии

## Цель

Первая версия проверяет главный вертикальный сценарий Liapoldus:

```text
Site → Page → ComponentTree → PageVersion → Snapshot
```

Backend реализован на Go как модульный монолит. Модули находятся в одном процессе и используют единый доменный слой, но не обращаются друг к другу через внутренние глобальные структуры.

## Границы модулей

- `domain` — сущности, ошибки и контракты репозиториев.
- `site` — создание и чтение сайтов.
- `page` — создание страниц, проверка дерева компонентов и выпуск версий.
- `snapshot` — фиксация последних версий страниц сайта.
- `store` — текущая in-memory реализация persistence-портов.
- `httpapi` — REST transport и преобразование HTTP-запросов в application-вызовы.
- `cmd/server` — композиция зависимостей и жизненный цикл процесса.

## Направление внешних запросов

Внешние HTTP/GraphQL/API-запросы не проходят через Go backend, если они доступны браузеру. Frontend Provider или Plugin получает публичную конфигурацию Operation и обращается к внешнему источнику напрямую.

```text
Go API: Operation definition + public config
                 ↓
Vue frontend: Provider / Plugin
                 ↓
External API
```

Это сохраняет backend простым control plane и не превращает его в API Gateway.

Серверная обработка разрешена только для server-only случаев: секретные ключи, локальная БД, webhooks, внутренние сети и операции с обязательной серверной проверкой. Такой endpoint должен быть отдельной явно обозначенной capability, а не неявным прокси для любого Provider.

Такое разделение позволяет заменить `store.Memory` на PostgreSQL без изменений в `site`, `page`, `snapshot` и HTTP API.

## Дерево компонентов

Страница хранит `ComponentNode`, а не HTML и не DOM. DOM является результатом рендеринга во frontend.

```text
ComponentNode
├── id
├── type
├── props
└── children[]
```

В первой версии разрешены типы `Container`, `Text`, `Image` и `Button`. Проверяются обязательные идентификаторы, поддерживаемый тип и максимальная глубина дерева 64.

Позже список типов будет заменён на `ComponentDefinition` со schema. Это позволит frontend-редактору получать metadata, ограничения children и описание props из API.

## Версионирование

Создание страницы выпускает `PageVersion` номер 1. Обновление дерева не изменяет существующую версию, а создаёт следующую. `Page` хранит ссылку на актуальное состояние для удобного чтения.

`Snapshot` фиксирует ссылки на конкретные `PageVersion`. Поэтому последующая правка страницы не меняет уже созданный snapshot.

## Почему in-memory

Это стартовый адаптер для запуска и тестов без PostgreSQL. Он не предназначен для production: данные теряются после перезапуска, отсутствуют транзакции и горизонтальное масштабирование.

Следующий persistence-адаптер должен использовать PostgreSQL и JSONB для `ComponentNode.props` и дерева. Публичные контракты репозиториев уже отделены от реализации.

## Следующие этапы

1. PostgreSQL adapter, миграции и конфигурация.
2. `ComponentDefinition` и schema validation.
3. Route и runtime resolver.
4. Vue 3 editor с рекурсивным renderer и drag-and-drop.
5. Snapshot build worker через Vite.
6. Assets, content и bindings.
7. Direct frontend Operations, providers и plugin SDK.
