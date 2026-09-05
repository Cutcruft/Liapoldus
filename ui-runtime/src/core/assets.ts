import type { AssetMeta, AssetRef } from '../types/asset';
import type { ContentData } from '../types/content';
import type { BuiltinAssetClient } from './builtin/builtin-client';
import type { RuntimeStore } from './store';

/**
 * Резолвер ассетов (спека §7c): метаданные + варианты, кэш в `store.assets`,
 * референс `{ assetId, variant? }` → URL варианта (default 'master').
 */
export class AssetResolver {
  constructor(
    private readonly store: RuntimeStore,
    private readonly assets: BuiltinAssetClient,
  ) {}

  /** Метаданные ассета; второй вызов — из кэша `store.assets` (без запроса). */
  async get(assetId: string, opts?: { force?: boolean }): Promise<AssetMeta> {
    const cached = this.store.getState().assets[assetId];
    if (cached && !opts?.force) return cached;
    const meta = await this.assets.get(assetId);
    this.store.getState().setAssets({ ...this.store.getState().assets, [assetId]: meta });
    return meta;
  }

  /** URL варианта; default 'master'. Неизвестный assetId/вариант → undefined (не бросает). */
  url(ref: AssetRef): string | undefined {
    const meta = this.store.getState().assets[ref.assetId];
    if (!meta) return undefined;
    const variant = ref.variant ?? 'master';
    const v = meta.variants.find((x) => x.name === variant);
    return v?.url;
  }

  /** Глубокий обход: объекты вида `{ assetId, variant? }` → строка URL; иначе значение остаётся как есть. */
  resolveDeep(data: ContentData): ContentData {
    return walk(this.url.bind(this), data) as ContentData;
  }
}

function walk(urlOf: (ref: AssetRef) => string | undefined, value: unknown): unknown {
  if (Array.isArray(value)) return value.map((v) => walk(urlOf, v));
  if (value !== null && typeof value === 'object') {
    const record = value as Record<string, unknown>;
    if (typeof record.assetId === 'string') {
      const resolved = urlOf({ assetId: record.assetId, variant: typeof record.variant === 'string' ? record.variant : undefined });
      return resolved ?? value;
    }
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(record)) out[k] = walk(urlOf, v);
    return out;
  }
  return value;
}