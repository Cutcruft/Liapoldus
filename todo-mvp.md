# Liapoldus — TODO до MVP

Цель: довести проект до рабочего MVP с полной функциональностью.
`make dev` запускает всё.

**Текущий статус:** M0–M2 + M3-slice 1 (Builds), слайсы 5 (Docker), 1 (Git), 2 (Auth+Dashboard+Settings), 3 (Token Editor), 4 (Dependencies UI) — выполнены.
Осталось: слайс 6 (Cleanup).

**Порядок слайсов:** 5 (Docker) → 1 (Git) → 2 (Auth) → 3 (Tokens) → 4 (Deps) → 6 (Cleanup)

### Финальные решения

| Решение | Детали |
|---------|--------|
| Git repo | Пересоздать bare repo: `dev`/`main` хранят **всё состояние**. Старые `defs/*` ветки — deprecated |
| Token preview | `srcdoc` (inline HTML в iframe) |
| Token defaults | Пустые поля (пользователь заполняет сам) |
| Publish modal | Preview diff modal: «+N pages, +M routes, -K forms» → подтверждение |
| Dashboard empty | Заглушка «Создайте первый сайт» + CTA кнопка |

---

## Слайс 1: Git + Snapshot Unification

**Цель**: снапшот = git commit полного состояния сайта. Publish = rebase dev→main + merge.
Rollback = current main → dev, snapshot → main. Restore = safety-snapshot + overwrite.

### 1.1 Бэкенд: пересоздать git repository

**Решение:** Пересоздать bare repo. `dev`/`main` хранят **всё состояние сайта** (pages, contents, routes, forms, components, deps). Старые `defs/*` ветки — deprecated, не используем.

**Файл:** `backend/internal/application/git/repository.go`

Новый интерфейс (заменяет существующий):

```go
type Repository interface {
    // InitRepo — init bare repo + создание ветки main с пустым коммитом
    InitRepo(ctx context.Context, siteID string) error

    // CommitOnBranch — коммит на指定енной ветке (с parent = HEAD ветки)
    CommitOnBranch(ctx context.Context, siteID, branch, message string, files map[string][]byte) (string, error)

    // CheckoutBranch — checkout ветки, возвращает SHA HEAD
    CheckoutBranch(ctx context.Context, siteID, branch string) (string, error)

    // HeadSHA — SHA HEAD指定енной ветки
    HeadSHA(ctx context.Context, siteID, branch string) (string, error)

    // Rebase — rebase sourceBranch на targetBranch
    Rebase(ctx context.Context, siteID, sourceBranch, targetBranch string) error

    // Merge — fast-forward merge sourceBranch в targetBranch, возвращает SHA
    Merge(ctx context.Context, siteID, sourceBranch, targetBranch string) (string, error)

    // ListBranches — список веток
    ListBranches(ctx context.Context, siteID string) ([]string, error)

    // Commits — история коммитов指定енной ветки (новые первые, limit)
    Commits(ctx context.Context, siteID, branch string, limit int) ([]CommitInfo, error)
}

type CommitInfo struct {
    SHA     string
    Message string
    Author  string
    Time    time.Time
}
```

**Удалить мёртвый `Head`** из `backend/internal/infra/git/repo.go` (не вызывается нигде).

### 1.2 Бэкенд: реализовать новые методы в go-git

**Файл:** `backend/internal/infra/git/repo.go`

Реализовать:
- `InitWithMain` — `PlainInit(bare)` → worktree checkout `main` → пустой коммит → push ref `refs/heads/main`
- `CommitOnBranch` — открыть bare repo → storer → записать blobs/tree → parent = `refs/heads/{branch}` → коммит → update ref
- `CheckoutBranch` — `worktree.Add` → `checkout.Options{Branch: refname}` → SHA HEAD
- `HeadSHA` — `storer.Reference("refs/heads/{branch}")` → hash
- `Rebase` — открыть repo → `git.Checkout` на source → `git.Rebase` onto target
- `Merge` — fast-forward: `repo.Merge(&git.MergeOptions{FastForwardOnly: true})`
- `ListBranches` — итерация refs `refs/heads/*`
- `Commits` — `object.NewCommitPreorderIter` от HEAD指定енной ветки

**Тест-файлы:**
- `backend/tests/unit/git_repo_branch_test.go`

### 1.3 Бэкенд: gitsnapshot service

**Новые файлы:**
- `backend/internal/application/gitsnapshot/service.go`
- `backend/internal/application/gitsnapshot/serialize.go`
- `backend/internal/application/gitsnapshot/restore.go`

**Service:**

