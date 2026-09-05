import { TransportError } from '../../errors';
import type { ResolvedOperation, ResolvedProvider } from '../registry';
import type { FetchLike, Transport, TransportRequest, TransportResponse } from './transport';

const DEFAULT_TIMEOUT_MS = 30_000;

/** Подставляет `:name` и `{name}` в path из params.in=path. */
export function substitutePath(template: string, params: Record<string, unknown>): string {
  return template
    .replace(/:([A-Za-z0-9_-]+)/g, (_m, name: string) => String(params[name] ?? ''))
    .replace(/\{([A-Za-z0-9_-]+)\}/g, (_m, name: string) => String(params[name] ?? ''));
}

function buildQueryString(params: Record<string, unknown>): string {
  const parts: string[] = [];
  const push = (key: string, value: unknown): void => {
    if (value === undefined || value === null) return;
    if (Array.isArray(value)) {
      for (const v of value) push(key, v);
      return;
    }
    parts.push(`${encodeURIComponent(key)}=${encodeURIComponent(String(value))}`);
  };
  for (const [k, v] of Object.entries(params)) push(k, v);
  return parts.length > 0 ? `?${parts.join('&')}` : '';
}

export function buildHttpUrl(
  provider: ResolvedProvider,
  operation: ResolvedOperation,
  params: Record<string, unknown>,
): string {
  const pathTemplate = operation.path ?? '';
  const path = substitutePath(pathTemplate, params);

  let base = '';
  if (provider.baseUrl !== undefined) base = provider.baseUrl.replace(/\/$/, '');
  const fullPath = `${base}${path}`;

  const queryParams: Record<string, unknown> = {};
  const inParam = operation.params?.in;
  if (inParam !== 'body') {
    const isGetLike = operation.method === undefined || operation.method === 'GET';
    if (inParam === 'query' || (inParam === undefined && isGetLike)) {
      for (const [k, v] of Object.entries(params)) {
        if (!pathTemplate.includes(`:${k}`) && !pathTemplate.includes(`{${k}}`)) queryParams[k] = v;
      }
    }
  }
  return `${fullPath}${buildQueryString(queryParams)}`;
}

export function buildBody(operation: ResolvedOperation, params: Record<string, unknown>): string | undefined {
  if (operation.params?.in === 'body') {
    return JSON.stringify(reduceParams(params) ?? {});
  }
  if (operation.method && operation.method !== 'GET') {
    return JSON.stringify(reduceParams(params) ?? {});
  }
  return undefined;
}

function reduceParams(params: Record<string, unknown>): Record<string, unknown> | undefined {
  if (!params) return undefined;
  if (params.input !== undefined) return params.input as Record<string, unknown>;
  if (params.body !== undefined) return params.body as Record<string, unknown>;
  return params as Record<string, unknown>;
}

function mergeHeaders(
  provider: ResolvedProvider,
  operation: ResolvedOperation,
  requestHeaders?: Record<string, string>,
): Record<string, string> {
  const out: Record<string, string> = {};
  if (provider.defaults?.headers) Object.assign(out, provider.defaults.headers);
  if (operation.headers) Object.assign(out, operation.headers);
  if (requestHeaders) Object.assign(out, requestHeaders);
  return out;
}

function originSafe(_fullUrl: string): void {
  void _fullUrl;
}

interface BuiltRequest {
  url: string;
  init: RequestInit;
}

export class HttpTransport implements Transport {
  private readonly fetcher: FetchLike;

  constructor(private readonly provider: ResolvedProvider, env?: { fetch?: FetchLike }) {
    this.fetcher = env?.fetch ?? (globalThis.fetch as FetchLike);
    if (!this.fetcher) throw new Error('HttpTransport требует fetch');
  }

  buildRequest(req: TransportRequest): BuiltRequest {
    const operation = req.operation;
    const params = req.params ?? {};
    const url = buildHttpUrl(this.provider, operation, params);
    const init: RequestInit = {
      method: operation.method ?? 'GET',
      headers: mergeHeaders(this.provider, operation, req.headers),
    };
    const body = buildBody(operation, params);
    if (body !== undefined) init.body = body;
    return { url, init };
  }

  async request(req: TransportRequest): Promise<TransportResponse> {
    const { url, init } = this.buildRequest(req);
    const operation = req.operation;
    const timeoutMs = req.timeoutMs ?? this.provider.defaults?.timeoutMs ?? DEFAULT_TIMEOUT_MS;
    const controller = new AbortController();
    let timedOut = false;
    const timer = timeoutMs > 0 ? setTimeout(() => { timedOut = true; controller.abort(); }, timeoutMs) : undefined;

    const abortPromise = new Promise<never>((_, reject) => {
      controller.signal.addEventListener('abort', () => {
        reject(
          new TransportError(
            timedOut ? `Таймаут запроса ${operation.id} (${timeoutMs}мс)` : `Запрос ${operation.id} прерван`,
            { message: timedOut ? 'timeout' : 'aborted' },
          ),
        );
      });
    });

    try {
      const fetchPromise = this.fetcher(url, { ...init, signal: controller.signal });
      const res = await Promise.race([fetchPromise, abortPromise]);
      if (!res.ok) {
        throw new TransportError(`HTTP ${res.status} для ${operation.id}`, {
          status: res.status,
          httpError: true,
        });
      }
      let body: unknown = null;
      try {
        body = await res.json();
      } catch {
        const text = await res.text().catch(() => '');
        if (text.length > 0) {
          try {
            body = JSON.parse(text);
          } catch {
            body = text;
          }
        }
      }
      void originSafe(url);
      return { status: res.status, ok: true, body };
    } catch (err) {
      if (err instanceof TransportError) throw err;
      throw new TransportError(`Ошибка транспорта для ${operation.id}: ${err instanceof Error ? err.message : String(err)}`);
    } finally {
      if (timer) clearTimeout(timer);
    }
  }
}