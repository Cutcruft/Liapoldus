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
- `store` — реализации persistence-портов для in-memory и PostgreSQL.
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

Для backend-расширений добавлен отдельный универсальный ESB gRPC layer. Он не смешан с typed management API: расширения регистрируют операции в `esb.Registry`, а transport маршрутизирует opaque payload по metadata `service`/`method`. Описание находится в [docs/esb.md](esb.md).

Такое разделение позволяет выбирать `store.Memory` или PostgreSQL без изменений в `site`, `page`, `snapshot` и HTTP API.

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

Это адаптер для unit-тестов и локальной разработки без PostgreSQL. Он не предназначен для production: данные теряются после перезапуска.

PostgreSQL adapter использует JSONB для дерева `ComponentNode` и транзакции для создания страниц, версий и snapshots. Публичные контракты репозиториев отделены от реализации.

## Следующие этапы

1. `ComponentDefinition` и schema validation.
2. Route и runtime resolver.
3. Vue 3 editor с рекурсивным renderer и drag-and-drop.
4. Snapshot build worker через Vite.
5. Assets, content и bindings.
6. Direct frontend Operations, providers и plugin SDK.