```go
type Service struct {
    repo        git.Repository
    sites       domain.SiteRepository
    pages       domain.PageRepository
    contents    domain.ContentRepository
    routes      domain.RouteRepository
    forms       domain.FormRepository
    components  domain.ComponentDefinitionRepository
    snapshots   domain.SnapshotRepository
    deps        LockResolver // из snapshot
}

// Commit — сериализация текущего состояния сайта → git commit на dev.
func (s *Service) Commit(ctx context.Context, siteID, message string) (string, error)

// Restore — safety-snapshot + checkout SHA → загрузка в БД.
func (s *Service) Restore(ctx context.Context, siteID, sha string) error

// Publish — rebase dev на main + merge dev→main + загрузка в БД.
func (s *Service) Publish(ctx context.Context, siteID, message string) (string, error)

// Rollback — current main → dev (commit), snapshot SHA → main (merge).
func (s *Service) Rollback(ctx context.Context, siteID, snapshotSHA, message string) (string, error)

// Commits — история коммитов dev.
func (s *Service) Commits(ctx context.Context, siteID string, limit int) ([]git.CommitInfo, error)

// Status — dev SHA, main SHA, clean/dirty.
func (s *Service) Status(ctx context.Context, siteID string) (*SiteStatus, error)
```

**serialize.go — формат файлов в git:**
```
repo root:
  site.json                    — {name, slug, defaultLocale, hosts}
  pages/{pageId}.json          — {tree, version, updatedAt}
  contents/{contentId}.json    — {collectionId, fields, translations}
  routes/{routeId}.json        — {matcher, priority, action}
  forms/{formId}.json          — {id, definition}
  components/{componentId}/
    source.tsx                 — исходный код компонента
    schema.json                — JSON Schema
    metadata.json              — метаданные
  deps-lock.json               — {lockedDeps: [...]}
```

**serialize.go — алгоритм:**
1. Загрузить site из БД → `site.json`
2. `pages.ListPagesBySite(siteID)` → для каждой: `ListPageVersions(pageID)` → дерево последней версии → `pages/{pageId}.json`
3. `contents.ListContentsBySite(siteID)` → `contents/{contentId}.json`
4. `routes.ListRoutesBySite(siteID)` → `routes/{routeId}.json`
5. `forms.ListFormsBySite(siteID)` → `forms/{formId}.json`
6. `components.ListBySite(siteID)` → для каждой: `components/{id}/source.tsx`, `schema.json`, `metadata.json`
7. `deps.ResolveLock(siteID)` → `deps-lock.json`
8. `repo.CommitOnBranch(siteID, "dev", message, files)` → SHA

**restore.go — алгоритм Restore:**
1. Safety-snapshot: `Commit(ctx, siteID, "restore safety: before restore to {sha}")`
2. `repo.Checkout(siteID, sha)` → `map[string][]byte` files
3. Парсим `site.json` → `sites.UpdateSite(siteID, parsed)`
4. Парсим `pages/*.json` → `pages.UpdateTree(pageID, tree)` для каждой (или Create если нет)
5. Парсим `contents/*.json` → upsert content + translations
6. Парсим `routes/*.json` → upsert routes
7. Парсим `forms/*.json` → upsert forms
8. Парсим `components/*/` → upsert component definitions
9. Парсим `deps-lock.json` → сохранить в `snapshot_deps_locks`

**restore.go — алгоритм Publish:**
1. `repo.HeadSHA(siteID, "dev")` и `repo.HeadSHA(siteID, "main")`
2. `repo.Rebase(siteID, "dev", "main")` — rebase dev на main
3. `repo.Merge(siteID, "dev", "main")` → SHA main
4. Загрузить файлы из main в БД (как Restore шаги 3-9)
5. Создать Build (publish → build pipeline)

**restore.go — алгоритм Rollback:**
1. `repo.HeadSHA(siteID, "main")` → текущий main SHA
2. Checkout текущего main → сериализовать → `CommitOnBranch(siteID, "dev", "rollback: current prod", files)`
3. Checkout指定 snapshot SHA → сериализовать → `CommitOnBranch(siteID, "main", "rollback to {sha}", files)`
4. Загрузить файлы из main в БД

### 1.4 Бэкенд: model updates

**Файл:** `backend/internal/domain/model.go`

```go
// Snapshot — добавить
type Snapshot struct {
    // ... существующие поля ...
    GitSHA *string `json:"gitSha,omitempty"`
}

// Site — добавить
type Site struct {
    // ... существующие поля ...
    DefaultBranch string `json:"defaultBranch"` // "main" по умолчанию
}
```

### 1.5 Бэкенд: admin API

**Новые файлы:**
- `backend/internal/api/admin/git_handler.go`

**Роуты (добавить в `server.go`):**
```go
protected.HandleFunc("POST /api/sites/{siteID}/git/commit", gitHandler.Commit)
protected.HandleFunc("POST /api/sites/{siteID}/git/restore", gitHandler.Restore)
protected.HandleFunc("POST /api/sites/{siteID}/git/publish", gitHandler.Publish)
protected.HandleFunc("POST /api/sites/{siteID}/git/rollback", gitHandler.Rollback)
protected.HandleFunc("GET /api/sites/{siteID}/git/commits", gitHandler.Commits)
protected.HandleFunc("GET /api/sites/{siteID}/git/status", gitHandler.Status)
```

**Обновить `server.go`:** App struct — добавить `GitSnapshot *gitsnapshot.Service`, wire handler.

### 1.6 Бэкенд: wiring

