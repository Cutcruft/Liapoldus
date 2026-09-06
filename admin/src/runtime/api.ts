import type { FetchLike } from '@liapoldus/ui-runtime';
import { substitutePath } from '@liapoldus/ui-runtime';
import type { AdminApiError } from './types';

export type HttpMethod = 'GET' | 'POST' | 'PUT' | 'DELETE';

export interface ApiResponse {
  ok: boolean;
  status: number;
  body: unknown;
  error?: AdminApiError;
}

export interface AdminApiEnv {
  /** Базовый URL admin API ('' при same-origin через vite-proxy). */
  baseUrl: string;
  getToken: () => string | null;
  fetchFn?: FetchLike;
  timeoutMs?: number;
}

const DEFAULT_TIMEOUT_MS = 30_000;

export interface AdminApi {
  request(method: HttpMethod, path: string, body?: unknown): Promise<ApiResponse>;
  /** Построение пути: подстановка `:param`/`{param}`, query — только если передан явно. */
  pathWithQuery: (
    template: string,
    params: Record<string, unknown>,
    query?: Record<string, unknown>,
  ) => string;
  /** Сырые байты (скачивание ассетов, предпросмотр). */
  fetchBytes(path: string): Promise<Response>;
}

export function createAdminApi(env: AdminApiEnv): AdminApi {
  const fetchFn: FetchLike = env.fetchFn ?? (globalThis.fetch as FetchLike);
  if (!fetchFn) throw new Error('AdminApi требует fetch');

  const pathWithQuery = (
    template: string,
    params: Record<string, unknown>,
    query?: Record<string, unknown>,
  ): string => {
    const path = substitutePath(template, params);
    const queryParts: string[] = [];
    if (query) {
      for (const [k, v] of Object.entries(query)) {
        if (v === undefined || v === null) continue;
        queryParts.push(`${encodeURIComponent(k)}=${encodeURIComponent(String(v))}`);
      }
    }
    const qs = queryParts.length > 0 ? `?${queryParts.join('&')}` : '';
    return `${path}${qs}`;
  };

  const request = async (method: HttpMethod, path: string, body?: unknown): Promise<ApiResponse> => {
    const controller = new AbortController();
    const timeoutMs = env.timeoutMs ?? DEFAULT_TIMEOUT_MS;
    let timedOut = false;
    const timer = timeoutMs > 0 ? setTimeout(() => { timedOut = true; controller.abort(); }, timeoutMs) : undefined;

    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    const token = env.getToken();
    if (token) headers.Authorization = `Bearer ${token}`;
    if (body instanceof FormData) delete headers['Content-Type'];

    const init: RequestInit = { method, headers, signal: controller.signal };
    if (body !== undefined) {
      init.body = body instanceof FormData ? body : JSON.stringify(body);
    }

    const url = `${env.baseUrl}${path}`;
    try {
      const res = await fetchFn(url, init);
      let parsed: unknown = null;
      const text = await res.text().catch(() => '');
      if (text.length > 0) {
        try {
          parsed = JSON.parse(text);
        } catch {
          parsed = text;
        }
      }
      if (!res.ok) {
        const message = typeof parsed === 'object' && parsed !== null && 'error' in (parsed as object)
          ? String((parsed as { error: unknown }).error)
          : `HTTP ${res.status}`;
        return {
          ok: false,
          status: res.status,
          body: parsed,
          error: { kind: res.status === 400 ? 'validation' : 'http', status: res.status, message },
        };
      }
      return { ok: true, status: res.status, body: parsed };
    } catch (err) {
      return {
        ok: false,
        status: 0,
        body: null,
        error: {
          kind: 'transport',
          message: timedOut ? 'Таймаут запроса' : err instanceof Error ? err.message : String(err),
        },
      };
    } finally {
      if (timer) clearTimeout(timer);
    }
  };

  const fetchBytes = async (path: string): Promise<Response> => {
    const headers: Record<string, string> = {};
    const token = env.getToken();
    if (token) headers.Authorization = `Bearer ${token}`;
    return fetchFn(`${env.baseUrl}${path}`, { headers });
  };

  return { request, fetchBytes, pathWithQuery };
}