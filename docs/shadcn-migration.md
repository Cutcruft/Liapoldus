# Миграция админки на shadcn/ui (Этап 6)

Согласованные решения (Е6): миграция **слайсами вместе с фичами** (без big-bang), полный набор базовых + overlay + форм, shadcn только в `admin` (ui-kit остаётся layout-примитивами), **дефолтная нейтральная тема** shadcn (neutral/zinc, oklch) — текущий синий акцент уходит.

## Целевой стек

- **shadcn/ui** (new-york, Tailwind v4) на React 18 + Vite 6 + Tailwind v4 (`@theme inline`, oklch-переменные в `admin/src/index.css`).
- **Алиас** `@/` → `admin/src` (tsconfig paths + `vite resolve.alias`) — стандарт для shadcn-компонентов.
- **`src/lib/utils.ts`** — `cn` (clsx + tailwind-merge).
- Компоненты копируются в `admin/src/components/ui` (по-файлово); Radix-пакеты — peer-зависимости admin (добавляет `npx shadcn add`, руками radix не ставим).
- **ui-kit не трогаем.** `ConfirmButton`/`Field`/`EntityTable` становятся тонкими обёртками над shadcn (их API сохраняем, чтобы не переписывать вызовы).
- Анимации: `tw-animate-css`.

## Миграционная карта (текущее → shadcn)

| Сейчас | shadcn |
| --- | --- |
| inline `<button className="rounded bg-blue-600…">` (primary) | `Button` (default) |
| outline-кнопки (`border border-neutral-300…`) | `Button variant=outline / ghost / secondary` |
| `ConfirmButton` (кастомный confirm) | `AlertDialog` (обёртка ConfirmButton сохраняет API) |
| `Field` (label-обёртка) | `Label` + `Input` (Field остаётся при необходимости) |
| `EntityTable` (grid) | `Table` (EntityTable-обёртка над Table) |
| `ToolButton` (TreePanel) | `Button variant=ghost size=sm` (icon) |
| native `<select>` (binding source, enum, routes action) | `Select` (Radix) |
| `AssetPicker` модалка | `Dialog` |
| табы переводов (инлайн) | `Tabs` |
| loading «…» / скелетоны | `Skeleton` |
| ошибки сохранения inline | `Sonner` (toast) |
| бейдж расширения/типа | `Badge` |
| checkbox-поля | `Checkbox` |
| формы M3 (definition) | `Form` (react-hook-form + zod) — по мере появления сложных форм |

## Тест-влияние (критично)

- `Input`/`Button`/`Checkbox` — нативные элементы, существующие `getByLabelText`/`getByRole('button')` и `fireEvent.change/toFrame` работают без изменений.
- **Radix `Select` — не нативный.** Паттерны `getByRole('combobox')` + `fireEvent.change` **ломаются**. Переписывать на: клик по триггеру (`click`) → `getByRole('option', { name: … })` → `click`. Затрагивает schema-форму (enum + binding source), routes action, future-экран форм.
- `Dialog`/`AlertDialog` рендерятся порталом в `<body>` — `screen`-запросы находят; для стабильности jsdom нужен **setup-файл admin** с полифилами: `ResizeObserver`, `PointerEvent`, `Element.prototype.scrollIntoView`, `HTMLElement.prototype.hasPointerCapture`.
- Продолжаем на `fireEvent`, `userEvent` не подключаем.

## U-слайсы (попутно с фичами)

- **U0 — Setup.** `shadcn init` (neutral), алиас `@/`, `lib/utils.ts`, тема в `index.css`; jsdom-полифилы (setupFiles в vitest admin); проверка peer-deps Radix под React 18. Критерий: всё зелёное, один пробный компонент в продакшн-пути.
- **U1 — Примитивы.** `Button/Input/Label/Badge/Skeleton/Sonner/AlertDialog`. `ConfirmButton` → обёртка над AlertDialog; `SitesPage` переводится первой (эталон списка).
- **U2 — Таблицы/диалоги/табы.** `Table/Dialog/Tabs`. `EntityTable` → обёртка над Table; списки Pages/Routes/Contents/Assets (по мере касания слайсами M2). AssetPicker → Dialog.
- **U3 — Формы и селекты.** `Select/Checkbox/Textarea` (+ `Form` RHF+zod для M3-форм). Переписываются native-селекты schema-формы и роут-экранов; переписать затронутые тесты (option-клики).
- **U4 — Редактор.** Chrome редактора: `ToolButton`/TreePanel/Inspector/Canvas-табы → shadcn; слайс Tiptap строится сразу на shadcn-контролах.
- **U5 — Данные/Dashboard.** Табличный dashboard, Sonner повсеместно, prep тёмной темы (только vars, переключатель — по желанию).

Каждый U-слайс: зелёный `npm test` + `tsc --noEmit` + `build`, строка в `docs/feature-status.md`.

### Выполнено

- **[x] U0 + U1 (одним слайсом).** `components.json` (new-york, neutral, lucide), тема в `styles.css` (`@theme inline`, oklch-переменные + `.dark`-блок, `tw-animate-css`), `src/lib/utils.ts` (`cn`), алиас `@/` (tsconfig paths + vite resolve) — добавлено в `admin/vite.config.ts` и `tsconfig.json`. Базовый набор в `src/components/ui/`: button, input, label, badge, skeleton, alert-dialog, sonner. Зависимости: `@radix-ui/react-alert-dialog|label|slot`, `cva`, `clsx`, `tailwind-merge`, `lucide-react`, `sonner`, `tw-animate-css` (React 18 deduped, peer-чисто). `src/test-setup.ts` (setupFiles): полифилы ResizeObserver/PointerEvent/hasPointerCapture/scrollIntoView/matchMedia для Radix в jsdom. `ConfirmButton` → AlertDialog (layout-независимый API сохранён; busy-состояние убрано). `SitesPage` — эталон списка на Button/Input/Skeleton/AlertDialog. `Toaster` смонтирован в `main.tsx` (вне тестов). Тесты: админка 114/114 (AlertDialog-флоу confirm'ов проходят без правок тестов), корень 366/366, typecheck чистый, build OK (JS 389 kB, CSS 31.79 kB).

## Риски

- **React 18 vs 19**: peer-deps Radix обычно поддерживают 18, но на U0 проверяем `npm ls react`; при конфликте — фикс версий или legacy-инсталляция shadcn.
- **Бандл**: вес админки вырастет на radix-модули; следить в сборке.
- **Визуальная регрессия**: нейтральная тема меняет облик (синий→zinc). Скриншот-тестов нет, so пересогласование безболезненно; единственный риск — неоднородность между мигрированным и ещё старым UI в переходный период (принято в обмен на поэтапность).