**Файлы:**
- `backend/internal/application/application.go` — wire `gitsnapshot.NewService(repo, sites, pages, contents, routes, forms, components, snapshots, deps)`
- `backend/cmd/server/main.go` — wire в admin.App

### 1.7 Бэкенд: тесты

**Новые файлы:**
- `backend/tests/unit/gitsnapshot_test.go`
- `backend/tests/unit/gitsnapshot_serialize_test.go`

**Тест-кейсы:**
- Serialize: полный сайт → JSON файлы, формат файлов корректен
- Commit: serialize → commit на dev → SHA не пустой
- Restore: commit → safety-snapshot → overwrite → данные совпадают
- Publish: dev commits → rebase → merge → main SHA изменился
- Rollback: current main → dev, snapshot → main
- Status: dev SHA, main SHA, clean/dirty
- Fast-forward only: если main ahead dev → publish ошибка

### 1.8 Фронтенд: operations + types + strings

**Файлы:**
- `admin/src/runtime/operations.ts` — добавить OperationKind:
  ```
  | 'commitSnapshot' | 'restoreSnapshot' | 'publishSite'
  | 'rollbackSite' | 'listCommits' | 'gitStatus'
  ```
- `admin/src/runtime/types.ts` — добавить `CommitInfo`, `SiteStatus`
- `admin/src/runtime/strings.ts` — добавить `git.*`, `op.commitSnapshot`
- `admin/src/runtime/index.ts` — export новых типов

### 1.9 Фронтенд: Git страница

**Новые файлы:**
- `admin/src/app/pages/SiteCommitsPage.tsx`
- `admin/src/app/pages/SiteCommitsPage.test.tsx`

**Обновить:**
- `admin/src/app/pages/SiteBuildsPage.tsx` — добавить таб «Git» (статус веток, коммиты, publish)
- `admin/src/app/pages/SiteHomePage.tsx` — nav: «Git» (после «Публикация»)
- `admin/src/app/AppRoutes.tsx` — роут `/sites/:siteId/git`

**SiteCommitsPage:**
- Таблица коммитов: SHA (short, 8 символов), message, author, date, badge (dev/main)
- Кнопка «Опубликовать на main» → **modal**: preview diff («+N pages, +M routes, -K forms» между dev и main) + commit message input → `POST /api/sites/{id}/git/publish`
- Кнопка «Восстановить из этого» → `POST /api/sites/{id}/git/restore` с confirm dialog + SHA
- Кнопка «Откатить на main» → `POST /api/sites/{id}/git/rollback` с confirm dialog + SHA
- Poll для обновления (каждые 5 сек или по кнопке)

**SiteBuildsPage — Git таб:**
- Статус: dev SHA (short), main SHA (short), clean/dirty badge
- Кнопка «Создать коммит» → modal с вводом commit message → `POST /api/sites/{id}/git/commit`

---

## Слайс 2: Auth + Dashboard + Settings

### 2.1 Бэкенд: auth

**Новые файлы:**
- `backend/internal/api/admin/auth_handler.go`

```go
// POST /api/auth/validate — проверка токена
// Request: {token: string}
// Response: 200 {valid: true} или 401 {error: "unauthorized"}
```

**Роут:** монтировать на внешнем mux до BearerAuth (как `/api/builds/ws`).

### 2.2 Бэкенд: dashboard

**Новые файлы:**
- `backend/internal/api/admin/dashboard_handler.go`

```go
// GET /api/dashboard
// Response: {
//   siteCount: int,
//   recentBuilds: Build[],
//   recentSnapshots: Snapshot[],
//   runtimeStatus: string
// }
```

### 2.3 Бэкенд: settings

**Новые файлы:**
- `backend/internal/api/admin/settings_handler.go`

```go
// GET /api/settings
// Response: {adminToken: string(masked), defaultLocale, redirectDefaultStatus}
```

### 2.4 Фронтенд: login

**Новые файлы:**
- `admin/src/app/pages/LoginPage.tsx`
- `admin/src/app/pages/LoginPage.test.tsx`

**Обновить:**
- `admin/src/app/AppRoutes.tsx` — роут `/login`, guard (redirect если нет токена)
- `admin/src/runtime/token-store.ts` — добавить `validateToken` (POST /api/auth/validate)

**LoginPage:**
- Поле ввода токена → `POST /api/auth/validate` → localStorage → redirect на `/`
- Если токен пуст → показать «Введите токен»
- Если невалидный → показать ошибку
- Submit на Enter

### 2.5 Фронтенд: dashboard

**Новые файлы:**
- `admin/src/app/pages/DashboardPage.tsx`
- `admin/src/app/pages/DashboardPage.test.tsx`

**Обновить:**
- `admin/src/app/AppRoutes.tsx` — заменить Placeholder на DashboardPage
- `admin/src/app/pages/Placeholder.tsx` — **удалить** (заменяется)

