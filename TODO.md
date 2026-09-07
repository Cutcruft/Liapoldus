# TODO — план действий Liapoldus

Детальный план работ. Единая спека редизайна — `docs/redesign/spec.md` (решения R1–R19,
доменная модель, API, интерфейс, план слайсов R0–R13); настоящий файл — рабочий прогресс
с конкретными шагами, файлами и критериями готовности.

- Команды тестов: `make test` (backend `go test ./...` + vitest), `make lint`
  (`go vet` + typecheck). По-пакетно: `cd backend && go test ./...`;
  `cd admin && npm run typecheck && npm test`; `cd ui-runtime && npm test`;
  `cd ui-kit && npm test`.
- Метки приоритета: **P0** — блокирует дальнейшее; **P1** — основное; **P2** — после базового.
- Метки области: **[B]** backend (Go), **[F]** admin (React/vite), **[R]** ui-runtime,
  **[K]** ui-kit, **[Docs]** документация, **[CI]** сборка/инфраструктура.

---

## Фаза 0 — Закоммитить текущий прогресс

Незакоммиченная работа в рабочей директории: комплект К4/К5 (плоская модель
`Page.list`, admin-порт редактора на лист, ui-runtime 262/262) и чистка документации.

- [x] **P0** Проверить статус: `git status`, `git diff --stat`. Убедиться, что в diff нет
      секретов/мусора (`data/`, `node_modules/`, `backend/tmp`, `backend/bin`).
- [x] **P0** Прогнать полный тестовый набор перед коммитом: `make test` и `make lint`.
      *(Попутно поправлен go vet: пустые append в builder.go/rebuilder.go.)*
- [x] **P0** Сделать коммиты по логическим группам (НЕ сливать в один):
  - `feat(ui-runtime): list-based PageRenderer/ElementNode, routeGroup bindings, boot/store/tree (K4)` —
    ui-runtime + удалённый `ui-runtime/src/types/tree.ts` + новый `ui-runtime/src/types/page.ts`;
  - `feat(admin): editor port to flat page list, list-utils, Inspector/TreePanel/PreviewPane rewrite (K5)` —
    admin editor + runtime admin;
  - `feat(backend): pages list wire contract, Route entity, migration 011_pages_list` —
    domain/model.go, route.go, handlers, postgres.go, migration;
  - `docs: consolidate redesign+API into docs/redesign/spec.md, merge json-descriptors, drop obsolete docs` —
    docs-файлы, README, build-test-spec;
  - `docs: remove completed todo-mvp.md` (отдельно — файл в корне, не попал в `git add docs/`).
- [x] **P0** Верификация после коммита: `git log --oneline`, рабочая директория чиста
      (`git status` без незакоммиченного), `make test` повторно зелёный.

**Done**: консервируем состояние К4/К5 до начала слайсов R7–R13 (5 коммитов: e40b447, 55b9cb8, 50652f2, 4306044, 204fcfe).

---

## Фаза 1 — R7: Новая модель страниц (данные)

Сердце перестройки. Контракт-часть выполнена (R7 «контракт» из спеки §8):
`Page.list: Element[]`, `ElementNode`, `BindingSource`, `PUT /api/pages/{pageID}`
body `{name, list}`, `POST /api/sites/{id}/pages` body `{name, slug, list}`;
admin-редактор переведён на лист. Осталось доработать сервисный слой и снапшоты.

Файлы: `backend/internal/domain/model.go`, `backend/internal/application/page/service.go`,
`backend/internal/application/snapshot/service.go`, `backend/internal/application/gitsnapshot/*`,
`backend/internal/infra/build/materializer/materializer.go`, `backend/internal/api/admin/handler.go`.

- [x] **P0 [B]** CRUD страниц/роутов на листе: проверить полноту валидации
      `validateList`/`validateElementProps` (page/service.go:126–172): schema-типы,
      обязательные поля, лимит элементов, `assignElementIDs` для вложенных элементов.
- [x] **P0 [B]** Валидация matcher-регулярок роута: `backend/internal/domain/route.go` +
      handler — ошибка на невалидный/неякоренный regex (`^...$`), нормализация `priority`.
      *(Добавлена якорность в backend route.Service + admin validateRouteFields/`matcherAnchored`.)*
- [x] **P1 [B]** Перестройка обработчиков содержимого страниц: `PageVersion`/снапшотные
      ссылки → `list`; проверить `list_versions`/`PageVersion{List}` на листе
      (model.go:100–106 уже использует `List` — покрыть тестами get/list версий).
      *(Добавлены проверки versions API в TestSiteAndPageFlow.)*
