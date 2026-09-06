# Редизайн админки — модель данных и бэкенд

Сущности, изменения API, влияние на `ui-runtime` и сборку. Решения-теги R* — из
`README.md`. Разрешена полная перестройка слоёв (R16); старые данные сбрасываем (R17).

- Связанные документы: `README.md`, `interface.md`, `docs/api-admin.md`, `docs/api-client.md`, `docs/ui-runtime/spec.md`.
- Статус: целевая модель; фиксация контрактов — в слайсах `todo.md`.

---

## 1. Целевая доменная модель

### 1.1 Компонент (по сути как сейчас, уточняется schema)

```ts
ComponentDefinition {
  id: string;          // уникальный id (пространство имён сайта)
  source: string;      // TSX (один файл, R6)
  version: string;     // git SHA коммита
  schema: PropSchema;  // авто-вывод из TS props (R8) + ручные уточнения
}
```

- Хранение/версии — через существующий git-механизм (компоненты — git-модель, этап 2).
- Новое: **серверная история/реестр** для UI (list, detail, history, где используется).

### 1.2 Инфраструктура

Переиспользуем домен дескрипторов `ui-runtime` (`docs/ui-runtime/json-descriptors.md`):

```ts
OperationDescriptor { kind:'operation'; id; provider: 'http'|'ws'|'sse'|'graphql';
                      typeOp: 'query'|'mutation'; method; path; scope: 'public'|'server'; ... }
EndpointDescriptor  { kind:'endpoint'; id; method; operationId; ... }
FormDefinition      { id; schema: FormFields[]; submit: { target: 'endpoint.<id>'|'operation.<id>' } }
```

- CRUD этих сущностей — admin-операции (раньше только декларативные в контракте).
- Внешние адреса frontend ходит сам (R12), без серверного прокси и секретов.

### 1.3 Страница и роут (новая модель, R9/R10)

```ts
Route {
  id; siteId; name;
  matcher: string;     // regex URL (якорные ^…$)
  priority: number;
  action: { type: 'page'; pageId: string } | { type: 'redirect'; target; status; keepQuery };
}

Element {
  id;                    // стабильный (draggable, bindings ссылаются стабильно)
  componentId: string;
  props: Record<string, { kind: 'literal'; value: unknown }
                     | { kind: 'binding'; source: BindingSource }>;
  bindings?: BindingSource[];  // дубль для явного списка (форма/операция/контент/query)
}

BindingSource =
  | { kind: 'content';    contentId; field }
  | { kind: 'form';       formId }
  | { kind: 'operation';  operationId }
  | { kind: 'query';      param }
  | { kind: 'routeGroup'; index: number }   // группы regex-роута → props (R10)

Page {
  id; siteId; name;
  list: Element[];       // чисто линейный, порядок значим (R9)
}
```

- Вложенного дерева нет — убрана сущность `ComponentNode` и её обработчики (tree, tree-utils).
- «Страницы» в UI = связка `Route → Page` (реестр роутов + композиция).

### 1.4 site-settings (новая сущность, R14)

```ts
SiteSettings {
  siteId: string;
  title: string; description: string; lang: string;
  faviconAssetId?: string;
  meta: Record<string, string>;   // og:, twitter:, доп. теги
  defaultPageId?: string;         // стартовая страница (берется из initialTree контракта)
}
```

- Значения текущих site-полей (имя/slug) не дублируем — только UI/metdata-уровень.
- Шрифты/фавиконка — ассеты (Медиа), в settings — ссылки `assetId`.

### 1.5 Контент и локали (R5)

- Сущность контента без изменений в ядре: `Content{ Base Fields; Translations: {locale → fields} }`, фоллбек `merged()`.
- Новое на API/UI: селектор языка, список локалей сайта, сводка «переведено/нет», отсутствующие переводы маскируются базовыми.

### 1.6 Снапшот и публикация

```ts
Snapshot { id; siteId; name; pages: SnapshotPage[]; depsLock; createdAt }   // как есть
Deployment { siteId; environment: 'development'|'production'; snapshotId }   // активный пин прод (R13)
```

- Добавляем пин активного прода (`Deployment`), откат = смена `snapshotId` и пере-публикация артефакта из artifactstore (уже идемпотентна).