**DashboardPage:**
- Счётчики: количество сайтов
- Quick actions: «Создать сайт» (redirect на SitesPage), «Выбрать сайт» (select dropdown)
- Последние builds: таблица (5 штук): site, status, date
- Последние snapshots: таблица (5 штук): site, name, date
- **Empty state:** если сайтов нет → заглушка «Создайте первый сайт» + CTA кнопка (redirect на SitesPage)

### 2.6 Фронтенд: settings

**Новые файлы:**
- `admin/src/app/pages/SettingsPage.tsx`
- `admin/src/app/pages/SettingsPage.test.tsx`

**Обновить:**
- `admin/src/app/AppRoutes.tsx` — заменить Placeholder на SettingsPage

**SettingsPage:**
- Admin token: текущий (masked), кнопка «Обновить» (ввод нового → validate → save)
- Default locale: select (ru/en/de/fr)
- Redirect default status: select (301/302/307/308)
- Version приложения: APP_VERSION

### 2.7 Фронтенд: site edit

**Обновить:**
- `admin/src/app/pages/SiteHomePage.tsx` — добавить кнопку «Редактировать» → inline форма edit
- `admin/src/runtime/operations.ts` — op: `updateSite`

**SiteHomePage edit:**
- Name, slug (read-only), locale (select), hosts (list add/remove)
- Save → `PUT /api/sites/{id}` → reload

### 2.8 Фронтенд: logout

**Обновить:**
- `admin/src/app/AppShell.tsx` — кнопка «Выйти» в footer → `localStorage.removeItem` → redirect `/login`

### 2.9 Фронтенд: constants

**Новые файлы:**
- `admin/src/runtime/constants.ts`

```ts
export const APP_VERSION = '0.1.0 (MVP)';
export const ENV_DEV = 'development';
export const DEFAULT_DEV_WS_PATH = '/dev/build/ws';
export const DEFAULT_BUILDS_WS_PATH = '/api/builds/ws';
```

---

## Слайс 3: Token Editor

### 3.1 Бэкенд: domain model

**Файл:** `backend/internal/domain/model.go`

```go
type TokenSet struct {
    Colors      map[string]ModeValues  `json:"colors"`
    Typography  TypographyTokens       `json:"typography"`
    Spacing     map[string]string      `json:"spacing"`
    Shadows     map[string]string      `json:"shadows"`
    Borders     BorderTokens           `json:"borders"`
    Breakpoints map[string]string      `json:"breakpoints"`
    ZIndex      map[string]int         `json:"zIndex"`
    Opacity     map[string]float64     `json:"opacity"`
    Transitions map[string]string      `json:"transitions"`
    Custom      map[string]interface{} `json:"custom"`
}

type ModeValues struct {
    Light string `json:"light"`
    Dark  string `json:"dark"`
}

type TypographyTokens struct {
    FontFamily map[string]string      `json:"fontFamily"`
    FontSize   map[string]string      `json:"fontSize"`
    FontWeight map[string]int         `json:"fontWeight"`
    LineHeight map[string]string      `json:"lineHeight"`
}

type BorderTokens struct {
    Radius map[string]string `json:"radius"`
    Width  map[string]string `json:"width"`
}
```

### 3.2 Бэкенд: token service

**Новые файлы:**
- `backend/internal/application/token/service.go`
- `backend/internal/application/token/repository.go`

```go
type Service struct {
    repo TokenRepository
}

func (s *Service) Get(ctx context.Context, siteID string) (*domain.TokenSet, error)
func (s *Service) Update(ctx context.Context, siteID string, tokens *domain.TokenSet) error
```

**Repository interface:**
```go
type TokenRepository interface {
    GetTokens(ctx context.Context, siteID string) (*domain.TokenSet, error)
    UpsertTokens(ctx context.Context, siteID string, tokens *domain.TokenSet) error
}
```

### 3.3 Бэкенд: persistence

**Memory:** `backend/internal/infra/storage/memory.go` — добавить `map[string]*domain.TokenSet` + impl

