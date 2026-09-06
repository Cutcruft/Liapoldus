# Dependency-сервис (zero-node npm-зависимости, Go)

Спецификация и тест-план. Статус: **фаза 1 — готово: domain/storage/registry/deps.Service/admin API; шаги 3 (диск-стор) и 4 (материализация+бандлинг+e2e) — выполнены; subpath-импорты из site-кода и внутри вершинных deps (фаза 2, слайсы 1–2) — реализованы; вложенные версионные layout (фаза 2, слайс 3) — реализованы; CSS/не-JS ассеты пакетов (фаза 2, слайс 4) — реализованы; peer-политика (фаза 2, слайс 5) — реализована; осталось: allowlist scopes, кэш-лимиты/эвикция**.

## 1. Цель и принципы

Добавление сторонних npm-пакетов сайта **без node/npm и shell-скриптов на любой ОС**
(darwin/linux/windows). npm-пакеты — это gzip-tarball'ы, а метаданные о них отдаёт обычный
HTTP API: чтобы «установить» пакет не нужен ни npm, ни node — нужно лишь распаковать
tarball (Go stdlib `archive/tar` + `compress/gzip`) и сбандлить (уже встроенный esbuild Go-API,
платформенные бинарники живут внутри Go-модуля).

Решения (подтверждены пользователем):

1. **Весь биндинг в Go, включая shared-бандлы.** `backend/scripts/build-shared` (npm/esbuild-node)
   заменяется Go-инструментом; один механизм и для фиксированного shared-набора, и для per-site deps.
2. **Источник — npm registry напрямую** (`https://registry.npmjs.org`, переопределяется env
   `LIAPOLDUS_NPM_REGISTRY` — зеркала/прокси, решение 2), не esm.sh/unpkg.
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
  ResolvedGraph: ResolveLock → экземплярный граф                // lock снапшота
  SnapshotLock { SnapshotID, Deps []LockedDep{Name, Version, Integrity, Hoisted, RequestedBy} }
    LockedDep.RequestedBy []string — родительские экземпляры «name@version» (или "site" для вершинных);
    LockedDep.Hoisted — экземпляр раскладывается в корневой node_modules/<name>. Слайс «вложенные
    версионные layout»: один пакет может встречаться несколько раз (по версии), порядок Deps = BFS
    (родители раньше детей) → раскладка детерминирована.

storage (новые tables `005_dependencies.sql`):
  dependencies(site_id, name, spec, resolved_version, integrity, updated_at)   // вершинные
  dep_packages(name, version, integrity, tarball_url, dependencies JSON)       // кэш метаданных
  blobs на диске: <data>/deps/<name>/<version>.tgz  (диск-стор internal/infra/deps/store; таблицы индексов нет)
