import type { ContentData } from '../types/content';
import type { BuiltinContentClient, ContentListEntry } from './builtin/builtin-client';
import type { AssetResolver } from './assets';
import type { RuntimeStore } from './store';

export interface ContentControllerOptions {
  siteId: string;
  /** применяет resolveDeep к контенту перед записью в стор (опц.) */
  assetResolver?: AssetResolver;
}

/**
 * Контент и локализация (спека §7b): get/list/batch через builtin,
 * результат кладётся в `store.content` (ключ — contentId) уже смёрженным под локалку.
 */
export class ContentController {
  constructor(
    private readonly store: RuntimeStore,
    private readonly client: BuiltinContentClient,
    private readonly opts: ContentControllerOptions,
  ) {}

  async get(contentId: string, opts?: { locale?: string }): Promise<ContentData> {
    const data = await this.client.get(contentId, opts);
    const assets = this.opts.assetResolver;
    this.put(contentId, assets ? assets.resolveDeep(data) : data);
    return this.store.getState().content[contentId];
  }

  /** Список контента; элементы сохраняются в `store.content` (fields — уже смёрженные). */
  async list(opts?: { collectionId?: string; locale?: string }): Promise<ContentListEntry[]> {
    const entries = await this.client.list(this.opts.siteId, opts);
    const assets = this.opts.assetResolver;
    for (const entry of entries) {
      this.put(entry.id, assets ? assets.resolveDeep(entry.fields as ContentData) : entry.fields as ContentData);
    }
    return entries;
  }

  async batch(ids: string[], opts?: { locale?: string }): Promise<Record<string, ContentData>> {
    const map = await this.client.batch(this.opts.siteId, { ids }, opts);
    const assets = this.opts.assetResolver;
    for (const [id, data] of Object.entries(map)) {
      this.put(id, assets ? assets.resolveDeep(data) : data);
    }
    return map;
  }

  /** Текущий кэш контента из стора. */
  all(): Record<string, ContentData> {
    return this.store.getState().content;
  }

  private put(contentId: string, data: ContentData): void {
    this.store.getState().setContent({ ...this.store.getState().content, [contentId]: data });
  }
}