**Postgres:** `backend/internal/infra/db/postgres.go` — миграция:
```sql
CREATE TABLE IF NOT EXISTS site_tokens (
    site_id TEXT PRIMARY KEY,
    tokens JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 3.4 Бэкенд: admin API

**Новые файлы:**
- `backend/internal/api/admin/token_handler.go`

```go
GET  /api/sites/{siteID}/tokens  — получить токены
PUT  /api/sites/{siteID}/tokens  — обновить токены (body: TokenSet JSON)
```

### 3.5 Бэкенд: wiring + тесты

- `application.go` — wire `token.NewService(repo)`
- `main.go` — wire в admin.App
- `backend/tests/unit/token_test.go`
- `backend/tests/integration/token_test.go`

### 3.6 Фронтенд: token editor page

**Новые файлы:**
- `admin/src/app/pages/SiteTokensPage.tsx`
- `admin/src/app/pages/SiteTokensPage.test.tsx`
- `admin/src/app/components/ColorPicker.tsx`
- `admin/src/app/components/ColorPicker.test.tsx`
- `admin/src/app/components/TokenPreview.tsx`

**Обновить:**
- `admin/src/app/pages/SiteHomePage.tsx` — nav: «Токены»
- `admin/src/app/AppRoutes.tsx` — роут `/sites/:siteId/tokens`
- `admin/src/runtime/operations.ts` — ops: `getTokens`, `updateTokens`
- `admin/src/runtime/types.ts` — `TokenSet` type
- `admin/src/runtime/strings.ts` — `nav.tokens`, `tokens.*`, `categories`

**SiteTokensPage:**
- Sidebar: категории (Colors, Typography, Spacing, Shadows, Borders, Breakpoints, Z-index, Opacity, Transitions, Custom)
- Content area: редактор выбранной категории
- Preview iframe (token-preview.html): live обновление при изменении токенов
- Save button + debounced autosave (500ms)
- Default values: **пустые поля** — пользователь заполняет всё сам (без предустановленных дефолтов)

**ColorPicker:**
- HSV color picker с polygon ring
- Hex input (#RRGGBB)
- RGB sliders (R, G, B 0-255)
- Light/Dark mode toggle (для ввода значений в обе темы)

**TokenPreview (iframe srcdoc):**
- Inline HTML в `<iframe srcdoc="...">` (без сервера)
- Полноценная страница-пример с CSS custom properties из token values:
  - Typography specimen: headings H1-H6, body text, links, code block
  - Color swatch grid: primary, secondary, background, text, muted, accent, success, warning, error
  - Spacing scale: визуальная шкала от xs до 2xl
  - Border radius examples: button, card, full
  - Shadow examples: sm, md, lg
  - Breakpoints: responsive preview в разных размерах
- Preview обновляется live (postMessage при изменении токенов)

### 3.7 Фронтенд: strings

```ts
nav: { tokens: 'Токены' },
tokens: {
  title: 'Дизайн-токены',
  save: 'Сохранить',
  saved: 'Сохранено',
  categories: {
    colors: 'Цвета',
    typography: 'Типографика',
    spacing: 'Отступы',
    shadows: 'Тени',
    borders: 'Границы',
    breakpoints: 'Точки перелома',
    zIndex: 'Z-index',
    opacity: 'Прозрачность',
    transitions: 'Анимации',
    custom: 'Пользовательские',
  },
  colors: {
    addColor: 'Добавить цвет',
    primary: 'Основной',
    secondary: 'Вторичный',
    background: 'Фон',
    text: 'Текст',
    muted: 'Приглушённый',
    accent: 'Акцент',
    success: 'Успех',
    warning: 'Предупреждение',
    error: 'Ошибка',
  },
  typography: {
    fontFamily: 'Шрифт',
    fontSize: 'Размер шрифта',
    fontWeight: 'Толщина шрифта',
    lineHeight: 'Высота строки',
  },
}
```

---

## Слайс 4: Dependencies UI

### 4.1 Бэкенд

Все CRUD операции уже реализованы:
- `GET/POST /api/sites/{id}/dependencies`
- `DELETE /api/sites/{id}/dependencies/{name}`
- `GET/POST /api/sites/{id}/dependencies/allowlist`
- `DELETE /api/sites/{id}/dependencies/allowlist?entry={pattern}` (query — path не матчит `*`/`/`)
- `GET/PUT /api/sites/{id}/cache-config`
- `POST /api/sites/{id}/cache-config/evict`

Добавлено:
- `POST /api/sites/{id}/dependencies/resolve` — принудительный resolve (body не используется)
  Возвращает плоский `SnapshotLock` (BFS-граф c `requestedBy`/`hoisted`/`peerDependencies`); дерево строит фронт (корни = `requestedBy: ["site"]`)

**Файл:** `backend/internal/api/admin/deps_handler.go` — добавить `Resolve` handler

### 4.2 Фронтенд: dependencies page

**Новые файлы:**
- `admin/src/app/pages/SiteDepsPage.tsx`
- `admin/src/app/pages/SiteDepsPage.test.tsx`

**Обновить:**
- `admin/src/app/pages/SiteHomePage.tsx` — nav: «Зависимости»
- `admin/src/app/AppRoutes.tsx` — роут `/sites/:siteId/deps`
- `admin/src/runtime/operations.ts` — ops: `addDependency`, `removeDependency`, `resolveDependencies`, `listAllowlist`, `addAllowlist`, `removeAllowlist`, `getCacheConfig`, `updateCacheConfig`, `evictCache`

**SiteDepsPage:**
- Список зависимостей: name + version range (из deps宣言)
- Кнопка «Добавить依赖» (modal: name + version range input)
- Кнопка «Удалить» рядом с каждой (confirm dialog)
- Кнопка «Resolve» → `POST /api/sites/{id}/dependencies/resolve` → дерево
- Иерархическое дерево (expand/collapse):
  - Root level: объявленные зависимости (name@version)
  - Children: транзитивные зависимости
  - Peer deps: highlighted badge
  - Hoisted: badge
  - Nested: badge с parent name
- Cache config: текущий лимит, кнопка «Очистить кэш» (evict)

### 4.3 Фронтенд: allowlist page

**Новые файлы:**
- `admin/src/app/pages/SiteAllowlistPage.tsx`
- `admin/src/app/pages/SiteAllowlistPage.test.tsx`

**Обновить:**
- `admin/src/app/pages/SiteHomePage.tsx` — nav: «Allowlist»
- `admin/src/app/AppRoutes.tsx` — роут `/sites/:siteId/allowlist`
- `admin/src/runtime/operations.ts` — ops: `addAllowlist`, `removeAllowlist`

**SiteAllowlistPage:**
- Список записей: name pattern (exact, @scope/*, *)
- Кнопка «Добавить» (modal: name input с валидацией)
- Кнопка «Удалить» рядом с каждой (confirm)
- Подсказка: «Пустой allowlist = разрешено всё»

---

## Слайс 5: Docker + Dev Environment

### 5.1 docker-compose.yml (корень проекта)

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: liapoldus
      POSTGRES_USER: liapoldus
      POSTGRES_PASSWORD: liapoldus
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U liapoldus"]
      interval: 5s
      timeout: 5s
      retries: 5

  backend:
    build:
      context: ./backend
      dockerfile: Dockerfile
    ports:
      - "8080:8080"
      - "18080:18080"
    environment:
      LIAPOLDUS_ADMIN_ADDR: ":8080"
      LIAPOLDUS_CLIENT_ADDR: ":18080"
      LIAPOLDUS_STORAGE: "postgres"
      LIAPOLDUS_DATABASE_URL: "postgres://liapoldus:liapoldus@postgres:5432/liapoldus?sslmode=disable"
      LIAPOLDUS_ASSET_DIR: "/data/assets"
      LIAPOLDUS_GIT_DIR: "/data/git"
      LIAPOLDUS_BUILD_DIR: "/data/build"
      LIAPOLDUS_DEPS_DIR: "/data/deps"
      LIAPOLDUS_DEFAULT_LOCALE: "ru"
      LIAPOLDUS_REDIRECT_DEFAULT_STATUS: "302"
      LIAPOLDUS_REDIRECT_ALLOWED_STATUSES: "301,302,307,308"
      LIAPOLDUS_COMPONENT_MAX_DEPTH: "10"
      LIAPOLDUS_PAGE_INITIAL_VERSION: "1"
      LIAPOLDUS_EMAIL_PATTERN: "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"
      LIAPOLDUS_MASTER_VARIANT_NAME: "original"
      LIAPOLDUS_ASSET_FALLBACK_NAME: "asset"
      LIAPOLDUS_ASSET_FALLBACK_MIME: "application/octet-stream"
      LIAPOLDUS_ASSET_FILE_URL_TEMPLATE: "/api/assets/{id}/file"
      LIAPOLDUS_ASSET_CACHE_MAX_AGE_SECONDS: "86400"
      LIAPOLDUS_MAX_UPLOAD_BYTES: "10485760"
      LIAPOLDUS_STARTUP_TIMEOUT: "10s"
      LIAPOLDUS_SHUTDOWN_TIMEOUT: "5s"
      LIAPOLDUS_READ_HEADER_TIMEOUT: "10s"
      LIAPOLDUS_NPM_REGISTRY: "https://registry.npmjs.org"
    volumes:
      - ./data:/data
    depends_on:
      postgres:
        condition: service_healthy

  frontend:
    build:
      context: ./admin
      dockerfile: Dockerfile.dev
    ports:
      - "5173:5173"
    environment:
      VITE_API_URL: "http://localhost:8080"
    volumes:
      - ./admin:/app
      - /app/node_modules
    depends_on:
      - backend

volumes:
  pgdata:
```

