# Backend — Git-модель компонентов. Тест-контракт (Этап 2)

Набор тестов — **контракт**, под который пишется реализация в `backend/`
(см. `docs/design/frontend.md` R4–R8 и `docs/ui-runtime/json-descriptors.md` §5–6).
Готово, когда все тесты зелёные: unit (моки репозиториев) + интеграция (реальный go-git на temp-директории).

## Границы этапа (что входит / не входит)

Входит:
- component-модель: `ComponentDefinition` (id, name, JSON Schema, metadata) как отдельный объект; `ComponentNode` → `definitionId` + `bindings`; сборка страниц из компонентов; валидация дерева против реестра.
- git-сервис: репо на сайт (внутренний **bare**, go-git); `ComponentVersion` = коммит (`src/definition.tsx` + `schema.json` + `metadata.json`); fetch/checkout по версии; rollback; история версий.

НЕ входит (отдельные заходы):
- сборщик esbuild и `Build`/артефакты (Этап 3);
- внешние remote GitHub/GitLab (SSH/HTTPS-токены), webhook/поллинг-автопикинг, управление ключами;
- Dependency-сервис (R9): `deps.json`/`lockfile.json`, CDN-кэш и integrity;
- schema на контент (`schemaId`) — контент привязан мягко (R4), остаётся как есть;
- dev-режим (fsnotify → incremental).

## Общие требования

- Runner: **go test**; TS strict не релевантен (сервер на Go).
- unit-тесты — `backend/tests/unit/*_test.go` (пакет `unit`, gomock); git-сервис unit-тестируется через интерфейс-мок `GitRepository`.
- интеграционные — `backend/tests/integration/git_repo_test.go`: реальный **go-git**, локальный bare в `t.TempDir()`, без сети и без Postgres.
- Никаких настоящих HTTP-запросов; все ресурсы — temp-директории.
- Ошибки — по существующим sentinel: `domain.ErrNotFound`, `domain.ErrAlreadyExists`, `domain.ErrInvalidRequest` (`internal/domain/model.go`); новые: `domain.ErrRepoNotInitialized`, `domain.ErrSchemaInvalid`, `domain.ErrVersionNotFound`.
- JSON Schema валидируется Go-библиотекой (кандидат `github.com/santhosh-tekuri/jsonschema/v6`); API для проверки — в спеку: `ValidateSchema(schema any) error` и `ValidateProps(props any, schema any) []string`.
- Дерево/биндинги в контракте соответствуют ui-runtime (`json-descriptors.md` §5–6): узел `{instanceId, definitionId, props, bindings:[{property, source}], children}`, sources: `content | routeParam | routeQuery | operation | form | props | runtime`.
- Фикстуры общие — в `backend/tests/unit/fixtures_test.go` (пакет unit).

---

## §1. `schema_test.go` — JSON Schema (валидация)

Коллбэк: schema приходит как `map[string]any` (JSON).

1. `ValidateSchema` валидной schema → nil.
2. schema без `type` на корне → **ErrSchemaInvalid**.
3. schema, являющаяся массивом/строкой (не объект) → **ErrSchemaInvalid**.
4. schema с невалидным keyword (напр. `type: "unknown"` на примитиве) → **ErrSchemaInvalid**.
5. `ValidateProps({title:"Карточка"}, schema {type:object, required:[title], properties:{title:{type:string}}})` → `[]` (nil).
6. `ValidateProps({}, ...)` без required `title` → ошибка по пути `$.title`.
7. `ValidateProps({title: 42}, `type: string`)` → ошибка по пути `$.title`.
8. `ValidateProps` на вложенном объекте (property `meta.published: boolean`) → корректный путь `$.meta.published`.
9. `ValidateProps(props, nil)` (schema нет) → ок, без валидации.
10. неточный тип (JSON `null` vs отсутствие) — `null` не проходит против `type: string`; отсутствие поля при не-required — ок.

## §2. `component_test.go` — реестр ComponentDefinition

Единицы: `internal/application/component` (сервис), репозиторий `ComponentRepository` (gomock), git — через `GitRepository` (мок). Изменения идут только через git-коммит (source of truth — репо, R5).

Фикстура определения:
`{ id: "card", name: "Карточка статьи", kind: "component", source: "export default ...", schema: {...}, metadata: {label:"Card", version:1} }`.

