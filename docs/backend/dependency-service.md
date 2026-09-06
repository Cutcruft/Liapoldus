# Dependency-сервис (zero-node npm-зависимости, Go)

Спецификация и тест-план. Статус: **дизайн, фаза 1 не начата**.

## 1. Цель и принципы

Добавление сторонних npm-пакетов сайта **без node/npm и shell-скриптов на любой ОС**
(darwin/linux/windows). npm-пакеты — это gzip-tarball'ы, а метаданные о них отдаёт обычный
HTTP API: чтобы «установить» пакет не нужен ни npm, ни node — нужно лишь распаковать
tarball (Go stdlib `archive/tar` + `compress/gzip`) и сбандлить (уже встроенный esbuild Go-API,
платформенные бинарники живут внутри Go-модуля).

Решения (подтверждены пользователем):

1. **Весь биндинг в Go, включая shared-бандлы.** `backend/scripts/build-shared` (npm/esbuild-node)
   заменяется Go-инструментом; один механизм и для фиксированного shared-набора, и для per-site deps.
2. **Источник — npm registry напрямую** (`https://registry.npmjs.org`), не esm.sh/unpkg.
3. **Резолв и бандл deps сайта — при публикации снапшота.** Сервер ходит в registry только на
   первый контакт с новой версией; далее — бессрочный local-кэш.
4. **Диапазоны → пин в снапшот.** Admin задаёт npm-диапазон (`^1.2`); при создании снапшота
   резолвится **точная версия + sha512 integrity**, которая и замораживается («lock»). Никаких
   `latest`/автоапдейтов.
5. **Несовместимые пакеты** (node-builtins `fs/net/child_process/…`, не-браузерные модули) →
   **fail сборки с ясной ошибкой** (имя пакета, отсутствующий модуль, hint), не маскируется в статус.

Зависимости объявляются админом (`pkg@range`); `npm-registry` остаётся публиковательной
площадкой, она же — единственный источник распространения. Нигде шаги не требуют установленного
node/npm; CI и dev-машина работают чисто на Go.

## 2. Термины

| Термин | Значение |
|---|---|
| spec | `name@range` в semver-нотации npm (`^1.2.3`, `~4.0`, `>=1 <2`). `latest`, `*`, теги запрещены (Решение 4). |
| resolved | `name@exact` + `integrity (sha512)` + `tarballURL` — результат резолва диапазона. |
| lock | замороженный в снапшот список resolved (все вершинные+транзитивные, flat). |
| shared set | фиксированные бандлы react/react-dom/jsx-runtime/ui-runtime для `<script type="importmap">` (Этап 5). |
| site deps | зависимости конкретного сайта, бандлятся в артефакты снапшота как `_deps/*`. |

## 3. Registry-протокол (без npm)

- **packument**: `GET https://registry.npmjs.org/<pkg>` (scoped — URL-encode `/` как `/@scope%2Fname`).
  Используем **abbreviated packument** (`Accept: application/vnd.npm.install-v2+json`) — компактный
  документ без описаний/ридов. Ключевые поля на версию: `name`, `version`, `dependencies[]`,
  `dist{integrity, shasum, tarball}`.
- **резолв диапазона**: semver-парсер в Go (`github.com/Masterminds/semver`), выбор максимальной
  удовлетворяющей версии из packument. Нет версии → `ErrUnresolvableSpec`.
- **tarball**: `GET dist.tarball` (или канонический `/-/layout`) → **проверка `dist.integrity`
  (sha512 base64) обязательна**; при отсутствии integrity — ошибка (sha1 fallback запрещён) —
  supply-chain (см. §9).
- **неизвестный пакет** (404/невалидный packument) → `ErrPackageNotFound`.

## 4. Домен и хранилище (content-addressed кэш)

Домен `application/deps` (`DomainService`), храним как и прочие сущности (memory+Postgres), blobs — на диске.

```
domain:
  Dependency   { SiteID, Name, Spec, ResolvedVersion, Integrity }   // вершинные, на сайт
  ResolvedGraph: FlatResolve → map[pkg]ResolvedPackage               // lock снапшота
  SnapshotLock { SnapshotID, Deps []LockedDep{Name, Version, Integrity} }

storage (новые tables `005_dependencies.sql`):
  dependencies(site_id, name, spec, resolved_version, integrity, updated_at)   // вершинные
  dep_packages(name, version, integrity, tarball_url, dependencies JSON)       // кэш метаданных
  dep_blobs(sha256, size)                                                       // индексы файлов
  blobs на диске: <data>/deps/<sha256[0:2]>/<sha256>  (диск-стор, по образцу artifactstore)
```