### 5.2 admin/Dockerfile.dev

```dockerfile
FROM node:22-alpine
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
CMD ["npm", "run", "dev"]
```

### 5.3 air config (Go hot-reload)

**Файл:** `backend/.air.toml`

```toml
root = "."
tmp_dir = "tmp"

[build]
  cmd = "go build -o ./tmp/main ./cmd/server"
  bin = "./tmp/main"
  delay = 1000
  exclude_dir = ["tmp", "vendor", "tests"]
  exclude_regex = ["_test.go"]
  include_ext = ["go"]
  kill_delay = "5s"
  send_interrupt = false

[color]
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"

[log]
  time = false
```

### 5.4 Makefile (корень проекта)

```makefile
.PHONY: build dev test lint clean docker-up docker-down

# Build all
build:
	cd admin && npm install && npm run build
	cd backend && go build -o ./bin/liapoldus ./cmd/server

# Dev mode with Docker
dev:
	docker-compose up -d postgres
	cd backend && air -c .air.toml &
	cd admin && npm run dev

# Dev mode (native — no Docker)
dev-native:
	go run ./backend/cmd/server &
	cd admin && npm run dev

# Test all
test:
	cd backend && go test ./...
	cd admin && npm test -- --run

# Lint
lint:
	cd backend && go vet ./...
	cd admin && npx tsc --noEmit
	cd admin && npx eslint src/

# Clean
clean:
	rm -rf backend/tmp backend/bin admin/dist data/
	docker-compose down -v

# Docker full start
docker-up:
	docker-compose up -d

docker-down:
	docker-compose down -v
```

