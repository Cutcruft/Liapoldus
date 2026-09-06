import type { AdminApi } from './api';
import type { Translate } from './i18n';
import type { OperationResult } from './types';
import type { AdminStringKey } from './strings';

export type OperationKind =
  | 'listSites'
  | 'createSite'
  | 'getSite'
  | 'updateSite'
  | 'deleteSite'
  | 'createPage'
  | 'listPages'
  | 'getPage'
  | 'saveTree'
  | 'deletePage'
  | 'listContents'
  | 'getContent'
  | 'createContent'
  | 'updateContent'
  | 'deleteContent'
  | 'putTranslation'
  | 'deleteTranslation'
  | 'listAssets'
  | 'uploadAsset'
  | 'getAsset'
  | 'deleteAsset'
  | 'listRoutes'
  | 'createRoute'
  | 'updateRoute'
  | 'deleteRoute'
  | 'listForms'
  | 'createForm'
  | 'getForm'
  | 'updateForm'
  | 'deleteForm'
  | 'listSubmissions'
  | 'createSnapshot'
  | 'listSnapshots'
  | 'deleteSnapshot'
  | 'createBuild'
  | 'getBuild'
  | 'runtimeStatus';

export interface OperationSpec<TArgs extends Record<string, unknown> = Record<string, unknown>> {
  kind: OperationKind;
  labelKey: AdminStringKey;
  method: 'GET' | 'POST' | 'PUT' | 'DELETE';
  /** Шаблон пути; `:param`/`{param}` подставляются из args. */
  route: (args: TArgs) => string;
  body?: (args: TArgs) => unknown;
  detail: (body: unknown, t: Translate) => string;
}