1. `Define` валидного определения (schema корректна) → создаёт `ComponentVersion` (коммит в git-моке с сообщением вида `component card: Карточка статьи`), в реестре актуальные имя/schema/metadata.
2. `Define` с невалидной JSON Schema → **ErrSchemaInvalid**, коммит НЕ создавался (git-мок не дёргался), реестр не изменён.
3. повторный `Define` того же `id` → **ErrAlreadyExists**.
4. `Get(id)` существующего → определение; `Get` неизвестного → **ErrNotFound**.
5. `List(siteId)` → все определения сайта.
6. `Update(id, patch{name,schema,source})` → новый коммит, `ComponentVersion.ID != прошлого`; реестр перезаписан.
7. `Update` неизвестного `id` → **ErrNotFound**.
8. `Update` с schema, ломающей текущие props страниц? — props страниц не хранятся в определении, поэтому **ок** (валидация страниц ленивая, на сборке; жёсткая на записи дерева — §3).
9. `Delete(id)` → `Get` → **ErrNotFound**; git-мок подтверждает: **история коммитов цела** (Delete не удаляет историю).
10. source отсутствует (`source: ""`) → **ErrInvalidRequest** (компонент обязан иметь `.tsx`-исходник).
11. `id` с недопустимыми символами (тире/пробел в начале, `../`) → **ErrInvalidRequest** (идентификатор — безопасный slug, используется в пути репо).

## §3. `page_assembly_test.go` — сборка страниц из компонентов

Единицы: `internal/application/page` расширяется валидацией/парсингом дерева.

Фикстура дерева (контракт ui-runtime §5):

```jsonc
{
  "root": {
    "instanceId": "i1",
    "definitionId": "layout.main",
    "props": { "variant": "wide" },
    "bindings": [],
    "children": [
      {
        "instanceId": "i2",
        "definitionId": "card",
        "props": { "image": "https://cdn.test/pic.png" },
        "bindings": [
          { "property": "title", "source": { "type": "content", "contentId": "c1", "path": "title" } }
        ],
        "children": []
      }
    ]
  }
}
```

1. входящий JSON парсится в `ComponentNode{ID: instanceId, DefinitionID, Props, Bindings, Children}` без потерь (roundtrip: `Serialize(Parse(json))` == каноничный JSON).
2. bindings парсятся: типы `content`, `routeParam`, `routeQuery`, `operation`, `form`, `props`, `runtime` — валидный source формируется без ошибок.
3. binding без `property` → **ErrInvalidRequest**; без/с неверным `source.type` → **ErrInvalidRequest**.
4. `source: {type:"content"}` без `contentId` → **ErrInvalidRequest** (для `routeParam`/`routeQuery` требуется `name`).
5. сохранение страницы с непустым дерево валидирует определения: любой `definitionId` без реестра (в т.ч. на глубине 5) → **ErrNotFound** с сообщением про definitionId.
6. корневой узел пуст (`definitionId: ""`) → **ErrInvalidRequest**.
7. props против schema определения `card` (`title` required) с `props: {theme:"dark"}` → ошибка **по пути** с указанием узла i2 и ключа `$.title`; страница не сохраняется.
8. props валидные против schema → сохраняется (создаётся `PageVersion` с тем же деревом).
9. глубина дерева > конфигурационного `MaxDepth` (default 32) → **ErrInvalidRequest**.
10. дети > конфигурационного `MaxChildren` (default 100) → **ErrInvalidRequest**.
11. мягкость контента (R4): binding на несуществующий `contentId` — запись страницы проходит без ошибок (контент проверяется в рантайме, не на записи).
12. `definitionId` = `layout.main` с хорошим определением в реестре, но дерево ставит его как *дочерний* узел (нарушение `kind`-констрейнта, если в metadata указан `kind: "layout"` с запретом) → фактически: metadata не обязательное правило валидации; такой узел **валиден** (ограничения по kind вводятся позже, не блокируют Этап 2).

## §4. `git_service_test.go` — git-сервис (мок GitRepository)

`internal/application/git`. Интерфейс:

```go
type GitRepository interface {
    Init(ctx context.Context, siteId string) error
    Commit(ctx context.Context, siteId, definitionId, message string, files map[string][]byte) (sha string, err error)
    ListVersions(ctx context.Context, siteId, definitionId string) ([]string, error)
    Checkout(ctx context.Context, siteId, sha string) (map[string][]byte, error)
    Head(ctx context.Context, siteId string) (sha string, err error)
}
```