```

- Версии **immutable** → записи кэша никто не переписывает; `dep_packages` и `dep_blobs` растут
  монотонно, эвикция не нужна.
- Идемпотентность: повторный резолв того же `name@exact`, fetch того же tarball — no-op.

## 5. Резолв и раскладка графа

- **Экземплярный граф (вложенный версионный layout, реализован)**. `deps.Service.ResolveLock`
  обходит граф BFS-очередью граней `{name, spec, requestedBy}`; для каждой грани — reuse первого
  (в порядке создания) экземпляра имени, чья версия удовлетворяет range; иначе резолвится новая
  версия и создаётся экземпляр: `Hoisted = (это первый экземпляр имени)`, `RequestedBy` = родитель
  («site» для вершинных). Вершинные имена уникальны (сортировка top-level по имени → BFS
  детерминирован); дубликаты имени — только среди транзитивных, это норма, `ErrVersionConflict`
  больше не эмитится (остаётся определённым для прежних контрактов, §11). Неуспех резолва range
  — `ErrUnresolvableSpec` на грани.
- **Раскладка** (materializer `internal/infra/deps/layout`): по записям lock в BFS-порядке
  вычисляются физические пути и распаковываются tarball'ы (всё из дискового кэша по sha512):
  hoisted-экземпляр → `node_modules/<name>`; несовместимый второй экземпляр → под каждым
  родителем `node_modules/<parent>/node_modules/<name>`. Вложенный экземпляр без известного
  родителя или запрошенный «site» → `DepBuildError` (битый lock). Полная детерминированность:
  точные версии+интегрити из lock → пересборка (Этап 3 rebuilder) повторяема.
- **Legacy flat-locks** (слайсы 1–2, без `hoisted`/`requestedBy`) читаются как прежде: единственный
  экземпляр каждого имени раскладывается в корневой `node_modules/<name>`.
- **Резолв на сборке**: esbuild стандартным node-резолвом (walk-up по `node_modules/`) находит для
  каждого импортирующего файла ближайший физический экземпляр → каждый потребитель получает свою
  версию. Вершинные имена уникальны → `dist/_deps/<name>@<ver>.js` и import-map не меняются;
  дубликаты инлайнятся в бандлы потребителей.
- **Peer-политика (peer-fail, слайс 5)**. Registry парсит `peerDependencies` +
  `peerDependenciesMeta` (packument abbr), `ResolvedVersion`/`DepPackage`/`LockedDep`
  несут их (SBOM-прозрачность). После заморозки графа `checkPeers` обходит все экземпляры
  в BFS-порядке: обязательный peer (не отмеченный `optional` и не self-ссылка) должен быть
  удовлетворён — экземпляром графа с версией в диапазоне peer **или** фиксированным shared
  external (react 18.3.1, react-dom 18.3.1, react/jsx-runtime, @liapoldus/ui-runtime 0.1.0,
  `build/shared.Versions()`) с версией в диапазоне. Иначе — `ErrUnsatisfiedPeer`
  (422 на API): `потребитель@version требует peer "name" (range); доступно: …`.
  Проверка на уровне имя/версия; физическую достижимость удовлетворителя из позиции
  потребителя оставляет walk-up вложенного layout. Optional-peer не требуем: «тихий»
  неверный резолв (напр. peer react@^17 при shared 18.3.1) теперь падает на создании
  снапшота, а не уходит в браузер.

## 6. Бандл deps (Go esbuild)

- Temp workspace на publish: `node_modules/` (раскладка §5) + по одному entry-файлу на
  вершинный dep: `import "<spec>"`. esbuild (Go API): `bundle, format=esm, platform=browser,
  target=es2022, external=SharedExternals` → единый ESM-файл на dep
  → артефакт `<snapshot>/_deps/<pkg>@<version>.js`. react/react-dom/ui-runtime НЕ дублируются:
  dep-бандл импортирует их bare (external) и резолвится через общий import-map (§7).
- **subpath-импорты** (`lodash/map`, `pkg/foo.js`): реализовано (фаза 2, слайсы 1–2). На этапе
  materialization сканируются definition-sources site-кода **и исходники самих вершинных deps**
  на bare-subpath спецификаторы (общий сканер `build.ScanImportSpecifiers`, regex
  `(?:^|[^.\w])(?:from|import)(?:\s*\(|\s+)…` — исключает `x.from()`); учитывается только subpath
  **объявленных** вершинных deps. На каждый — отдельный бандл `_deps/<pkg>@<ver>/<subpath>.js`
  + запись `manifest.deps` (ключ = спецификатор) + `manifest.externals` += спецификатор.
  Потребляющий dep-бандл держит спецификатор external (shared import-map), транзитивные остаются
инлайн. Subpath нерезолвится → `DepBuildError`; non-JS/CSS ассет (`.png`/`.woff`/…) →
   fail с hint (одинаково для site-кода и dep-кода). Коллизии имён subpath между разными
   версиями одного пакета резолвятся тем же физическим walk-up (§5): субпат-артефакт берётся
   из hoisted-экземпляра.
- **CSS/не-JS ассеты пакетов** (слайс 4): CSS поддерживается тремя путями, всё остальное
  (`.png`/`.woff`/…) — честный fail с hint.
  - **Combined CSS** бандла: относительный/self-баре CSS-импорт внутри пакета
    (`import "./brand.css"`) — esbuild css-loader в `bundle()`: импорт вырезается из JS,
    стили собираются в **соседний** `.css`-артефакт `_deps/<pkg>@<ver>.css` (тот же
    basename, `.js`→`.css`), `DepRef.CSSArtifact` фиксируется. Stub не нужен.
  - **Bare CSS-subpath** (`agent/theme.css`) из site-кода или из кода вершинного dep —
    `bundleCSS()`: css-бандл физического `node_modules/<pkg>/<subpath>` → артефакт
    `_deps/<pkg>@<ver>/<subpath>.css` + **JS-stub** `…css.js` (`export default undefined;`),
    на который import-map мапит спецификатор (голый CSS-импорт невалиден в ESM);
    спецификатор → `manifest.externals` + `manifest.deps`.
  - `<link>`-источник: `manifest.styles` = дедуплицированный отсортированный список публичных
    `.css`-артефактов (combined + subpath-ные); runtime-shell инжектит `<link rel=stylesheet>`
    до монтажа site-бандла. Self-баре css-субпат пакета инлайнится в combined css автоматически.
  - Фонт/картинка в бандле → `DepBuildError` с hint («No loader is configured» → «…CSS
    поддерживается, ассеты (шрифты/картинки) — нет»).
- **Несовместимые**: esbuild-ошибка/нерезолвимое импорт → оборачиваем в `DepBuildError`
  `{pkg, version, missing, hint}` (hint: «fs/net/child_process — браузерный рантайм не
  предоставляет; выберите альтернативу без node-API»). Список заблокированных builtins
  статичен (`fs pedendes/net/path/child_process/process/stream/…`).

## 7. Снапшот + lock + import-map

- `CreateSnapshot` получает опциональные вершинные deps сайта (резолв на этом шаге; невалид →
  ошибка снапшота). В снапшот пишется `deps_lock: LockedDep[]` (top-level + транзитивные;
   вложенный версионный layout — по BFS `name@version` с `hoisted`/`requestedBy`;
   обязательные peer-контракты удовлетворены peer-политикой (§5), `peerDependencies`/
   `peerDependenciesMeta` каждого экземпляра фиксируются в lock).
- Артефакты публикации: `_deps/*.js` рядом с site bundle; **отдаются существующим FileServer**
  артефактов (`/build/<site>/<env>/<snapshot>/_deps/<file>`) — отдельного endpoint нет.
- `manifest.json` пополняется:
  ```
  "deps": { "lodash": "/build/<site>/<env>/<snapshot>/_deps/lodash@4.17.21.js",
            "lodash/map": "/build/.../_deps/lodash@4.17.21/map.js",
            "agent/theme.css": "/build/.../_deps/agent@1.0.0/theme.css.js" }
  "styles": ["/build/<site>/<env>/<snapshot>/_deps/agent@1.0.0.css",
             "/build/.../_deps/agent@1.0.0/theme.css"]
  ```
  `deps[<css-спецификатор>]` указывает на JS-stub, реальный CSS — в `manifest.styles` для `<link>`.
- `manifest.externals` = `SharedExternals` + вершинные deps сайта (+ их subpath-импорты).
- Включение react во внешние deps: dep-бандл импортирует `react` bare, import-map (Этап 5)
  склеивает `manifest.shared + manifest.deps` в `<script type="importmap">` — react из
  `/build/_shared/react/18.3.1.js` один на всех.

## 8. API

- Admin: `GET /api/sites/{id}/dependencies`, `POST /api/sites/{id}/dependencies {name, spec}`,
  `DELETE /api/sites/{id}/dependencies/{name}`. Порт `deps.DomainService`; хендлер в admin api.
- Ошибки: неизвестный пакет → 404; нерезолвящийся диапазон → 422 `{error, detail|hint}`;
  неудовлетворённый peer (peer-политика §5) → 422;
  конфликт версий → 422 (legacy-контракт; при вложенном layout резолв больше не падает — см. §5);
  `latest`/тег-спек → 400.
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

**Фаза 1 (MVP):**
1. ✅ `domain` + `storage` (tables 005, memory+postgres), `deps.Service` (CRUD вершинных deps,
   резолв + flat-graph + кэш + integrity).
2. ✅ Registry-фетчер `internal/infra/deps/registry` (packument abbr, scoped `%2F`, semver через
   Masterminds/semver, backoff, 404/unresolvable маппинг).
3. ✅ Диск-стор tarball-блобов `internal/infra/deps/store` (`<data>/deps/<name>/<version>.tgz`,
   write-and-rename, идемпотентный `Save`; `registry.Client.Fetch` верифицирует sha512-integrity).
4. ✅ Интеграция публикации: materializer (node_modules-layout из кэша + esbuild per-dep бандлы в
   `dist/_deps/<name>@<version>.js` (scoped → `%2F`)), `manifest.deps` (публичный артефакт `dist/_deps/...`,
   отдаётся существующим FileServer `/build/<site>/<env>/<snapshot>/dist/...`), `manifest.externals`
   = `SharedExternals` + вершинные deps + transitives-инлайнинг; `DepBuildError`
   `{pkg, version, missing, hint}` для несовместимых (браузерные builtins `fs` и пр.).
   Lock в снапшот уже готов: `snapshot.Service` получает variadic `LockResolver` = `deps.Service.ResolveLock`.
5. ✅ Admin API deps CRUD (`GET/POST /api/sites/{id}/dependencies`, `DELETE .../{name}` с scoped `%2F`);
   ошибки 400/404/422 (`ErrInvalidDepSpec`/`ErrPackageNotFound`/`ErrUnresolvableSpec`+`ErrVersionConflict`).
6. ✅ Тесты unit (валидация, best-effort probe, flat-граф, reuse/конфликт, кэш no-op, packument-резолв с
   httptest-фейком, admin + lock в снапшот, 422 при нерезолвящемся графе; layout: bundle/DepBuildError/
   scoped/integrity/диск-кэш) и integration (локальный registry-сервер + memory/postgres).
7. ✅ e2e: локальный httptest-registry (реальный sha512) + реальный esbuild: полный цикл
   build → `_deps`-бандл с инлайном transitive + `from "react"` external, site-бандл держит
   `from "agent"` external, `manifest.json` deps/externals; падение на node-builtin импорте.

**Фаза 2:** ✅ subpath-импорты из site-кода (слайс 1: сканирование definition-sources, отдельные
бандлы `_deps/<pkg>@<ver>/<subpath>.js` + import-map + externals, scoped `%2F`, fail-fast на
нерезолвящемся/CSS). ✅ subpath-импорты внутри вершинных deps (слайс 2: общий сканер
`build.ScanImportSpecifiers` для layout+materializer, `scanDepSubpaths` по распакованным
`node_modules/<dep>`, per-dep externals в бандле, shared-корни/react остаются external,
транзитивные/self — инлайн, нерезолвящийся и non-JS subpath dep-кода → DepBuildError с hint).
✅ вложенные версионные layout (слайс 3: экземплярный граф в `ResolveLock` — BFS по граням,
reuse-first-satisfying по порядку создания, `LockedDep{Hoisted,RequestedBy}`; раскладка
hoisted → корневой `node_modules/<name>`, несовместимая версия → под каждым родителем;
физический walk-up резолвит каждому потребителю свою версию; legacy flat-locks без маркеров
читаются как прежде; `ErrVersionConflict` не эмитится; unit-тесты резолвера и layout + e2e
двух вершинных deps с разными версиями транзитивного).
✅ CSS/не-JS ассеты пакетов (слайс 4: combined css соседним `.css`-артефактом через
esbuild css-loader в `bundle()`; bare css-subpath — `bundleCSS()`: свой `.css`-артефакт +
JS-stub `…css.js` в import-map; `.css` убран из unsupported-ext; non-CSS ассеты так же
fail-fast с hint, «No loader is configured» детектится в `depBuildError`;
`manifest.styles` (dedup, сортировка) для `<link>`; инверсия css-reject-тестов на png +
unit-тесты трёх путей + e2e `TestDependencyBuildCSSArtifacts`).
✅ peer-политика (слайс 5: registry парсит `peerDependencies`/`peerDependenciesMeta`,
`ResolvedVersion`/`DepPackage`/`LockedDep` несут их; post-pass `checkPeers` по BFS-порядку —
обязательный peer удовлетворяется экземпляром графа или фиксированным shared external,
иначе `ErrUnsatisfiedPeer` (422), optional/self exempt; lock фиксирует peer-метаданные;
unit: 6 кейсов политики + парсинг packument, integration `TestDependencyPeerPolicy`).
Осталось: allowlist scopes, кэш-лимиты/эвикция.

**Фаза 3:** `cmd/dependency-build` (замена `scripts/build-shared`, npm больше нигде не упоминается),
ликвидация shell-обёрток, SBOM/audit-экспорт.

## 12. Открытые ограничения (фиксируем сейчас, решим в фазах)

- Monorepo-/workspace-пакеты и git-спецификаторы npm (`user/repo#branch`) не поддерживаются
  (только registry-scoped).
- Duplicate-версии — поддержаны вложенным версионным layout (слайс 3); см. §5.
- Шрифты/картинки/др. статические ассеты пакетов — не бандлятся (fail с hint); CSS
  поддерживается (слайс 4): combined css соседним артефактом + bare css-subpath
  (`manifest.styles` для `<link>`).
- Один npm-источник публичный; mirror/registry-proxy — вне scope.