- Версии **immutable** → записи кэша никто не переписывает; `dep_packages` и `dep_blobs` растут
  монотонно, эвикция не нужна.
- Идемпотентность: повторный резолв того же `name@exact`, fetch того же tarball — no-op.

## 5. Резолв и раскладка графа

- **Flat-граф с одной версией на пакет** (Решение фазы 1): резолвим вершинные spec'ы, затем
  транзитивные (семверно, тем же методом). Если два пакета требуют **разные** версии одного
  пакета (напр. react@17 и react@18) — `ErrVersionConflict` с понятным сообщением
  («pkgA requires react@17, pkgB requires react@18»). Вложенные версионные layout — фаза 2.
- **Раскладка**: при публикации распаковываем tarball'ы содержимое в локальный
  `node_modules/<name>` (плоский layout, как npm hoisting). Всё из кэша по sha256.
  Согласовано: точная версия+интегрити из снапшота (lock) → раскладка детерминирована и
  пересборка (Этап 3 rebuilder) повторяема.
- peers: peerDependencies учитываем при резолве диапазона, но не инсталим отдельно, если
  версия уже в графе (фаза 1). Неудовлетворённый peer → предупреждение в лог, не fail
  (финальное ужесточение — фаза 2).

## 6. Бандл deps (Go esbuild)

- Temp workspace на publish: `node_modules/` (раскладка §5) + по одному entry-файлу на
  вершинный dep: `import "<spec>"`. esbuild (Go API): `bundle, format=esm, platform=browser,
  target=es2022, external=SharedExternals` → единый ESM-файл на dep
  → артефакт `<snapshot>/_deps/<pkg>@<version>.js`. react/react-dom/ui-runtime НЕ дублируются:
  dep-бандл импортирует их bare (external) и резолвится через общий import-map (§7).
- **subpath-импорты** (`lodash/map`, `pkg/foo.js`): обнаруживаются из site-кода и вершинных deps
  на этапе materialization (список внешних bare и bare-subpath спецификаторов);
  на каждый — отдельный бандл `_deps/<pkg>@<ver>/<subpath>.js`. Точные маппинги import-map для
  каждого. Фаза 1 покрывает top-level + прямые subpath; экзотика (коллизии имён subpath между
  версиями) — фаза 2.
- **CSS/не-JS ассеты** пакетов: фаза 2 (import-map для CSS-запросов и `<link>`, взятие из
  бандла и самостоятельная отдача). Фаза 1: пакет с CSS-импортами → fail с ясным сообщением,
  что ассеты не поддерживаются ещё.
- **Несовместимые**: esbuild-ошибка/нерезолвимое импорт → оборачиваем в `DepBuildError`
  `{pkg, version, missing, hint}` (hint: «fs/net/child_process — браузерный рантайм не
  предоставляет; выберите альтернативу без node-API»). Список заблокированных builtins
  статичен (`fs pedendes/net/path/child_process/process/stream/…`).

## 7. Снапшот + lock + import-map

- `CreateSnapshot` получает опциональные вершинные deps сайта (резолв на этом шаге; невалид →
  ошибка снапшота). В снапшот пишется `deps_lock: LockedDep[]` (top-level + транзитивные, flat).
- Артефакты публикации: `_deps/*.js` рядом с site bundle; **отдаются существующим FileServer**
  артефактов (`/build/<site>/<env>/<snapshot>/_deps/<file>`) — отдельного endpoint нет.
- `manifest.json` пополняется:
  ```
  "deps": { "lodash": "/build/<site>/<env>/<snapshot>/_deps/lodash@4.17.21.js",
            "lodash/map": "/build/.../_deps/lodash@4.17.21/map.js" }
  ```
- `manifest.externals` = `SharedExternals` + вершинные deps сайта (+ их subpath-импорты).
- Включение react во внешние deps: dep-бандл импортирует `react` bare, import-map (Этап 5)
  склеивает `manifest.shared + manifest.deps` в `<script type="importmap">` — react из
  `/build/_shared/react/18.3.1.js` один на всех.

## 8. API

