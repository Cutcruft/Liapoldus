# Editor / Admin SPA — спецификация (Этап 6)

Спецификация React-редактора `admin/` для разработки и эксплуатации сайтов Liapoldus.

- Связанные документы: `README.md` (архитектура), `docs/api-admin.md` (REST-контракт), `docs/api-client.md` (runtime-contract), `docs/ui-runtime/spec.md`, `docs/feature-status.md`.
- Статус: **дизайн согласован, M0 в разработке**. Открытые вопросы закрыты (см. §2).

---

## 1. Цель

`admin/` — это SPA-редактор поверх admin REST API с живым превью. Он закрывает пункты **Этапа 6** TODO:

1. Дерево страниц (страницы/роуты), props/content по schema (live-валидация), bindings.
2. Темы и токены; UI зависимостей «как package.json» (R9).
3. Git-операции из UI: commit/push/tag, новая версия, снапшот.
4. Публикация (снапшот → Build → environment), статусы Build, rollback.

Рантайм-ядро редактора строится **на `@liapoldus/ui-runtime`**: все запросы — через его `ApiClient`, локализация — через `I18n`, реальное время — через его transport/SyncEngine. Позиционирование в UI — через собственный layout-кит `@liapoldus/ui-kit` (Tailwind v4, без кастомного CSS). Rich-text — Tiptap со slash-меню.

## 2. Согласованные решения

