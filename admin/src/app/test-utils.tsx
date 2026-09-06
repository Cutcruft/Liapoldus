import { cleanup, render } from '@testing-library/react';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { AdminProvider } from './admin-context';
import { appRoutes } from './AppRoutes';
import { createAdminApi, createTokenStore } from '../runtime';

export interface MockCall {
  url: string;
  method: string;
  init: RequestInit;
}

export function jsonResponse(status: number, body?: unknown): Response {
  return new Response(body === undefined ? null : JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

export interface RenderAppOptions {
  path: string;
  /** Токен для tokenStore; по умолчанию 'test-token', передайте null чтобы симулировать выход. */
  token?: string | null;
  handler?: (url: string, init: RequestInit, calls: MockCall[]) => Response | Promise<Response>;
}

const EMPTY_DASHBOARD = {
  siteCount: 0,
  sites: [],
  recentBuilds: [],
  recentSnapshots: [],
  runtimeStatus: 'no-builds',
} as const;

export async function renderApp(opts: RenderAppOptions) {
  const calls: MockCall[] = [];
  const handler = opts.handler ?? (() => jsonResponse(200, EMPTY_DASHBOARD));
  const fetchFn = async (input: string | URL | RequestInfo, init?: RequestInit) => {
    const url = String(input);
    const safeInit = init ?? {};
    calls.push({ url, method: safeInit.method ?? 'GET', init: safeInit });
    return handler(url, safeInit, calls);
  };
  const tokenStore = createTokenStore(opts.token === undefined ? 'test-token' : opts.token);
  const api = createAdminApi({ baseUrl: '', getToken: () => tokenStore.getState().token, fetchFn });
  cleanup();
  const router = createMemoryRouter(appRoutes, { initialEntries: [opts.path] });
  const utils = render(
    <AdminProvider api={api} tokenStore={tokenStore}>
      <RouterProvider router={router} />
    </AdminProvider>,
  ) as ReturnType<typeof render> & { calls: MockCall[]; api: typeof api };
  return { ...utils, calls, api, tokenStore };
}