/** Реестр операций admin API (каждая строка = один пункт исполнителя). */
export const OPERATIONS: Record<OperationKind, OperationSpec> = {
  listSites: {
    kind: 'listSites',
    labelKey: 'op.listSites',
    method: 'GET',
    route: () => '/api/sites',
    detail: (b, t) => t('result.count', { n: Array.isArray(b) ? b.length : 0 }),
  },

  createSite: {
    kind: 'createSite',
    labelKey: 'op.createSite',
    method: 'POST',
    route: () => '/api/sites',
    body: (args) => ({
      name: String(args.name),
      slug: String(args.slug),
      defaultLocale: String(args.defaultLocale ?? 'ru'),
      hosts: Array.isArray(args.hosts) ? args.hosts : [],
    }),
    detail: (b, t) => {
      const name = typeof b === 'object' && b !== null && 'name' in b ? String((b as { name: unknown }).name) : '';
      return t('result.site.created', { name });
    },
  },

  getSite: {
    kind: 'getSite',
    labelKey: 'op.getSite',
    method: 'GET',
    route: (args) => `/api/sites/{siteId}`,
    detail: (b, t) => {
      const name = typeof b === 'object' && b !== null && 'name' in b ? String((b as { name: unknown }).name) : '';
      return name || t('result.count', { n: 0 });
    },
  },

  updateSite: {
    kind: 'updateSite',
    labelKey: 'op.updateSite',
    method: 'PUT',
    route: (args) => `/api/sites/{siteId}`,
    body: (args) => args.patch ?? {},
    detail: () => 'OK',
  },

  deleteSite: {
    kind: 'deleteSite',
    labelKey: 'op.deleteSite',
    method: 'DELETE',
    route: (args) => `/api/sites/{siteId}`,
    detail: (b, t) => t('result.site.deleted', { name: t('site.title') }),
  },

  createPage: {
    kind: 'createPage',
    labelKey: 'op.createPage',
    method: 'POST',
    route: (args) => `/api/sites/{siteId}/pages`,
    body: (args) => ({
      name: String(args.name),
      slug: String(args.slug),
      root: (args.root as { id: string; type: string }) ?? { id: 'root', type: 'Container', children: [] },
    }),
    detail: (b, t) => {
      const version = typeof b === 'object' && b !== null && 'version' in b ? (b as { version: unknown }).version : 1;
      return t('result.saved.v', { version: Number(version) });
    },
  },

  listPages: {
    kind: 'listPages',
    labelKey: 'op.listPages',
    method: 'GET',
    route: (args) => `/api/sites/{siteId}/pages`,
    detail: (b, t) => t('result.count', { n: Array.isArray(b) ? b.length : 0 }),
  },

  getPage: {
    kind: 'getPage',
    labelKey: 'op.getPage',
    method: 'GET',
    route: (args) => `/api/pages/{pageId}`,
    detail: (b, t) => {
      const name = typeof b === 'object' && b !== null && 'name' in b ? String((b as { name: unknown }).name) : '';
      return name || t('result.count', { n: 0 });
    },
  },

  saveTree: {
    kind: 'saveTree',
    labelKey: 'op.saveTree',
    method: 'PUT',
    route: (args) => `/api/pages/{pageId}/tree`,
    body: (args) => ({ root: args.root }),
    detail: (b, t) => {
      const version = typeof b === 'object' && b !== null && 'version' in b ? (b as { version: unknown }).version : 0;
      return t('result.saved.v', { version: Number(version) });
    },
  },

  deletePage: {
    kind: 'deletePage',
    labelKey: 'op.deletePage',
    method: 'DELETE',
    route: (args) => `/api/pages/{pageId}`,
    detail: () => 'OK',
  },

  listContents: {
    kind: 'listContents',
    labelKey: 'op.listContents',
    method: 'GET',
    route: (args) => `/api/sites/{siteId}/contents`,
    detail: (b, t) => t('result.count', { n: Array.isArray(b) ? b.length : 0 }),
  },

  getContent: {
    kind: 'getContent',
    labelKey: 'op.getContent',
    method: 'GET',
    route: (args) => `/api/sites/{siteId}/contents/{contentId}`,
    detail: (b, t) => {
      const name = typeof b === 'object' && b !== null && 'name' in b ? String((b as { name: unknown }).name) : '';
      return name || t('result.count', { n: 0 });
    },
  },

  createContent: {
    kind: 'createContent',
    labelKey: 'op.createContent',
    method: 'POST',
    route: (args) => `/api/sites/{siteId}/contents`,
    body: (args) => {
      const body: Record<string, unknown> = { collectionId: args.collectionId, fields: args.fields ?? {} };
      if (args.id) body.id = args.id;
      return body;
    },
    detail: () => 'OK',
  },

  updateContent: {
    kind: 'updateContent',
    labelKey: 'op.updateContent',
    method: 'PUT',
    route: (args) => `/api/sites/{siteId}/contents/{contentId}`,
    body: (args) => ({ fields: args.fields }),
    detail: () => 'OK',
  },

  deleteContent: {
    kind: 'deleteContent',
    labelKey: 'op.deleteContent',
    method: 'DELETE',
    route: (args) => `/api/sites/{siteId}/contents/{contentId}`,
    detail: () => 'OK',
  },

  putTranslation: {
    kind: 'putTranslation',
    labelKey: 'op.putTranslation',
    method: 'PUT',
    route: (args) => `/api/sites/{siteId}/contents/{contentId}/translations/{locale}`,
    body: (args) => ({ fields: args.fields }),
    detail: () => 'OK',
  },

  deleteTranslation: {
    kind: 'deleteTranslation',
    labelKey: 'op.deleteTranslation',
    method: 'DELETE',
    route: (args) => `/api/sites/{siteId}/contents/{contentId}/translations/{locale}`,
    detail: () => 'OK',
  },

  listAssets: {
    kind: 'listAssets',
    labelKey: 'op.listAssets',
    method: 'GET',
    route: (args) => `/api/sites/{siteId}/assets`,
    detail: (b, t) => t('result.count', { n: Array.isArray(b) ? b.length : 0 }),
  },

  uploadAsset: {
    kind: 'uploadAsset',
    labelKey: 'op.uploadAsset',
    method: 'POST',
    route: (args) => `/api/sites/{siteId}/assets`,
    body: (args) => args.form as FormData | undefined,
    detail: (b) => (typeof b === 'object' && b !== null && 'name' in b ? String((b as { name: unknown }).name) : 'OK'),
  },

  getAsset: {
    kind: 'getAsset',
    labelKey: 'op.getAsset',
    method: 'GET',
    route: (args) => `/api/assets/{assetId}`,
    detail: (b) =>
      typeof b === 'object' && b !== null && 'name' in b ? String((b as { name: unknown }).name) : '',
  },

  deleteAsset: {
    kind: 'deleteAsset',
    labelKey: 'op.deleteAsset',
    method: 'DELETE',
    route: (args) => `/api/assets/{assetId}`,
    detail: () => 'OK',
  },

  listRoutes: {
    kind: 'listRoutes',
    labelKey: 'op.listRoutes',
    method: 'GET',
    route: (args) => `/api/sites/{siteId}/routes`,
    detail: (b, t) => t('result.count', { n: Array.isArray(b) ? b.length : 0 }),
  },

  createRoute: {
    kind: 'createRoute',
    labelKey: 'op.createRoute',
    method: 'POST',
    route: (args) => `/api/sites/{siteId}/routes`,
    body: (args) => ({ matcher: args.matcher, priority: Number(args.priority), action: args.action }),
    detail: () => 'OK',
  },

  updateRoute: {
    kind: 'updateRoute',
    labelKey: 'op.updateRoute',
    method: 'PUT',
    route: (args) => `/api/sites/{siteId}/routes/{routeId}`,
    body: (args) => args.patch ?? {},
    detail: () => 'OK',
  },

  deleteRoute: {
    kind: 'deleteRoute',
    labelKey: 'op.deleteRoute',
    method: 'DELETE',
    route: (args) => `/api/sites/{siteId}/routes/{routeId}`,
    detail: () => 'OK',
  },

  listForms: {
    kind: 'listForms',
    labelKey: 'op.listForms',
    method: 'GET',
    route: (args) => `/api/sites/{siteId}/forms`,
    detail: (b, t) => t('result.count', { n: Array.isArray(b) ? b.length : 0 }),
  },

  createForm: {
    kind: 'createForm',
    labelKey: 'op.createForm',
    method: 'POST',
    route: (args) => `/api/sites/{siteId}/forms`,
    body: (args) => ({ name: args.name, definition: args.definition }),
    detail: () => 'OK',
  },

  getForm: {
    kind: 'getForm',
    labelKey: 'op.getForm',
    method: 'GET',
    route: (args) => `/api/sites/{siteId}/forms/{formId}`,
    detail: (b, t) => {
      const name = typeof b === 'object' && b !== null && 'name' in b ? String((b as { name: unknown }).name) : '';
      return name || t('result.count', { n: 0 });
    },
  },

  updateForm: {
    kind: 'updateForm',
    labelKey: 'op.updateForm',
    method: 'PUT',
    route: (args) => `/api/sites/{siteId}/forms/{formId}`,
    body: (args) => args.patch ?? {},
    detail: () => 'OK',
  },

  deleteForm: {
    kind: 'deleteForm',
    labelKey: 'op.deleteForm',
    method: 'DELETE',
    route: (args) => `/api/sites/{siteId}/forms/{formId}`,
    detail: () => 'OK',
  },

  listSubmissions: {
    kind: 'listSubmissions',
    labelKey: 'op.listSubmissions',
    method: 'GET',
    route: (args) => `/api/sites/{siteId}/forms/{formId}/submissions`,
    detail: (b, t) => t('result.count', { n: Array.isArray(b) ? b.length : 0 }),
  },

  createSnapshot: {
    kind: 'createSnapshot',
    labelKey: 'op.createSnapshot',
    method: 'POST',
    route: (args) => `/api/sites/{siteId}/snapshots`,
    body: (args) => ({ name: args.name }),
    detail: () => 'OK',
  },

  listSnapshots: {
    kind: 'listSnapshots',
    labelKey: 'op.listSnapshots',
    method: 'GET',
    route: (args) => `/api/sites/{siteId}/snapshots`,
    detail: (b, t) => t('result.count', { n: Array.isArray(b) ? b.length : 0 }),
  },

  deleteSnapshot: {
    kind: 'deleteSnapshot',
    labelKey: 'op.deleteSnapshot',
    method: 'DELETE',
    route: (args) => `/api/snapshots/{snapshotId}`,
    detail: () => 'OK',
  },

  createBuild: {
    kind: 'createBuild',
    labelKey: 'op.createBuild',
    method: 'POST',
    route: (args) => `/api/sites/{siteId}/builds`,
    body: (args) => ({ snapshotId: args.snapshotId, environment: args.environment }),
    detail: (b, t) => {
      const status = typeof b === 'object' && b !== null && 'status' in b ? String((b as { status: unknown }).status) : '';
      return status ? status : t('result.count', { n: 0 });
    },
  },

  getBuild: {
    kind: 'getBuild',
    labelKey: 'op.getBuild',
    method: 'GET',
    route: (args) => `/api/builds/{buildId}`,
    detail: (b, t) => {
      const status = typeof b === 'object' && b !== null && 'status' in b ? String((b as { status: unknown }).status) : '';
      return status ? status : t('result.count', { n: 0 });
    },
  },

  runtimeStatus: {
    kind: 'runtimeStatus',
    labelKey: 'op.runtimeStatus',
    method: 'GET',
    route: () => '/api/runtime/status',
    detail: (b, t) => {
      const version = typeof b === 'object' && b !== null && 'version' in b ? (b as { version: unknown }).version : undefined;
      return version === undefined ? t('result.runtime.miss') : t('result.saved.v', { version: Number(version) });
    },
  },
};

export function operationByKind(kind: OperationKind): OperationSpec {
  const spec = OPERATIONS[kind];
  if (!spec) throw new Error(`Неизвестная операция: ${kind}`);
  return spec;
}

/** Выполняет операцию: строит URL/боди, шлёт, нормирует в OperationResult. */
export async function runOperation(
  api: AdminApi,
  kind: OperationKind,
  args: Record<string, unknown>,
  t: Translate,
): Promise<OperationResult> {
  const spec = operationByKind(kind);
  const path = api.pathWithQuery(
    spec.route(args as never),
    args,
    args.query as Record<string, unknown> | undefined,
  );
  const res = await api.request(spec.method, path, spec.body?.(args));
  if (!res.ok) {
    const detail =
      kind === 'runtimeStatus' ? t('result.runtime.miss') : (res.error?.message ?? 'Ошибка');
    return {
      ok: false,
      status: 'error',
      label: t(spec.labelKey),
      detail,
      error: res.error,
    };
  }
  return {
    ok: true,
    status: 'success',
    label: t(spec.labelKey),
    detail: spec.detail(res.body, t),
    data: res.body,
  };
}