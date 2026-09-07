/** Доменные типы admin-рантайма. Формы соответствуют docs/api-admin.md. */

export interface AdminApiError {
  kind: 'http' | 'transport' | 'validation';
  status?: number;
  message: string;
}

/** Нормализованный Result операции (не зависит от транспорта). */
export type OperationStatus = 'pending' | 'success' | 'error';

export interface OperationResult {
  ok: boolean;
  status: OperationStatus;
  /** Локализованная подпись операции (например "Создать сайт"). */
  label: string;
  /** Короткое описание результата (количество, тип, id). */
  detail: string;
  data?: unknown;
  error?: AdminApiError;
}

export type Site = {
  id: string;
  name: string;
  slug: string;
  defaultLocale: string;
  hosts: string[];
  createdAt?: string;
};

/** Источник значения свойства (literal = хранится как есть). */
export type BindingSource =
  | { source: 'literal' }
  | { source: 'content' | 'route' | 'query' | 'operation' | 'form'; path: string };

export type ComponentNode = {
  id: string;
  type: string;
  props?: Record<string, unknown>;
  bindings?: Record<string, BindingSource>;
  children?: ComponentNode[];
};

export type Page = {
  id: string;
  siteId: string;
  name: string;
  slug: string;
  root: ComponentNode;
  version: number;
  createdAt?: string;
};

export type PageVersion = {
  versionId: string;
  version: number;
  root: ComponentNode;
  createdAt?: string;
};

export type ContentSummary = {
  id: string;
  collectionId: string;
  fields: Record<string, unknown>;
};

export type ContentDetail = ContentSummary & {
  translations?: Record<string, { fields: Record<string, unknown> }>;
  createdAt?: string;
  updatedAt?: string;
};

export type ContentLocaleStat = {
  locale: string;
  contentCount: number;
};

/** Сводка локалей сайта (R3): базовый язык + локали в переводе + итоги. */
export type SiteLocales = {
  baseLocale: string;
  locales: ContentLocaleStat[];
  total: number;
};

export type AssetVariant = { name: string; url: string; mime: string; size: number };

export type AssetMeta = {
  id: string;
  siteId: string;
  name: string;
  mime: string;
  size: number;
  variants: AssetVariant[];
  etag?: string;
  createdAt?: string;
};

/** Ссылка на контент, использующий ассет (R4: ответ `GET /assets/{id}/usage`). */
export type AssetUsageContent = { id: string; collectionId: string };

/** Ссылка на форму, использующую ассет в определении. */
export type AssetUsageForm = { id: string; name: string };

/** «Где используется ассет» — контент (поля/галереи/переводы) и формы (definitions). */
export type AssetUsage = {
  assetId: string;
  contents: AssetUsageContent[];
  forms: AssetUsageForm[];
};

export type RouteAction =
  | { type: 'renderPage'; pageId: string }
  | { type: 'serveAsset'; assetId: string }
  | { type: 'redirect'; target: string; status?: number; keepQuery?: boolean };

export type Route = {
  id: string;
  siteId: string;
  matcher: string;
  priority: number;
  action: RouteAction;
  createdAt?: string;
};

export type FormDefinition = {
  id: string;
  fields?: Array<{ name: string; type: string; [k: string]: unknown }>;
  submit?: { endpoint?: string; [k: string]: unknown };
  [k: string]: unknown;
};

export type AdminForm = {
  id: string;
  siteId: string;
  name: string;
  definition: FormDefinition;
  createdAt?: string;
};

export type Submission = {
  id: string;
  formId: string;
  siteId: string;
  payload: Record<string, unknown>;
  createdAt: string;
};

export type Snapshot = {
  id: string;
  siteId: string;
  name: string;
  gitSha?: string;
  createdAt?: string;
};

export type BuildStatus = 'queued' | 'building' | 'ready' | 'failed';

export type Build = {
  id: string;
  siteId: string;
  snapshotId: string;
  environment: string;
  status: BuildStatus;
  log: string[];
  artifactDir: string;
  createdAt: string;
  startedAt?: string;
  finishedAt?: string;
};