| # | Область | Решение |
| --- | --- | --- |
| E1 | Сборка/раздача | Vite SPA в монорепо; dev-server проксирует admin `:8080` и client `:18080`; прод-сборку отдаёт Go-бинарник на `/admin/*`. Node нужен только разработчику (zero-node рантайм не трогаем). |
| E2 | ui-kit | Отдельный независимый npm-пакет `@liapoldus/ui-kit`, используется **только** в `admin/`. Layout-примитивы для позиционирования без CSS; Tailwind v4. |
| E3 | Data-слой | **ui-runtime `ApiClient`**: admin CRUD объявляется декларативно (descriptor'ами) в `RuntimeRegistry`, транспортом служит расширенный `HttpTransport` (auth-заголовок + парсинг `{"error":...}`). |
| E4 | Роутинг админки | React Router (nested: `/` → `/sites` → `/sites/:siteId/...`). |
| E5 | Превью | Встраиваем живой рендер: iframe на клиентский URL реального dev-билда; авто-сборка после автосейва; мгновенный рефреш по `DevRebuildEvent` (WS `/dev/build/ws`). |
| E6 | Draft-состояние | Правки дерева — в slice-store; автосохранение с дебаунсом → `PUT /tree` → version++. |
| E11 | Состояние | Админка **не импортирует zustand напрямую**. Состояние — через обёртку ui-runtime (`createSliceStore`/`useSelector`); zustand остаётся внутренней зависимостью ui-runtime. |
| E7 | Набор примитивов | Средний: Box, Stack, Inline, Columns, Grid, Spacer, Divider, Frame, Sidebar, SplitPane. |
| E8 | Стек тестов | vitest + @testing-library/react + jsdom (как в ui-runtime). |
| E9 | Локализация UI | ui-runtime `I18n` над локальным словарём строк (коллекция `strings` в `RuntimeStore.content`); detect через localStorage/navigator/defaultLocale. |
| E10 | Backend | Изменения только во frontend. Зазоры backend (Этап 4: `/runtime/tree|routes|tokens`) — зависимости/риски (§12), в этом плане не закрываются. |

## 3. Монорепо

```
Liapoldus/
  ui-runtime/   npm-пакет (есть). Админка подключает его по alias на `src/` (TS), build в dist — для публикации.
  ui-kit/       @liapoldus/ui-kit — layout-примитивы, lib-build через Vite. Только admin.
  admin/        React SPA (Vite). Единственный потребитель ui-kit + ui-runtime.
  package.json  npm workspaces: ["ui-kit","admin"]; скрипты dev/build/test/typecheck по пакетам.
```

Рутовые скрипты:

```jsonc
{ "workspaces": ["ui-kit", "admin"],
  "scripts": {
    "dev": "npm -w admin run dev",
    "build": "npm -w ui-kit run build && npm -w admin run build",
    "test": "npm -w ui-runtime test && npm -w ui-kit test && npm -w admin test",
    "typecheck": "npm -w ui-runtime run typecheck && npm -w ui-kit run typecheck && npm -w admin run typecheck" } }
```

Vite-прокси admin:

```ts
proxy: {
  '/api/*':  { target: 'http://localhost:8080',  changeOrigin: true },
  '/runtime/*': { target: 'http://localhost:18080', changeOrigin: true },
  '/build/*': { target: 'http://localhost:18080', changeOrigin: true },
  '/dev/*': { target: 'http://localhost:18080', changeOrigin: true, ws: true },
}
```

## 4. ui-kit — layout-примитивы

Примитивы **только** про позиционирование; рендерятся в Tailwind-утилиты (v4, классы генерируются как статические строки). Стилизация (цвета, кнопки, поля) — вне кита, в админке через shadcn-style компоненты на Tailwind.

### Шкалы

```ts
type Spacing = 0 | 1 | 2 | 3 | 4 | 5 | 6 | 8 | 10 | 12 | 16;   // × 4px
type Align   = 'start' | 'center' | 'end' | 'stretch' | 'baseline';
type Justify = 'start' | 'center' | 'end' | 'between' | 'around' | 'evenly';
type Pad     = Spacing | { x?: Spacing; y?: Spacing };
```

### Примитивы и API

| Примитив | Tailwind-маппинг | Ключевые props |
| --- | --- | --- |
| `Box` | `p-*`, `px-*`, `py-*`, `grow`, `basis` | `as?`, `pad?`, `grow?`, `className?` |
| `Stack` | `flex flex-col items-* justify-* gap-* p-*` | `gap`, `align`, `justify`, `pad` |
| `Inline` | `flex flex-row items-* justify-* gap-* flex-wrap?` | `gap`, `align`, `justify`, `wrap?` |
| `Columns` | `grid grid-cols-<count> gap-*` | `count` (1..6), `gap` |
| `Grid` | `grid grid-cols-* grid-rows-* col-span-*` | `cols`, `rows?`, `gap`, `span?` |
| `Spacer` | `flex-1` | `axis?` |
| `Divider` | `border-t` / вертикальный `h-full w-px` | `orientation?`, `space?` |
| `Frame` | `overflow-auto h-full` (или `h-*`) | `height?`, `overflow?` |
| `Sidebar` | flex: боковая панель `w-* shrink-0` + контент `flex-1 min-w-0 overflow-auto` | `side?`, `width?` (xs…xl) |
| `SplitPane` | flex + `[flex-basis:<ratio>%]` на первой панели | `direction?`, `ratio?`, `min?` |

Правила: каждый примитив принимает `className` (escape-hatch), не генерирует inline-`style` (кроме отсутствующих), имеет default `as`-элемент `<div>`. Респонсив не входит в v1.

## 5. Админка — архитектура

### 5.1 admin-runtime (реализация через ui-runtime)

```ts
// src/admin-runtime/
//   types.ts          доменные типы (Site, Page, ComponentNode, Build, ...) по docs/api-admin.md
//   operations.ts     OperationDescriptor[] для admin CRUD (провайдер `liapoldus.admin`)
//   admin-transport.ts AdminHttpTransport extends HttpTransport
//   admin-api.ts       createAdminApi(...): { registry, api: ApiClient, i18n, sites/pages/builds (typed helpers) }
//   token-store.ts     slice-store (ui-runtime createSliceStore): token, setToken, clearToken
//   store.ts           доменные slice-сторы редактора (draft страницы, выбранный site) через ui-runtime
//   strings.ts         RU-словарь UI-строк (контент `strings`)
```

- **Источник путей** — `<админ origin>` (относительные `/api/...`); в dev проксирует Vite, в prod Go отдаёт SPA и API с одного origin.
- **`AdminHttpTransport`** поверх exported `HttpTransport`: в `buildRequest` инжектит `Authorization: Bearer <token>` из token-стора (getter, не при построении), не-2xx разбирает как `{"error":...}` → `AdminApiError(status, message, body)`.
- **Типизированные helpers** не идут в ApiClient напрямую: описание admin-операций кладётся в `RuntimeRegistry`, helpers (`sites.list()`, `pages.updateTree(id, root)`, `builds.create(...)`) — тонкие функции над `api.query/mutate` с доменными типами.
- **События/WS**: `ApiClient.subscribe('dev.rebuild', ...)` через WS-провайдер `liapoldus.ws` (url `/dev/build/ws` + `?siteId=`); статусы Build — poll/WS по возможности.

### 5.2 Расширение ui-runtime public API (требуется)

В `ui-runtime/src/index.ts` добавить экспорты (типы и классы, которые admin переиспользует):

```ts
export { HttpTransport, substitutePath, buildHttpUrl } from './core/transport/http';
export type { Transport, TransportRequest, TransportResponse, FetchLike, TransportEnv, WebSocketLike, WebSocketCtor, EventSourceLike, EventSourceCtor } from './core/transport/transport';
export type { ProviderProtocol, OperationType, OperationScope, CachePolicy, ProviderDescriptor, SubscribeFields, OperationDescriptor, OperationTypeBinding, EndpointDescriptor, RouteDescriptor, ResolvedRoute, ThemeDescriptor, ThemeTokenDef, FallbackDescriptor, ContractDescriptor, Descriptor } from './types/descriptor';
export type { ResolvedOperation, ResolvedProvider } from './core/registry';
// Обобщённый store (zustand-обёртка): админка не импортирует zustand напрямую.
export { createSliceStore } from './core/slice-store';
export type { SliceStore } from './core/slice-store';
export { useSelector } from './react/use-selector';
```

Контракт: ничего не менять в поведении существующих экспортов; добавления — только ре-экспорт. Новые тесты ui-runtime покрывают «экспорт сущeствует и совместим с parseDescriptors/join-операциями».

### 5.3 AppShell и контексты

```ts
// src/app/main.tsx            — ReactDOM.createRoot
// src/app/router.tsx          — createBrowserRouter (routes §6)
// src/app/shell.tsx           — AppShell из ui-kit: Sidebar (nav) + Stack (topbar) + Frame (Outlet)
// src/app/admin-context.tsx   — React context с createAdminApi() (registry/client/i18n/token)
// src/index.css               — @import "tailwindcss";
```

AppShell: left `Sidebar` — навигация и переключатель сайтов; topbar — индикатор окружения dev/prod, статус Build, кнопка «Опубликовать», выбор locale; `Frame` — контент (`<Outlet/>`).

## 6. Информационная архитектура (роуты)

```
/                         → redirect /sites
/sites                    список сайтов, создание
/sites/:siteId            дашборд: статусы dev/prod, последний снапшот, действия
/sites/:siteId/pages              страницы + роуты
  /pages/:pageId                   ⭐ редактор страницы (§7)
/sites/:siteId/content            контент по коллекциям (strings, статьи)
/sites/:siteId/assets             библиотека ассетов
/sites/:siteId/forms              формы + сабмиты
/sites/:siteId/routes             роуты (matcher/priority/action)
/sites/:siteId/components         реестр компонентов + исходники + git-версии/rollback
/sites/:siteId/themes             темы/токены
/sites/:siteId/deps               зависимости (package.json-UI, R9)
/sites/:siteId/builds             сборки: snapshot→build→env, статусы, rollback
/sites/:siteId/settings           настройки (name/slug/hosts/locale)
```

`loaders`-навигация: `/sites` (list), `/sites/:siteId` (site + envs), вложенные — по секциям. Хранилище сайта (`/sites/:siteId/*`) держит выбранный site в slice-store ui-runtime (переключатель в sidebar).

## 7. Редактор страницы (ядро, `pages/:pageId`)

`SplitPane vertical (дерево | canvas | инспектор)`, width-холст гибкий.

```
LEFT  Дерево                 CENTER Canvas-превью          RIGHT Инспектор
  ComponentNode tree           iframe → клиентский URL        Tabs: Props | Bindings | Structure
  add/remove/move по            (/build/<site>/<env>/... или   Props: автоформа из schema
  schema allowChildren          текущий роут)                    string/number/bool/enum/asset/richtext
  клик → focus instance         авто-рефреш по DevRebuildEvent   live-валидация (R4)
                                (WS) + авто-сборка после save  Bindings: literal | source(path)
```

- **Draft:** slice-store `usePageDraftStore` (tree, `version`, dirty-флаг) через ui-runtime `createSliceStore`/`useSelector` — zustand напрямую не импортируется. Автосейв (дебаунс 1500ms) → `PUT /pages/{id}/tree`; индикатор «сохранено • версия N» / «сохранение…» в topbar.
- **Поток превью:** save → `POST /sites/{id}/builds {snapshotId или последний, environment:development}` → publish dev → iframe reload; WS `DevRebuildEvent` обновляет iframe без ручного рефреша. WYSIWYG незакоммиченных правок (design-mode над registry боотом) — stretch goal M1+.
- **Schema-форма:** генератор полей из JSON-Schema: `string` (в т.ч. format `rich-text` → Tiptap §9, `asset` → asset-picker), `number`, `boolean`, `enum`, массив. Live-валидация (R4) — обводка + список ошибок, но сохранение допустимо.

## 8. Области (вне ядра)

- **Контент (`/content`):** коллекции→items, таблица, редактор fields (по schema), переводы overlay по locale. Rich-text поля через Tiptap.
- **Ассеты (`/assets`):** список, upload (multipart), превью по mime, выбор в asset-picker.
- **Роуты (`/routes`):** CRUD, matcher regex, priority, action (renderPage/serveAsset/redirect c $n-группами) — live-валидация regex.
- **Компоненты (`/components`):** реестр (definition/schema/metadata), исходник `.tsx` (editor), git: commit+message, история `ComponentVersion`, checkout/rollback, tag.
- **Темы (`/themes`):** список Theme, редактор tokens/CSS-переменных; в v1 — управление значениями + предпросмотр.
- **Зависимости (`/deps`):** «как package.json» — добавить `pkg@^range`, показать lock (exact+sha512), integrity-cache; пересборка. Зависит от Dependency-сервиса (Фаза 1).
- **Публикация (`/builds`):** snapshot → build (dev/prod), живые статусы (queued→building→ready/failed) через WS/poll, лог из `Build.Log`, активация/rollback выбором предыдущего снапшота; защита: prod требует сознательного действия.

## 9. Tiptap rich-text «IDE»

- Триггер: schema-поле `format: "rich-text"` (или `type`-маркер) → `TiptapEditor`.
- Хранение: **HTML-строка** в `fields` (BSON/API-friendly); рантайм рендерит через санатизированный HTML.
- Extensions: `StarterKit`, `Underline`, `Link`, `Image` (набирается через asset-picker), `Placeholder`, `CharacterCount`.
- **Slash-меню** (`/`): кастомный ProseMirror Extension — ввод `/` открывает паллет в позиции курсора; стрелки+Enter, Escape — закрыть.
- Блоки: параграф, H1–H3, список, цитата, code-block, divider, image. Инлайн: bold/italic/underline/strike, link, code.
- Toolbar: контекстные кнопки + индикатор символов.

## 10. Тест-спека (контракты)

Стек: **vitest + @testing-library/react + jsdom**. Пакеты: `ui-kit`, `admin`; ui-runtime — существующие тесты + новые на экспорты (§5.2).

| Модуль | Файл тестов | Контракт |
| --- | --- | --- |
| ui-kit primitives | `ui-kit/src/primitives/**/*.test.tsx` | рендер элемента, маппинг props→className (table-driven: каждая комбинация → ожидаемый класс), `className`-мерж, default-`as`. |
| ui-runtime exports | `ui-runtime/tests/...` | переэкспорт `HttpTransport`/типов существует; descriptor совместим с `registerOperation` + `parseDescriptors`. |
| ui-runtime slice-store | `ui-runtime/tests/unit/slice-store.test.ts` | `createSliceStore`: init, `setState` (patch/updater), `useSelector` стабильность (===), slice-подписка, отписка. |
| AdminTransport | `admin/src/admin-runtime/admin-transport.test.ts` | зоookmock fetch: метод/url/body, инжект `Authorization` из getter, не-2xx → `AdminApiError` c `status`+`message` из `{"error"}`, без тела → generic. |
| admin-api | `admin/src/admin-runtime/admin-api.test.ts` | mock-fetch: `sites.list()` URL+query, `pages.updateTree` метод+body, `builds.create` sync статус `queued→ready`; ошибки пробрасываются. |
| admin i18n | `admin/src/admin-runtime/i18n.test.ts` | `I18n.t()` по словарю, detect/локализация, fallback-defaultLocale, интерполяция `{n}`. |
| token-store | `admin/src/admin-runtime/token-store.test.ts` | set/clear + localStorage. |
| AppShell | `admin/src/app/shell.test.tsx` | рендер nav-ссылок, topbar-элементов, `Outlet`. |
| Router | `admin/src/app/router.test.tsx` | createMemoryRouter: `/`→redirect, `/sites` рендер, `/:siteId` layout. |

Критерий приёмки M0: все **тесты зелёные**, `tsc --noEmit` чистый в трёх пакетах; ui-runtime не регрессирует.

## 11. План работ (admin TODO)

### M0 — Фундамент и харнес  🎯 ([x] = выполнено)

- [x] `docs/editor-spec.md` (этот файл).
- [x] ui-runtime: расширить index.ts экспортами §5.2 (+ тест).
- [x] ui-runtime: обобщённый slice-store (`createSliceStore`/`useSelector`) — zustand-обёртка + тесты. Админка не зависит от zustand.
- [x] root `package.json` (workspaces, скрипты) + `.gitignore`.
- [x] `ui-kit`: конфиги (tsconfig/vitest), примитивы §4, тесты §10 (25).
- [x] `admin`: Vite+React+TS+Tailwind v4+Router scaffold, proxy `:8080`/`:18080`, `styles.css` tailwind.
- [x] `admin-runtime`: types, operations.ts, AdminApi/request, createAdminApi, token-store, i18n/strings (+ тесты, 20).
- [x] AppShell + роутер + admin-context + заглушки секций (+ тесты).
- [x] Всё зелёное: `npm test` (ui-runtime 227 / ui-kit 25 / admin 20), `tsc --noEmit` во всех пакетах.

### M1 — Ядро редактора

- [x] Список страниц и роутов; crud-страниц. *(Слайс 1/3: Sites/SiteHome/SitePages/SiteRoutes, use-operation, ConfirmButton убрал window.confirm, EntityTable/Field, ops listPages/deletePage, страницы покрыты 13 тестами — админка 33/33.)*
- [ ] Редактор дерева: рендер, add/remove/move, focus, undo/redo (slice-store + история).
- [ ] Инспектор: генератор формы из JSON-Schema, live-валидация (R4), asset-picker, binds-переключатель literal/source.
- [ ] Bindings: picker источника (content/route/query/operation/form + path).
- [ ] Draft-стор + автосейв (дебаунс 1500) + индикатор версии.
- [ ] Canvas-превью: iframe, авто-сборка dev после save, WS DevRebuildEvent → reload. (stretch: design-mode draft.)
- [ ] Тесты: page-store, schema-form, debounce-save, bindings-picker, preview-reload.

### M2 — Контент/ассеты/формы/роуты + Tiptap

- [ ] Контент: коллекции, items, translations overlay.
- [ ] Ассеты: upload/list/preview/picker.
- [ ] Формы: definition + submissions.
- [ ] Роуты: CRUD + валидация matcher/action.
- [ ] Tiptap: `@tiptap/*`, slash-меню, asset-Image, HTML round-trip.
- [ ] Тесты соответствующих менеджеров и Tiptap-утилит.

### M3 — Публикация, git, темы, зависимости

- [ ] Builds: snapshot→build→env, живые статусы, лог, rollback; publish-защита prod.
- [ ] Git-UI: commit/push/tag, история версий компонентов, checkout/rollback.
- [ ] Темы/токены: editor токенов + предпросмотр (зависит от `/runtime/tokens`).
- [ ] Зависимости: package.json-UI (зависит от Dependency-сервиса Фаза 1).
- [ ] Dashboard сайта, E2E (vitest browser / Playwright) по сценарию «создать сайт → страницу → опубликовать».

### M4 — Полировка

- [ ] Аутентификация: экран входа с токеном, `401`-обработка.
- [ ] Keyboard-first: паллет команд (Cmd+K), горячие клавиши.
- [ ] Доступность (WCAG): w n-tests, фокус-трапы модалок.
- [ ] Мета-инфра: CI (build+test+typecheck), экспорт токенов ui-kit в tailwind config.

## 12. Backend-зависимости и риски

Редактор **фронтенд-only** (E10). Открытые зависимости — в статус не включаются, на блокируют M0–M1:

- `/runtime/tree|routes|tokens` (Этап 4, часть 2) — нужны Canvas-превью в «дизайн-режиме» и тем-предпросмотру. v1-превью (iframes dev-билд) не зависит от них.
- Dependency-сервис Фаза 1 — `/deps` (M3).
- `ComponentRegistry` в entry-шаблоне (упомянуто в Этапе 3 §8) — `admin-runtime` пока не зависит, бандл-контракт уточнится в M1.
- Content batch (`POST /runtime/*` пусто) — переводы пишутся per-locale; батч не требуется.

## 13. Ссылки

- `docs/api-admin.md` — REST-контракт (все CRUD, builds).
- `docs/api-client.md` — runtime-contract (`/runtime/contract`), boot.
- `docs/ui-runtime/spec.md`, `test-spec.md` — рантайм-ядро.
- `docs/backend/build-test-spec.md` — сборка (builds/rebulider).
- `README.md::Этап 6` — цели редактора.