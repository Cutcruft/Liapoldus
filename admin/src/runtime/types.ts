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
  createdAt?: string;
};

/** Статус рантайма сайта (Этап 4: /runtime/status — ожидается на бэкенде). */
export type RuntimeStatus = {
  siteId: string;
  version: number;
  onClientReady: boolean;
};