/** Событие dev-ребортера (client `/dev/build/ws`). */
export type DevRebuildEvent = {
  siteId: string;
  environment: string;
  snapshotId: string;
  artifactDir?: string;
  status: BuildStatus;
  error?: string;
  updatedAt: string;
};

/** Событие жизненного цикла сборки (admin `/api/builds/ws`, M3). */
export type BuildEvent = {
  siteId: string;
  environment: string;
  snapshotId: string;
  artifactDir?: string;
  status: BuildStatus;
  error?: string;
  updatedAt: string;
};

/** Статус рантайма сайта (Этап 4: /runtime/status — ожидается на бэкенде). */
export type RuntimeStatus = {
  siteId: string;
  version: number;
  onClientReady: boolean;
};

/** Один коммит в истории сайта (git). */
export type GitCommitInfo = {
  sha: string;
  message: string;
  author: string;
  time: string;
};

/** Головная позиция ветки site-git. */
export type GitBranchStatus = {
  existing: boolean;
  sha?: string;
  head?: GitCommitInfo | null;
};

/** Статус site-git: ветки dev/main + признак «БД отличается от dev HEAD». */
export type GitStatus = {
  siteId: string;
  branches: string[];
  dev: GitBranchStatus;
  main: GitBranchStatus;
  dirty: boolean;
};

/** Ответ `GET /api/sites/{siteId}/git`: статус + история dev. */
export type GitOverview = {
  status: GitStatus;
  commits: GitCommitInfo[];
};

/** Один сайт в ответе дашборда: базовая модель + полный git-статус. */
export type DashboardSite = {
  siteId: string;
  name: string;
  slug: string;
  defaultLocale: string;
  hosts: string[];
  git?: GitStatus;
};

/** Строка «последние сборки» дашборда (агрегирована по всем сайтам). */
export type RecentBuild = {
  id: string;
  siteId: string;
  siteName: string;
  snapshotId: string;
  environment: string;
  status: BuildStatus;
  createdAt: string;
};

/** Строка «последние снапшоты» дашборда (агрегирована по всем сайтам). */
export type RecentSnapshot = {
  id: string;
  siteId: string;
  siteName: string;
  name: string;
  gitSha?: string;
  createdAt: string;
};

/** Ответ `GET /api/dashboard` (слайс 2.2). */
export type Dashboard = {
  siteCount: number;
  sites: DashboardSite[];
  recentBuilds: RecentBuild[];
  recentSnapshots: RecentSnapshot[];
  runtimeStatus: 'ok' | 'degraded' | 'no-builds' | string;
};

/** Ответ `GET /api/settings` (слайс 2.3); adminToken приходит замаскированным. */
export type Settings = {
  adminToken: string;
  defaultLocale: string;
  redirectDefaultStatus: number;
};

/** Режим цвета токена: светлый/тёмный. */
export type ColorMode = 'light' | 'dark';

/** Один цветовой токен: роль + значение для каждого режима. */
export type ColorToken = {
  name: string;
  value: Record<ColorMode, string>;
};

/** Имена скалярных групп токенов (порядок = порядок в UI/JSON). */
export const TOKEN_GROUPS = [
  'typography',
  'spacing',
  'shadows',
  'borders',
  'breakpoints',
  'zIndex',
  'opacity',
  'transitions',
  'custom',
] as const;

export type TokenGroup = (typeof TOKEN_GROUPS)[number];

/**
 * Набор дизайн-токенов сайта (полная замена через GET/PUT /api/sites/{id}/tokens).
 * colors всегда массив (может быть пустым); скалярные группы — опциональные
 * map «имя токена → CSS-значение» ("2rem", "0 2px 4px rgba(0,0,0,.1)").
 */
export type TokenSet = {
  colors: ColorToken[];
  typography?: Record<string, string>;
  spacing?: Record<string, string>;
  shadows?: Record<string, string>;
  borders?: Record<string, string>;
  breakpoints?: Record<string, string>;
  zIndex?: Record<string, string>;
  opacity?: Record<string, string>;
  transitions?: Record<string, string>;
  custom?: Record<string, string>;
};