- Admin: `GET /api/sites/{id}/dependencies`, `POST /api/sites/{id}/dependencies {name, spec}`,
  `DELETE /api/sites/{id}/dependencies/{name}`. Порт `deps.DomainService`; хендлер в admin api.
- Ошибки: неизвестный пакет → 404; нерезолвящийся диапазон → 422 `{error, detail|hint}`;
  конфликт версий → 422; `latest`/тег-спек → 400.
- Резолв **не** в POST: объявление зависимости валидирует форму/пакетный синтаксис, а сам резолв
  диапазона в пин-версию происходит на `CreateSnapshot` (Решение 3). Чтобы admin видел, что подтянет,
  POST отдаёт `resolvedVersion` информативно-пробно (best-effort) — но lock-источник только снапшот.

## 9. Supply-chain и безопасность

- exact-версия + sha512 integrity на каждой распаковке; репозиторий никогда не берёт
  непроверенное тело.
- Версии immutable → кэш бессрочен; «оффлайн» бьёт только по новым резолвам.
- Никаких `latest`/тегов; публикация = явный snapshor (admin меняет диапазон/версию).
- SBOM (`deps_lock` со всеми integrity) — часть снапшота, доступна в API снапшота.
- Опционально (фаза 2): allowlist scopes. Публичные маршруты не открываются: зависимости не
  содержат секретов/токенов админа.
- Rate-limit/backoff к registry + таймауты; ретраи с экспоненциальной задержкой.

## 10. Замена shared-сборки (полный zero-node)

- `backend/scripts/build-shared` (npm ci + esbuild-node) → Go `cmd/dependency-build`:
  тот же fetcher/resolver/cache, резолвит фиксированную таблицу
  `react@18.3.1` (+ её транзитивы), `react-dom`, `react/jsx-runtime`, `@liapoldus/ui-runtime`
  (из `ui-runtime/src`, вход `index.ts`), бандлит → `internal/infra/build/shared/embed/<key>/<ver>.js`.
- Результат всё так же коммитится в репо (реального node не нужно). `shared.go` таблица Artifacts
  остаётся, сверка — unit-тестом (задекларированное = сгенерённое).
- Раскладка shared-артефактов в `build/_shared/...` при `Install` — без изменений.
- Shell-обёртки CI (`scripts/*.sh`) — ликвидируются; при необходимости — Go-таски вместо них.

## 11. Этапность

**Фаза 1 (MVP, следующая работа):**
1. `domain` + `storage` (tables 005, memory+postgres), `deps.Service` (CRUD вершинных deps,
   резолв + flat-graph + кэш + integrity).
2. Registry-фетчер `internal/infra/deps/registry` (packument abbr, tarball+integrity, semver через
   Masterminds/semver, rate-limit/backoff).
3. Диск-стор blobs `internal/infra/deps/store` (по образцу artifactstore).
4. Интеграция публикации: lock в снапшот, materializer (node_modules-layout из кэша → esbuild
   per-dep бандлы в `_deps/`, `manifest.deps` + externals, `DepBuildError` для несовместимых).
5. Admin API deps CRUD; ошибки 400/404/422.
6. Тесты: unit (semver-резолв, integrity, конфликтные версии, cache no-op), integration
   (фейковый registry httptest + реальный esbuild Go; постгрес-очередь §5 прод-аналог), e2e
   (публикация сайта с lodash-es → бандл `_deps` + import-map в манифесте).
7. Документация обновляется вместе с кодом; коммиты по шагам, отдельно после каждой фазы.

**Фаза 2:** subpath-покрытие полностью, вложенные версионные layout (конфликты), CSS/ассеты,
peer-политика (fail по неудовлетворённым peers), allowlist scopes, кэш-лимиты/эвикция.

**Фаза 3:** `cmd/dependency-build` (замена `scripts/build-shared`, npm больше нигде не упоминается),
ликвидация shell-обёрток, SBOM/audit-экспорт.

## 12. Открытые ограничения (фиксируем сейчас, решим в фазах)

- Monorepo-/workspace-пакеты и git-спецификаторы npm (`user/repo#branch`) не поддерживаются
  (только registry-scoped).
- Duplicate-версии — только фазой 2 (вложенный layout).
- CSS/фонты пакетов — фаза 2; фаза 1 честно падает с hint.
- Один npm-источник публичный; mirror/registry-proxy — вне scope.