- [x] **P1 [B]** Снапшот/материализация по листу: `collectElementDefs(version.List, ...)`
      (materializer.go:223) — список componentId из элементов вместо обхода дерева;
      покрыть снапшот-сборку по листу тестами (`gitsnapshot_test`, `materializer_test`).
- [x] **P1 [B]** Убрать остаточные tree-ссылки в сервисном слое, если остались
      (проверить `grep -rn Tree t page/snapshot/gitsnapshot`). *(Комментарий в page/service.go.)*
- [x] **P1**: Тесты: CRUD страниц/роутов, порядок листа, regex-группы, снапшот-сборка по листу
      (`backend/tests/unit/page_test.go`, `admin_handler_test.go`, `runtime_*_test.go`).
      *(Итог: backend ok, ui-runtime 262/262, admin 305/305.)*

**Done**: backend полностью живёт на листе; ни одной ветки дерева в коде страниц.

---

## Фаза 2 — R8: Раздел «Страницы»: реестр маршрутов (UI)

Frontend на модель R7. Роут-API на бэке уже есть (`routeHandler` CRUD в server.go:134–141).
Композиция страницы (лист, drag, блок элемента, биндинги) перенесена в R13.

Файлы: `admin/src/app/SiteTab.tsx` (редакторный раздел `pages` — placeholder),
`admin/src/app/components/*`, `admin/src/app/pages/SiteRoutesPage.tsx`/`RouteEditorPage.tsx`
(legacy, dev-only — можно переиспользовать логику).

- [x] **P0 [F]** Раздел «Страницы» в редакторе (SiteTab `section=pages`): реестр роутов —
      list/create/update/delete; поля matcher (regex), priority, action
      (`renderPage` → pageId | `redirect` + status + keepQuery).
      *(Реализован `admin/src/app/pages/PagesSection.tsx`: реестр с формой создания и
      удалением через ConfirmButton; delete/update — через `runOperation`. Редактирование
      пока ведёт в legacy `RouteEditorPage` (`route.edit`); связку с контент-редактором
      переимпортирует R13.)*
- [x] **P1 [F]** Таблица «группы regex → props»: предпросмотр захваченных групп матчера
      против тестового path (map `$1, $2…` → props, ср. `routeGroup.index` в ui-runtime).
      *(Филд «Тестовый path (превью групп)» в форме создания: `matchGroups()` раскрывает
      `$1 → value` для валидного матчера; привязки групп к пропсам — в контент-редакторе R13.)*
- [x] **P1 [F]** Связка роут→страница: из `renderPage.pageId` переход в контент-редактор
      (создаётся в R13; на этом слайсе — колонка «Страница» с pageId; переход появится в R13).
- [ ] **P1 [B]** (если дельты найдутся в e2e) роут-CRUD валидации по спеке §3.8.
- [x] **P2 [F]** Респонсив/UX: конфликт overlapping матчеров через `overlapWarnings`
      (admin route-utils) в UI. *(Блок предупреждений в форме создания; для legacy-редактора
      уже было в RouteEditorPage.)*
- [x] Тесты: `SiteTab.test` (раздел pages), CRUD роутов, валидация матчера, связка роут→страница.
      *(`PagesSection.test.tsx`: реестр + колонка страницы, POST `{matcher,priority,action}`,
      валидация regex и якорности, превью групп `$1 → 42`, переход; тема дублей — статические
      тесты `route-utils.test.ts`)*

**Done**: реестр маршрутов управляется из нового раздела; legacy роут-страницы можно закрыть
(остаются в AppRoutes для редактирования деталей и back-nav, пока не готов переезд на inline-редактор R13).

---

## Фаза 3 — R9: Менеджер состояний

Снапшоты и публикация (pin активного прод-снапшота).

Файлы: `backend/internal/domain/model.go` (нет сущности Deployment),
`backend/internal/application/snapshot/service.go`, `backend/internal/api/admin/handler.go`
(снапшот-роуты server.go:156–159 уже есть), `admin/src/app/SiteTab.tsx` (`section=states` —
placeholder).

- [ ] **P0 [B]** Сущность `Deployment` в домене: id, siteId, snapshotId (активный прод),
      environment, createdAt. Роуты `POST /api/sites/{siteID}/deployments`,
      `GET /api/sites/{siteID}/deployments/active`, `POST .../rollback`. *(R13 из спеки; см. §3.9.)*
- [ ] **P0 [B]** Миграция `012_deployments.sql`: таблица deployment, FK на snapshots; идемпотентность.
- [ ] **P1 [B]** Операции pin/rollback: публикация снапшота = переключение активного
      Deployment (снапшот-версия → индекс). Идемпотентность повторной публикации.