/** Объявленная зависимость сайта (GET /dependencies). */
export type Dependency = {
  name: string;
  spec: string;
  resolvedVersion?: string;
};

/**
 * Один замороженный инстанс графа зависимостей (как в depsLock снапшота).
 * Один пакет может встречаться несколько раз (вложенный layout, spec §5):
 * hoisted-инстанс живёт в node_modules/<name>, остальные — вложены под каждый
 * parent из requestedBy (ключ инстанса "name@version"; "site" = объявлен в топе).
 */
export type LockedDep = {
  name: string;
  spec: string;
  version: string;
  integrity?: string;
  hoisted?: boolean;
  requestedBy?: string[];
  peerDependencies?: Record<string, string>;
  peerDependenciesMeta?: Record<string, boolean>;
};

/** Ответ POST /dependencies/resolve — полный граф текущего набора сайта. */
export type ResolveResult = {
  deps: LockedDep[];
};

/** Per-site лимит кэша тарболов зависимостей (spec §11, channel 7). 0 = не задан. */
export type SiteCacheConfig = {
  maxDepsBytes: number;
};

/** Результат ручной очистки кэша (POST /cache-config/evict). */
export type EvictResponse = {
  evicted: number;
  evictedBytes: number;
};

/** Строка реестра компонентов (R5, `GET /sites/{id}/components/registry`). */
export type RegistryComponent = {
  id: string;
  name: string;
  kind: string;
  currentSha?: string;
  /** false = исходник отличается от dev-HEAD (или git ещё нет) → «грязно». */
  committed: boolean;
  usageCount: number;
  updatedAt: string;
};

/** Ответ реестра: dev-HEAD + все определения сайта. */
export type ComponentRegistry = {
  devSha?: string;
  components: RegistryComponent[];
};

/** Полное определение компонента (registry-запись + source). */
export type AdminComponent = {
  id: string;
  siteId: string;
  name: string;
  kind: string;
  source: string;
  schema: Record<string, unknown>;
  metadata?: Record<string, unknown>;
  currentSha?: string;
  committed?: boolean;
  createdAt: string;
  updatedAt: string;
};

/** Один коммит истории компонента: sha + сообщение + исходник на тот момент. */
export type ComponentHistoryEntry = {
  sha: string;
  message: string;
  time: string;
  source: string;
};

/** Страница, использующая компонент (R5, `GET /components/{id}/usage`). */
export type ComponentUsagePage = {
  id: string;
  name: string;
  count: number;
};

export type ComponentUsage = {
  componentId: string;
  pages: ComponentUsagePage[];
};

/** Ответ `POST /components/{id}/commit`: sha + число отмеченных определений. */
export type ComponentCommitResult = {
  sha: string;
  count: number;
};

/** Выведенная из TSX проп (панель Schema, R5 — авто-эвристика). */
export type InferredProp = {
  name: string;
  type: string;
  required: boolean;
  default?: string;
};

/** Управляемый дескриптор операции (R6, `GET /sites/{id}/operations`). */
export type AdminOperation = {
  id: string;
  siteId: string;
  /** Системные строки (siteId пустой у исходника) редактировать нельзя. */
  system: boolean;
  provider: string;
  typeOp: 'query' | 'mutation';
  method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  path: string;
  cache: 'immutable' | 'disabled' | 'ttl';
  ttl?: number;
  scope?: string;
  resultType?: string;
  params?: Record<string, unknown>;
  poll?: Record<string, unknown>;
  subscribe?: Record<string, unknown>;
  createdAt?: string;
  updatedAt?: string;
};

/** Управляемый дескриптор эндпоинта (R6, `GET /sites/{id}/endpoints`). */
export type AdminEndpoint = {
  id: string;
  siteId: string;
  system: boolean;
  method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  path: string;
  operationId: string;
  createdAt?: string;
  updatedAt?: string;
};

/** Форма дескриптора операции для предпросмотра вызова. */
export type OperationParamSpec = {
  in: 'query' | 'path' | 'body';
  fields?: Record<string, { required?: boolean }>;
};