### 5.5 admin/vite.config.ts — env-based proxy

```ts
// Заменить хардкод proxy targets на env vars
const API_TARGET = import.meta.env.VITE_API_URL || 'http://localhost:8080';
const CLIENT_TARGET = import.meta.env.VITE_CLIENT_URL || 'http://localhost:18080';
```

### 5.6 .env.example files

**backend/.env.example:**
```
LIAPOLDUS_ADMIN_ADDR=:8080
LIAPOLDUS_CLIENT_ADDR=:18080
LIAPOLDUS_STORAGE=postgres
LIAPOLDUS_DATABASE_URL=postgres://liapoldus:liapoldus@localhost:5432/liapoldus?sslmode=disable
LIAPOLDUS_ASSET_DIR=./data/assets
LIAPOLDUS_GIT_DIR=./data/git
LIAPOLDUS_BUILD_DIR=./data/build
LIAPOLDUS_DEPS_DIR=./data/deps
LIAPOLDUS_DEFAULT_LOCALE=ru
LIAPOLDUS_REDIRECT_DEFAULT_STATUS=302
LIAPOLDUS_REDIRECT_ALLOWED_STATUSES=301,302,307,308
LIAPOLDUS_COMPONENT_MAX_DEPTH=10
LIAPOLDUS_PAGE_INITIAL_VERSION=1
LIAPOLDUS_EMAIL_PATTERN=^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$
LIAPOLDUS_MASTER_VARIANT_NAME=original
LIAPOLDUS_ASSET_FALLBACK_NAME=asset
LIAPOLDUS_ASSET_FALLBACK_MIME=application/octet-stream
LIAPOLDUS_ASSET_FILE_URL_TEMPLATE=/api/assets/{id}/file
LIAPOLDUS_ASSET_CACHE_MAX_AGE_SECONDS=86400
LIAPOLDUS_MAX_UPLOAD_BYTES=10485760
LIAPOLDUS_STARTUP_TIMEOUT=10s
LIAPOLDUS_SHUTDOWN_TIMEOUT=5s
LIAPOLDUS_READ_HEADER_TIMEOUT=10s
LIAPOLDUS_NPM_REGISTRY=https://registry.npmjs.org
```

**admin/.env.example:**
```
VITE_API_URL=http://localhost:8080
VITE_CLIENT_URL=http://localhost:18080
```

---

## Слайс 6: Doc + Code Cleanup

### 6.1 Удаление мёртвого кода

| Файл | Действие |
|------|---------|
| `backend/internal/infra/git/repo.go` | Удалить метод `Head` (строки 221-255) |
| `admin/src/components/ui/label.tsx` | Удалить файл |
| `admin/src/app/Placeholder.tsx` | Удалить файл |
| `admin/src/app/editor/dev-ws.ts` | Объединить с `build-ws.ts` → общий `app/ws-client.ts` |

### 6.2 Извлечение хардкода в конфиги

| Файл | Что извлекаем |
|------|--------------|
| `admin/src/app/AppShell.tsx` | Version `'v0.1 (M0)'` → `APP_VERSION` из `runtime/constants.ts` |
| `admin/src/app/editor/schemas.ts` | Builtin components → загрузка из backend API (`GET /api/sites/{id}/components`) |
| `admin/src/app/editor/dev-ws.ts` | WS path → `DEFAULT_DEV_WS_PATH` из constants |
| `admin/src/app/builds/build-ws.ts` | WS path → `DEFAULT_BUILDS_WS_PATH` из constants |
| `admin/src/app/editor/preview-store.ts` | `ENV_DEV` → `ENV_DEV` из constants |
| `admin/src/app/editor/EditorPage.tsx` | Debounce `500` → `AUTOSAVE_DEBOUNCE_MS` из constants |
| `backend/internal/config/config.go` | Исправить комментарий "no code defaults" → задокументировать реальные дефолты |
| `admin/vite.config.ts` | Proxy targets → env vars (сделано в слайсе 5) |

### 6.3 Новые файлы

- `admin/src/runtime/constants.ts` — `APP_VERSION`, `ENV_DEV`, `DEFAULT_DEV_WS_PATH`, `DEFAULT_BUILDS_WS_PATH`, `AUTOSAVE_DEBOUNCE_MS`

### 6.4 Объединение WS клиентов

- `admin/src/app/ws-client.ts` — общий WS клиент (объединение `editor/dev-ws.ts` и `builds/build-ws.ts`)
- Удалить `editor/dev-ws.ts`
- Обновить `editor/preview-store.ts` и `builds/build-event.ts` для использования нового клиента

### 6.5 Обновление документации

