import type { RedirectStatus, ResolvedRoute } from '../types/descriptor';
import { applyTargetTemplate, appendQuery, splitPath } from './matcher';
import type { MatchFunction } from './matcher';

export interface HttpRequest {
  method: string;
  path: string;
  headers?: Record<string, string>;
}

export interface AssetRecord {
  assetId: string;
  bytes: string;
  mime: string;
  etag?: string;
}

export interface HttpResponse {
  status: number;
  headers: Record<string, string>;
  /** тело при renderPage/serveAsset */
  body?: string;
  location?: string;
}

export interface RequestRouterDeps {
  /** та же функция матчинга, что у клиентского Router (§12a#7) */
  match: MatchFunction;
  /** единственная HTML-оболочка для всех renderPage-роутов (§12a#8) */
  htmlShell: string;
  assets: Record<string, AssetRecord> | ((assetId: string) => AssetRecord | undefined);
  cacheHeaders?: Record<string, string>;
  htmlCacheHeaders?: Record<string, string>;
}

const DEFAULT_ASSET_CACHE = { 'cache-control': 'public, max-age=31536000, immutable' };
const DEFAULT_HTML_CACHE = { 'cache-control': 'public, max-age=60' };

/** Edge-адаптер: тот же набор роутов применяется к входящему HTTP-запросу (§12a). */
export class RequestRouter {
  constructor(private deps: RequestRouterDeps) {}

  match(path: string): ResolvedRoute | null {
    return this.deps.match(path);
  }

  handle(req: HttpRequest): HttpResponse {
    const resolved = this.deps.match(req.path);
    if (!resolved) return { status: 404, headers: {} };

    const action = resolved.route.action;
    if (action.type === 'renderPage') {
      return {
        status: 200,
        headers: { 'content-type': 'text/html', ...this.deps.htmlCacheHeaders, ...DEFAULT_HTML_CACHE },
        body: this.deps.htmlShell,
      };
    }
    if (action.type === 'serveAsset') {
      const asset = this.lookup(action.assetId);
      if (!asset) return { status: 404, headers: {} };
      return {
        status: 200,
        headers: {
          'content-type': asset.mime,
          ...(asset.etag ? { etag: asset.etag } : {}),
          ...this.deps.cacheHeaders,
          ...DEFAULT_ASSET_CACHE,
        },
        body: asset.bytes,
      };
    }
    const { pathname, query } = splitPath(req.path);
    const target = applyTargetTemplate(action.target, resolved.route.matcher, pathname);
    const location = action.keepQuery ? appendQuery(target, query) : target;
    const status: RedirectStatus = action.status ?? 301;
    return { status, headers: { location }, location };
  }

  private lookup(assetId: string): AssetRecord | undefined {
    if (typeof this.deps.assets === 'function') return this.deps.assets(assetId);
    return this.deps.assets[assetId];
  }
}