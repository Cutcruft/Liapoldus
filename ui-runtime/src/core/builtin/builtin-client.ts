import { AssetNotFoundError, TransportError } from '../../errors';
import type { AssetMeta } from '../../types/asset';
import type { ContentData } from '../../types/content';
import type { FormDefinition } from '../../types/form';

export interface ContentGetOptions {
  locale?: string;
}

export interface LocaleOptions {
  locale?: string;
}

export interface ContentListOptions {
  collectionId?: string;
  locale?: string;
}

export interface ContentBatchOptions {
  ids: string[];
  locale?: string;
}

export interface AssetBatchOptions {
  ids: string[];
  locale?: string;
}

export interface FormSubmitPayload {
  formId: string;
  values: Record<string, unknown>;
  locale?: string;
  submittedAt?: number;
}

export type ContentListEntry = ContentData & { id: string; collectionId?: string };

export interface BuiltinContentClient {
  get(contentId: string, opts?: ContentGetOptions): Promise<ContentData>;
  list(siteId: string, opts?: ContentListOptions): Promise<ContentListEntry[]>;
  batch(siteId: string, opts: ContentBatchOptions, localeOpts?: LocaleOptions): Promise<Record<string, ContentData>>;
}

export interface BuiltinAssetClient {
  get(assetId: string): Promise<AssetMeta>;
  list(siteId: string, _opts?: { locale?: string }): Promise<AssetMeta[]>;
  batch(ids: string[], _localeOpts?: AssetBatchOptions): Promise<Record<string, AssetMeta>>;
}

export interface BuiltinFormClient {
  get(formId: string): Promise<FormDefinition>;
  submit(payload: FormSubmitPayload): Promise<{ submissionId: string; status: 'ok' }>;
}

/** Стабильный контракт делегирования (приглушает дженерики ApiClient). */
export interface BuiltinApi {
  query(operationId: string, input?: Record<string, unknown>): Promise<unknown>;
  callEndpoint(endpointId: string, input?: Record<string, unknown>): Promise<unknown>;
}

/** Типизированный доступ к builtin-операциям; делегирует в ApiClient (live-resolution через реестр). */
export class BuiltinClient {
  readonly content: BuiltinContentClient;
  readonly asset: BuiltinAssetClient;
  readonly form: BuiltinFormClient;

  constructor(private readonly api: BuiltinApi) {
    this.content = {
      get: (contentId, opts) => this.api.query('content.get', { contentId, locale: opts?.locale }) as Promise<ContentData>,
      list: (siteId, opts) =>
        this.api.query('content.list', {
          siteId,
          collectionId: opts?.collectionId,
          locale: opts?.locale,
        }) as Promise<ContentListEntry[]>,
      batch: (siteId, opts, localeOpts) =>
        this.api.query('content.batch', { siteId, ids: opts.ids, locale: localeOpts?.locale }) as Promise<
          Record<string, ContentData>
        >,
    };
    this.asset = {
      get: async (assetId) => {
        try {
          return (await this.api.query('asset.get', { assetId })) as AssetMeta;
        } catch (e) {
          if (isNotFound(e)) throw new AssetNotFoundError(`Ассет '${assetId}' не найден`);
          throw e;
        }
      },
      list: (siteId, _opts) => this.api.query('asset.list', { siteId }) as Promise<AssetMeta[]>,
      batch: (ids, _localeOpts) => this.api.query('asset.batch', { ids }) as Promise<Record<string, AssetMeta>>,
    };
    this.form = {
      get: (formId) => this.api.query('form.get', { formId }) as Promise<FormDefinition>,
      submit: (payload) =>
        this.api.callEndpoint('form.submit', {
          formId: payload.formId,
          locale: payload.locale,
          submittedAt: payload.submittedAt,
          values: payload.values,
        }) as Promise<{ submissionId: string; status: 'ok' }>,
    };
  }
}

function isNotFound(e: unknown): boolean {
  return e instanceof TransportError && e.cause?.status === 404;
}