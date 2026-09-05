import { boot, type BootRuntime } from '../../src/core/boot';
import type { RouteDescriptor, ThemeDescriptor } from '../../src/types/descriptor';
import type { TreeDeclaration } from '../../src/types/tree';
import { FakeWebSocket, makeFakeFetch, type FetchCall } from './helpers';

export function routeHome(): RouteDescriptor {
  return {
    id: 'home',
    matcher: '^/(|new)$',
    priority: 0,
    action: { type: 'renderPage', pageId: 'page.home' },
  };
}

export function routeArticle(): RouteDescriptor {
  return {
    id: 'article',
    matcher: '^/articles/(?<articleId>[0-9]+)$',
    priority: 10,
    action: { type: 'renderPage', pageId: 'page.article' },
  };
}

export function routeRedirect(): RouteDescriptor {
  return {
    id: 'legacy',
    matcher: '^/old$',
    priority: 5,
    action: { type: 'redirect', target: '/new' },
  };
}

export function themeDefault(): ThemeDescriptor {
  return {
    themeId: 'default',
    tokens: { '--color-primary': '#111111', '--color-bg': '#ffffff' },
  };
}

export function feedOp(poll = true): Record<string, unknown> {
  return {
    id: 'feed.list',
    typeOp: 'query',
    providerId: 'cms',
    method: 'GET',
    path: '/api/sites/{siteId}/feeds',
    params: { in: 'query', fields: { siteId: { required: true }, locale: { required: false }, page: { required: false } } },
    cache: 'disabled',
    type: 'content[]',
    ...(poll ? { poll: { schedule: '* * * * * *', cache: true } } : {}),
  };
}

export interface ContractOverrides {
  operations?: unknown[];
  tree?: TreeDeclaration;
  dev?: boolean;
  locale?: string;
  version?: string;
  routes?: RouteDescriptor[];
}

export function buildContract(overrides?: ContractOverrides): Record<string, unknown> {
  return {
    siteId: 'acme',
    environment: 'prod',
    version: overrides?.version ?? 'v1',
    locale: overrides?.locale ?? 'ru',
    providers: [{ id: 'cms', protocol: 'http', baseUrl: 'http://cms.test' }],
    operations: overrides?.operations ?? [feedOp()],
    endpoints: [],
    routes: overrides?.routes ?? [routeHome(), routeArticle()],
    themes: [themeDefault()],
    enabledChannels: { ws: true, sse: true },
    capabilities: { formSubmissions: true, dev: overrides?.dev ?? false },
    ...(overrides?.tree ? { tree: overrides.tree } : {}),
  };
}

export interface BootFromContractResult {
  runtime: BootRuntime;
  fetch: ReturnType<typeof makeFakeFetch>['fetch'];
  calls: FetchCall[];
}

/** Счётчик уникальных siteId: boot кэширует сессии по siteId, а тестам нужен свежий рантайм. */
let bootSeq = 0;

/** boot из inline-контракта + фейковый fetch (общий харнесс для React-слоя). */
export async function bootFromContract(
  overrides?: ContractOverrides,
  handler?: (call: FetchCall) => unknown | Error | Promise<unknown> | Promise<Error>,
): Promise<BootFromContractResult> {
  bootSeq += 1;
  const siteId = `site-${bootSeq}`;
  const { fetch, calls } = makeFakeFetch((call) => {
    if (handler) return handler(call);
    return buildContract(overrides);
  });
  const runtime = await boot(siteId, 'production', {
    env: {
      fetch,
      navigatorLanguage: overrides?.locale ?? 'ru',
      WebSocket: FakeWebSocket,
      storage: {"getItem":()=>null,"setItem":()=>undefined,"removeItem":()=>undefined} as unknown as Storage,
    },
  });
  return { runtime, fetch, calls };
}

export async function until(fn: () => boolean, ms = 1000): Promise<void> {
  const start = Date.now();
  while (!fn()) {
    if (Date.now() - start > ms) throw new Error('until(): таймаут');
    await new Promise((r) => setTimeout(r, 5));
  }
}