- [ ] **P1 [F]** Раздел «Состояния»: список снапшотов (dev/prod + статусы), «создать снапшот»,
      «выпустить в прод» (confirm), «откатиться» (пин более раннего prod-снапшота).
- [ ] **P1 [F]** Индикатор «есть незакоммиченные правки → создать снапшот» (по версиям
      сущностей vs последний снапшот; источник — дельта `updatedAt`/version).
- [ ] Тесты: выпуск/откат/idempotent (unit + `admin_r5_test`-стиль), UI-флоу (SnapshotSection.test).

**Done**: прод-состояние сайта управляется одним активным снапшотом; откат — пин старее.

---

## Фаза 4 — R10: База сайта (site-settings)

Backend + Frontend (R14 из спеки).

Файлы: `backend/internal/domain/*` (нет SiteSettings), `backend/internal/api/admin/settingsHandler`,
`admin/src/app/SiteTab.tsx` (`section=base` — placeholder), `docs/redesign/spec.md` §3.3/§5.5.

- [ ] **P0 [B]** Entity `SiteSettings` + CRUD: id/siteId + jsonb; поля: favicon (assetId),
      lang, meta (title/description/og), фолбэки дефолтов. Миграция `013_site_settings.sql`.
- [ ] **P1 [B]** Контракт runtime и index.html получают мета/фавиконку: расширить
      `runtime/contract.go`/материализатор (siteSettings в контракт, `<link rel="icon">` в shell).
- [ ] **P1 [F]** Раздел «База»: форма site-settings; интеграция фавиконки (пикер из Медиа,
      `AssetPicker`); og-meta; превью title/description.
- [ ] **P2 [F]** Темы/токены (наследуемый токен-редактор, `SiteTokensPage` → раздел),
      шрифты как ассеты, базовые шаблоны/заготовки (импорт).
- [ ] Тесты: CRUD settings, мета в контракте и HTML (`runtime_contract_test`,
      `materializer_test`), UI-форма (BaseSection.test).

**Done**: метаданные сайта/фавиконка из конфигурации попадают в контракт и сборку.

---

## Фаза 5 — R11: ui-runtime: рендер по листу и route-группы

Первый пункт слайса выполнен (К4, `[x]` в спеке §8). Продолжить по оставшимся пунктам.

Уже есть: `PageRenderer`/`ElementNode` рендер по листу; `routeGroup` binding уже реализован
в `ui-runtime/src/types/page.ts`, `src/core/tree.ts:49`, `src/core/descriptor.ts:242`.

- [ ] **P1 [R]** Regex-группы роута → props: собрать группы матчера в `ResolvedRoute.params`
      и подставить в binding `routeGroup.index` при рендере (покрыть в `render.test`/`tree.test`).
- [ ] **P1 [R]** `RouteOutlet`/`initialPage`: типы контракта расширены siteSettings + page list
      (`src/types/descriptor.ts`); boot-дерево домашней страницы (пеервый пейнт).
- [ ] **P2 [R]** Синхронизация с CRUD-дескрипторами (operations/forms уже через ApiClient);
      убедиться, что `content.get/list/batch` несут актуальные `poll.schedule`.
- [ ] **P1 [Docs]** Обновить `docs/ui-runtime/spec.md` (постраничная доставка §16, routeGroup §A6).
- [ ] Тесты рантайма: `npm test` в ui-runtime; не сломать 262/262.

**Done**: рантайм выдаёт страницу по листу с группы regex, контракт включает siteSettings.

---

## Фаза 6 — R12: Удаление старой модели и e2e

Порт редактора выполнен (К5, `[x]`). Осталось удалить бэкендовые tree-хвосты и закрыть e2e.

- [ ] **P0 [B]** Удалить tree-операции/поля бэка: проверить миграции 001–010 и код на
      `root`/tree-ветки (`grep -rn "root\\b\|TreeVersion\\|tree" backend/internal`),
      контент/формы/ассеты не трогать. Миграция 011 уже дропнула `root` из pages.
- [ ] **P1 [CI]** e2e «создать сайт → компонент → страница → выпуск → откат»: обновить
      `backend/scripts/test.sh` — сейчас smoke-скрипт использует **старый** page API
      (`POST /api/pages/{id}/tree`, root-container); перевести на лист
      (`{name, slug, list}`, `PUT /api/pages/{pageID}` body `{name, list}`).
- [ ] **P1 [Docs]** Финал документации: README-раздел админки переписан, `docs/redesign/spec.md`
      статусы R7–R13 отмечены `[x]`, остальные доки без битых ссылок.
- [ ] **P2 [F]** Удалить legacy-роуты/страницы admin (`LEGACY_ROUTES` в AppRoutes.tsx), когда
      все разделы переедут на SiteTab; оставить в dev до R13.