---

## 2. Изменения API

### 2.1 Новые админ-ресурсы

| Ресурс | Операции | Комментарий |
| --- | --- | --- |
| `/components` | list, get, getHistory, delete | реестр компонентов (R6/R8); source и schema |
| `/settings` (site) | get, update | site-settings CRUD (R14) |
| `/locales` | list, getSummary, updateTranslation | сводка/редактирование переводов (R5) |
| `/operations`, `/endpoints`, `/forms` | CRUD | инфраструктура как управляемая модель (R11) |
| `/pages`, `/routes` | CRUD (новая модель листа) | вместо tree-операций |
| `/deployments` | pin, rollback | активный прод-снапшот, откат (R13) |

### 2.2 Упрощаются/удаляются

- tree-операции (`/tree`), ComponentNode-моделей не будет.
- Старый редактор/превью-флоу `page-store`/`tree-utils` удаляются.
- Скрытые страницы контента/форм перекладываются на consolidated ресурсы.

### 2.3 Runtime-контракт (`api-client`)

- Остаются: routes (новый формат с page/list), initialTree → **initialPage/list** (страница по умолчанию из site-settings/defaultPage или первый роут), operations/endpoints/forms (инфраструктура).
- Добавляется: `siteSettings`, route-группы в дескрипторах роутов.
- Прежние поля контракта, связанные с деревом, заменяются на лист.

---

## 3. Влияние на ui-runtime

- **Рендер**: `PageRenderer`/`RouteOutlet` рендерят **лист** (порядок элементов), вместо рекурсии по дереву; элемент → компонент + props (литерал или приведённый binding). Механизм `InstanceNode` заменяется на `ElementNode`.
- **Route-группы**: при матче роута сохраняем regex-группы → доступ `{{route.$n}}` и `BindingSource.routeGroup` (R10).
- **Операции/формы**: `ApiClient` (query / callEndpoint) уже есть — расширяем декларативными операциями/формами из раздела «Инфраструктура»; формы `submit.target` уже поддержан (type form.ts).
- **Контент-биндинги**: `merged()`-механизм и `AssetResolver` не меняются.
- **Контракт/типы**: расширяем `descriptor.ts` новыми полями (siteSettings, page list, routeGroups) — правки в `docs/ui-runtime/json-descriptors.md`.

---

## 4. Влияние на сборку (build)

- Materializer почти без изменений: определения компонентов собираются по **используемым page.list** (перечисляем `componentId` элементов страниц снапшота вместо обхода дерева).
- `entry.tsx` генерится как сейчас (register + boot). Метаданные site-settings (title/favicon/og) подмешиваются в `index.html` и контракт.
- Возможность будущей постраничной раздачи (per-page chunks, см. TODO.md Этап 5): линейная модель листа подходит напрямую — чанк страницы = её элементы.

---

## 5. Что удаляем (R17)

- Дерево-модель: `ComponentNode`, `tree-utils`, tree-операции/таблицы.
- Старый редактор: `EditorPage`, `TreePanel`, `Inspector` (дерево-интерфейсы), дерево-превью флоу.
- `page-store`/`preview-store` в старом виде (заменяются новыми slice-сторами разделов).
- Старые данные страниц/деревьев — сбрасываются (прода нет; конвертеры не нужны).

---

## 6. Риски и открытые вопросы

1. **tiptap как code-редактор TSX**: подсветка+верные отступы+вставка — нетривиально; подбираем tiptap-extension code-block либо инкапсулируем код в скрытый компонент с синхронизацией (решение на слайсе R5).
2. **Live-валидация TSX в браузере**: размер вендор-чанка (TS-транспилер). Вариант — лёгкий транспилер + линт-эвристики, полная проверка типов на бэке при сборке. Решение на слайсе R5.
3. **Route-группы и priority** при «страница = лист»: совместимость со сквозным кэшем маршрутов (client edge) — проверяем на R7/R8.
4. **Инфраструктура как JSON-модель**: перенос дескрипторов из статики контракта в CRUD — синхронизация deployed-контракта с изменениями (решение на R6/R11).
5. **Постраничная раздача** (TODO Этап 5) увязывается с новой линейной моделью — не блокирует, но учитываем при проектировании чанков на R7.