1. `InitRepo(siteId)` — идемпотентно: повторный call не ошибка; первый — дергает git-мок `Init`.
2. `Release(siteId, definitionId, source, schema, metadata)` → пишет файлы `src/definition.tsx`, `schema.json`, `metadata.json` одним коммитом (сообщение `component <id>: <name>`), возвращает `ComponentVersion{ID: sha, SiteID, DefinitionID}`.
3. порядок файлов в коммите детерминирован (сортировка ключей) — сравнимо в тестах.
4. `Releases(siteId, definitionId)` → `[]ComponentVersion` по `ListVersions` (хронологический порядок).
5. `CheckoutVersion(siteId, sha)` → файлы из `git.Checkout`.
6. `Rollback(siteId, definitionId, sha)` → делает `Checkout(sha)` и перезаписывает актуальное состояние в реестре (для компонента `definitionId`), создаёт новый коммит-откат (история растёт, не переписывается).
7. `CheckoutVersion` неизвестного sha → **ErrVersionNotFound**.
8. `InitRepo` без предварительного `Init` (пустой мок: `ErrRepoNotInitialized` от клиента) → пробрасывается **ErrRepoNotInitialized** из commit/rollback-операций.
9. `Rollback(definitionId)` без версий → **ErrVersionNotFound**.
10. транзакционность: если `Checkout` вернул ошибку, реестр не перезаписывается (не частичное состояние).

## §5. `git_repo_test.go` — интеграция: реальный go-git на локальном bare

`backend/tests/integration/git_repo_test.go`, пакет `integration`. Используется `t.TempDir()`:

структура temp:
```
<tmp>/sites/<siteId>/repo.git     (bare)
<tmp>/workspace/<siteId>          (клоны/checkout, для чтения)
```
Размещение вне контента/ДБ подтверждает R6.

1. `InitRepo` создаёт bare-репозиторий на диске (`HEAD` существует, `gitdir` = repo.git); повторный init — ок.
2. `Commit` двух компонентов (`card`, `layout.main`) → 2 коммита; файлы в дереве коммита: `src/definition.tsx`, `schema.json`, `metadata.json`; sha-версии различны.
3. `ListVersions(card)` → ровно 2 sha (после двух правок) в хронологическом порядке; первый sha — не пустой.
4. `CheckoutVersion(sha-card-v2)` → содержимое `src/definition.tsx` соответствует v2 (не v1) → доказывает честный checkout по версии.
5. содержимое `schema.json` после `Release` валидно через `ValidateSchema`.
6. `Rollback(card, sha-card-v1)` → в реестре определение v1; итоговая история содержит 3 коммита (не усечена).
7. `CheckoutVersion` произвольного hex-несуществующего sha → **ErrVersionNotFound**.
8. удаление `Delete(card)` НЕ удаляет коммиты: `ListVersions` по-прежнему возвращает историю (source of truth в git).
9. снапшот страницы: после создания страницы с `card` v1 и последующего `Release(card v2)` страница по-прежнему собирается с v1 (дерево хранит ссылку на компонент + props; версия фиксируется снапшотом/версией страницы, не «текущая головная»).
10. e2e (сервис-уровень): `Define card → Release v1 → Define layout.main → собираем страницу из [layout.main, card] → обновляем card (v2) → Rollback на v1` — финальная сборка использует v1, история канонична.

---

## Порядок внедрения (рекомендуемый)

1. `schema_test.go` + `domain` (модель ComponentDefinition/ComponentVersion/Node/Binding) + sentinel-ошибки.
2. `component_tests.go` + реестр (моки; git-мок-интерфейс).
3. `page_assembly_test.go` + валидация/парсинг дерева в page-сервисе.
4. `git_service_test.go` + git-сервис поверх интерфейса.
5. `git_repo_test.go` + реализация go-git (внутренний bare).
6. Обновить `docs/feature-status.md` (ComponentDefinition/schema/bindings, ComponentVersion, git-сервис) и закрыть пункты в `TODO.md`.

Каждый шаг зелёный независимо, коммит по шагам (как в Этапе 1).

## Примечания от пользователя

- Источник правды исходников — git (R5); в Postgres — только метаданные/ссылки на версии, не сам код.
- Один репозиторий на сайт (R6); binding-модель и формат дерева — единые с `ui-runtime` (json-descriptors §5–6).
- props — жёсткая валидация по JSON Schema на записи дерева; контент, на который ссылаются bindings, — мягкий (R4).
- Внешние интеграции и автопикинг — отдельные заходы; сейчас внутренний bare + ручной `Release`/`Rollback`.