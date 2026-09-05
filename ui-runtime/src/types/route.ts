import type { RouteDescriptor } from './descriptor';

export interface RouteMatch {
  route: RouteDescriptor;
  params: Record<string, string>;
}

export interface RouteContext {
  path: string;
  params: Record<string, string>;
}

export interface HttpResponse {
  status: number;
  contentType: string;
  body: unknown;
}

export type HttpMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';

/** Ссылка на ассет из payload-дескриптора ассета. */
export interface AssetRef {
  assetId: string;
  path?: string;
}

export type ViewDestination =
  | { name: string; params: Record<string, string> }
  | { external: string };

export interface ServeAssetResult {
  path: string;
  contentType: string;
  source: 'asset' | 'static' | 'fallback';
}