**Done**: в коде и API нет ни одного упоминания старого дерева; e2e на листе.

---

## Фаза 7 — R13: Контент-редактор (дизайнерский workspace) и ассеты в схеме

Центральный экран для дизайнеров (R18/R19 из спеки). Самый крупный слайс.

Файлы: `backend/internal/application/component/schema.go` (schema-вывод), `component_handler.go`,
`admin/src/app/content/*`, `admin/src/app/editor/*`, `admin/src/app/rich-text/*`,
`docs/redesign/spec.md` §2.3.1, §5.4.

- [ ] **P0 [B]** Schema компонента расширяется контент-слотами: `role: content|config`,
      типы значений + `asset`/`asset[]` (kinds), `label`/`required`/`default`,
      `defaultSource?: BindingSource`; авто-вывод из TSX (R5) + ручные уточнения;
      валидация/миграция модели схемы (см. `schema-utils.ts` admin).
- [ ] **P0 [B]** Admin-операции элементов: `addElement`/`moveElement`/`removeElement`/
      `updateElement` (поверх `updatePage`); `createContentFromElement` (авто-создание
      Content-рекорда, collectionId=componentId, биндинг слотов `<contentId>.<field>`).
- [ ] **P1 [B]** Usage «где используется»: расширить на `Element.props` (литералы
      asset/значения), `Content.Fields` всех локалей, `Form.Definition`, `SiteSettings.faviconAssetId`
      (assets/usage).
- [ ] **P1 [F]** Workspace «Контент»: слева страницы; внутри страницы — лист элементов
      (drag = структура); клик → предпросмотр-редактор.
- [ ] **P1 [F]** Предпросмотр-редактор: live canvas (ui-runtime `PageRenderer` по листу, R11)
      + схема-зависимые контролы; выбор/создание Content-рекорда; переводы
      (base+locale = «вариативность», см. ContentEditor/JsonFieldsEditor).
- [ ] **P2 [F]** Биндинг-пикеры: per-слот literal | binding
      (content/operation/endpoint/form/query/routeGroup).
- [ ] **P2 [F]** Ассеты: тип `asset` во всех редакторах (пикер+загрузка+превью), превью-URL
      и runtime-URL через AssetResolver (admin `AssetPicker`/`AssetFieldControl`).
- [ ] Тесты: schema-слоты/валидация, element-мутации, createContentFromElement, workspace
      (композиция/drag/контролы/превью), usage по ассетам.

**Done**: дизайнер собирает страницу из листа в live-превью, связывая контент и ассеты по схеме.

---

## Фаза 8 — Работы вне редизайна (из спеки §10)

Не требуют редизайна, доводят текущую систему.

- [ ] **P2 [CI]** Ликвидация shell-обёрток CI: `backend/scripts/test.sh` (smoke на лист),
      `backend/scripts/lint.sh` (gofmt+vet+golangci), `backend/scripts/build-shared/build.mjs`
      (собрать в Go: общий `build-shared` уходит на `cmd/dependency-build` — react-семья уже покрыта).
- [ ] **P2 [CI]** SBOM/audit-экспорт зависимостей: `npm audit`, `govulncheck`/`go vuln`,
      CycloneDX для go.mod/package-lock; цепочка в CI.
- [ ] **P2 [B/R]** Общие wire-типы admin↔ui-runtime: `AssetMeta`/`FormDefinition`/`Route`
      дублированы в `admin/src/runtime/types.ts` и `ui-runtime/src/types/` — вынести общий пакет.
- [ ] **P2 [CI/Docs]** Development/Production изоляция: окружения не мешают друг другу,
      перенос только через снапшот; задокументировать.
- [ ] **P2 [CI/Docs]** Site isolation: один сайт не зависит от состояния другого;
      задокументировать границы.
- [ ] **P2 [B]** Логирование/метрики Build: структурированные логи сборок, гистограмма
      времени/размера бандла, алерты на падение сборки.
- [ ] **P2 [Docs]** Обновить архитектурную документацию под актуальную модель (README §32 → spec.md).

---

## Открытые вопросы

- Разбиение Фазы 0 на 4 коммита — подтвердить порядок/имена (используя стиль репо: `feat(area): …`, `docs: …`).
- R9 «индикатор незакоммиченных правок»: определять по `updatedAt`/версиям сущностей или вводить
  явный флаг dirty в схеме? (Вынести в спецификацию R9 при реализации.)
- R10 темы/токены: переиспользовать существующий токен-редактор целиком или ограничить
  read-only наследование на этом слайсе? (Вынести в спецификацию R10 при реализации.)