| Файл | Действие |
|------|---------|
| `TODO.md` | Удалить выполненные пункты (M0-M2 + M3-slice 1), оставить только актуальное |
| `docs/editor-spec.md` | Добавить M3 секции (Git, Tokens, Deps, Auth), обновить M0-M2 |
| `docs/api-admin.md` | Добавить git/, tokens/, deps (resolve), auth секции |
| `docs/api-client.md` | Проверить актуальность |
| `docs/feature-status.md` | Актуализировать все строки, добавить M3 |
| `docs/backend/dependency-service.md` | Исправить §4 (stale про dep_blobs), обновить статус |
| `docs/shadcn-migration.md` | Обновить: U0-U1 done, U2-U5 pending. Или удалить если не планируется |
| `README.md` | Обновить: quick start с `make dev` и `docker-compose` |

### 6.6 Config consistency

- `backend/internal/config/config.go` — добавить комментарии к дефолтным значениям
- `backend/Dockerfile` — добавить HEALTHCHECK, expose оба порта (admin + client)
- `backend/docker-compose.yml` (существующий) — удалить (заменяется корневым)
- `admin/.env.example` + `backend/.env.example` — создать (сделано в слайсе 5)

---

## Порядок выполнения слайсов

```
1. Git + Snapshot Unification  ✓ ВЫПОЛНЕНО
   пересоздание bare repo (dev/main), gitsnapshot service,
   serialize, restore, admin API, model updates,
   git page, tests

2. Auth + Dashboard + Settings  ✓ ВЫПОЛНЕНО
   login, dashboard, settings, site edit, logout, tests

3. Token Editor  ✓ ВЫПОЛНЕНО
   token service, DB, API, editor page, color picker,
   preview iframe (srcdoc), tests

4. Dependencies UI  ✓ ВЫПОЛНЕНО
   deps page, allowlist page, resolve tree, cache config, tests

5. Docker + Dev Environment  ✓ ВЫПОЛНЕНО
   docker-compose, Makefile, air, .env.example,
   vite proxy env vars

6. Doc + Code Cleanup  ← ФИНАЛЬНЫЙ
   delete dead code, extract hardcodes, update docs,
   README, config consistency
```

---

## Критические зависимости

```
5 (Docker)       → 1,2,3,4 (все могут       ─┐
                    использовать make dev)   │
1 (Git+Snapshot) → 2 (Auth) → 3 (Tokens)   ├→ 6 (Cleanup)
1 (Git+Snapshot) → 4 (Deps)                ─┘
```

- Слайс 5 (Docker) не зависит от 1-4 (инфраструктура)
- Слайс 1 не зависит от 2-5 (ядро)
- Слайс 2 зависит от 1 (dashboard читает git status)
- Слайсы 3 и 4 независимы от 1 и 2 (токены/deps хранятся в БД)
- Слайс 5 параллелен с 1-4
- Слайс 6 — финальный

---

## Acceptance Criteria

### Слайс 1: Git + Snapshot
- [x] `POST /api/sites/{id}/git/commit` создаёт коммит с полным состоянием сайта
- [x] `POST /api/sites/{id}/git/restore` восстанавливает из SHA (с safety-snapshot)
- [x] `POST /api/sites/{id}/git/publish` rebase+merge dev→main
- [x] `POST /api/sites/{id}/git/rollback` main→dev, snapshot→main
- [x] `GET /api/sites/{id}/git/commits` возвращает историю
- [x] `GET /api/sites/{id}/git/status` возвращает статус веток
- [x] Страница «Git» в админке: коммиты, статус, publish/restore/rollback
- [x] Все unit-тесты проходят
- [x] `go build ./...` чисто

### Слайс 2: Auth + Dashboard + Settings
- [x] `/login` — ввод токена → validate → localStorage → redirect
- [x] Dashboard: количество сайтов, quick actions, recent builds/snapshots
- [x] Settings: admin token (masked), default locale, redirect status
- [x] SiteHomePage: edit name/locale/hosts
- [x] Logout: clear localStorage → redirect
- [x] Все unit-тесты проходят

### Слайс 3: Token Editor
- [x] Страница «Токены»: categories sidebar, editor, live preview iframe
- [x] ColorPicker: react-colorful (HSV picker + hex input)
- [x] TokenPreview: полноценная страница-пример с CSS custom properties
- [x] Сохранение токенов в БД (nested JSON)
- [x] Все unit-тесты проходят

### Слайс 4: Dependencies UI
- [x] Страница «Зависимости»: npm-подобный список, add/remove/resolve
- [x] Иерархическое дерево resolved deps (expand/collapse)
- [x] Страница «Allowlist»: add/remove patterns
- [x] Cache config + evict
- [x] Все unit-тесты проходят

### Слайс 5: Docker + Dev
- [x] `make dev` запускает postgres + backend (hot-reload) + frontend
- [x] `make build` собирает всё
- [x] `make test` запускает все тесты
- [x] docker-compose up работает из коробки
- [x] .env.example файлы на месте

### Слайс 6: Doc + Code Cleanup
- [ ] Удалён мёртвый код (Head, label.tsx, Placeholder, dev-ws)
- [ ] Хардкод извлечён в конфиги/константы
- [ ] Все доки актуальны
- [ ] README содержит quick start
- [ ] `go vet ./...` и `npx tsc --noEmit` чисто
