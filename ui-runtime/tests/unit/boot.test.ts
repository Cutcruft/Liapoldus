import { describe, expect, it, beforeEach } from 'vitest';
import { DescriptorValidationError, TransportError } from '../../src/errors';
import type { RouteDescriptor, ThemeDescriptor } from '../../src/types/descriptor';
import type { TreeDeclaration } from '../../src/types/tree';
import { boot } from '../../src/core/boot';
import type { FetchCall } from './helpers';
import { FakeWebSocket, makeFakeFetch, resetFakes } from './helpers';

const routeHome: RouteDescriptor = {
  id: 'home',
  matcher: '^/(|new)$',
  priority: 0,
  action: { type: 'renderPage', pageId: 'page.home' },
};

const routeArticle: RouteDescriptor = {
  id: 'article',
  matcher: '^/articles/(?<articleId>[0-9]+)$',
  priority: 10,
  action: { type: 'renderPage', pageId: 'page.article' },
};

const themeDefault: ThemeDescriptor = {
  themeId: 'default',
  tokens: { '--color-primary': '#111111', '--color-bg': '#ffffff' },
};

const readFeedOp = {
  id: 'feed.list',
  typeOp: 'query',
  providerId: 'cms',
  method: 'GET',
  path: '/api/sites/{siteId}/feeds',
  params: { in: 'query', fields: { siteId: { required: true }, locale: { required: false } } },
  cache: 'ttl',
  ttl: 60,
  type: 'content[]',
  poll: { schedule: '* * * * * *', cache: true },
};

const otherOp = {
  id: 'other.list',
  typeOp: 'query',
  providerId: 'cms',
  method: 'GET',
  path: '/api/sites/{siteId}/others',
  params: { in: 'query', fields: { siteId: { required: true }, locale: { required: false } } },
  cache: 'disabled',
  type: 'content[]',
  poll: { schedule: '* * * * * *' },
};

function buildContract(overrides?: {
  operations?: unknown[];
  tree?: TreeDeclaration;
  dev?: boolean;
  locale?: string;
  version?: string;
}): Record<string, unknown> {
  return {
    siteId: 'acme',
    environment: 'prod',
    version: overrides?.version ?? 'v1',
    locale: overrides?.locale ?? 'ru',
    providers: [{ id: 'cms', protocol: 'http', baseUrl: 'http://cms.test' }],
    operations: overrides?.operations ?? [readFeedOp],
    endpoints: [],
    routes: [routeHome, routeArticle],
    themes: [themeDefault],
    enabledChannels: { ws: true, sse: true },
    capabilities: { formSubmissions: true, dev: overrides?.dev ?? false },
    ...(overrides?.tree ? { tree: overrides.tree } : {}),
  };
}

const treeA: TreeDeclaration = {
  snapshotId: 'snap-1',
  versionId: 'v1',
  root: {
    instanceId: 'root',
    definitionId: 'page.home',
    props: { title: { content: 'Главная' } },
    bindings: [],
    children: [],
  },
};

const treeB: TreeDeclaration = {
  snapshotId: 'snap-2',
  versionId: 'v2',
  root: {
    instanceId: 'root-b',
    definitionId: 'page.home',
    props: { title: { content: 'Другая' } },
    bindings: [],
    children: [],
  },
};

async function until(fn: () => boolean, ms = 1000): Promise<void> {
  const start = Date.now();
  while (!fn()) {
    if (Date.now() - start > ms) throw new Error('until(): таймаут');
    await new Promise((r) => setTimeout(r, 5));
  }
}

function contractCalls(calls: FetchCall[]): FetchCall[] {
  return calls.filter((c) => new URL(c.url).pathname === '/runtime/contract');
}

describe('16. boot', () => {
  beforeEach(() => {
    resetFakes();
  });

  it('1. GET /runtime/contract с siteId + environment (+ optional versionId)', async () => {
    const { fetch, calls } = makeFakeFetch(() => buildContract());
    await boot('acme', 'production', { env: { fetch } });
    const list = contractCalls(calls);
    expect(list).toHaveLength(1);
    const q = new URL(list[0].url).searchParams;
    expect(q.get('siteId')).toBe('acme');
    expect(q.get('environment')).toBe('production');
    expect(q.get('versionId')).toBeNull();
  });

  it('2. контракт регистрируется (провайдер/операция/эндпоинт), ready: true', async () => {
    const { fetch } = makeFakeFetch(() => buildContract());
    const rt = await boot('acme', 'production', { env: { fetch } });
    expect(rt.ready).toBe(true);
    expect(rt.registry.getProvider('cms')).not.toBeNull();
    expect(rt.registry.getOperation('feed.list')).not.toBeNull();
    expect(rt.registry.getOperation('content.list')).not.toBeNull();
    expect(rt.registry.getOperation('content.get')).not.toBeNull();
    expect(rt.registry.getEndpoint('form.submit')).not.toBeNull();
    rt.dispose();
  });

  it('3. тема применяется через DesignTokens и кладётся в store', async () => {
    const { fetch } = makeFakeFetch(() => buildContract());
    const rt = await boot('acme', 'production', { env: { fetch } });
    expect(rt.tokens.get('--color-primary')).toBe('#111111');
    expect(rt.store.getState().tokens['--color-bg']).toBe('#ffffff');
    rt.dispose();
  });

  it('4. роуты загружаются через Router', async () => {
    const { fetch } = makeFakeFetch(() => buildContract());
    const rt = await boot('acme', 'production', { env: { fetch } });
    expect(rt.store.getState().routes).toHaveLength(2);
    expect(rt.router.match('/articles/42')?.route.id).toBe('article');
    expect(rt.router.match('/new')?.route.id).toBe('home');
    rt.dispose();
  });

  it('5. defaultLocale из контракта применяется через I18n.detect', async () => {
    const { fetch } = makeFakeFetch(() => buildContract({ locale: 'ru' }));
    const rt = await boot('acme', 'production', { env: { fetch } });
    expect(rt.store.getState().locale).toBe('ru');
    rt.dispose();
  });

  it('5a. локальный детективный источник (navigatorLanguage) поддерживается', async () => {
    const { fetch } = makeFakeFetch(() => buildContract({ locale: 'ru' }));
    const rt = await boot('acme', 'production', {
      env: { fetch, navigatorLanguage: 'en-US' },
      i18n: { supportedLocales: ['en', 'ru'] },
    });
    expect(rt.store.getState().locale).toBe('en');
    rt.dispose();
  });

  it('6. если tree в контракте — TreeController.load', async () => {
    const { fetch } = makeFakeFetch(() => buildContract({ tree: treeA, operations: [] }));
    const rt = await boot('acme', 'production', { env: { fetch } });
    expect(rt.store.getState().tree?.root.instanceId).toBe('root');
    expect(rt.store.getState().tree?.versionId).toBe('v1');
    rt.dispose();
  });

  it('7. poll-расписания стартуют через SyncEngine (запросы с текущим locale)', async () => {
    let feedBody: unknown = { state: 'ok', items: [{ id: 'n1' }] };
    const { fetch, calls } = makeFakeFetch((call) => {
      const u = new URL(call.url);
      if (u.pathname.endsWith('/feeds')) return feedBody;
      return buildContract();
    });
    const rt = await boot('acme', 'production', { env: { fetch } });
    expect(rt.sync.activeSize).toBe(1);

    await until(() => {
      return calls.some((c) => new URL(c.url).pathname.endsWith('/feeds'));
    });
    const feedCall = calls.find((c) => new URL(c.url).pathname.endsWith('/feeds'))!;
    expect(new URL(feedCall.url).pathname).toBe('/api/sites/acme/feeds');
    const q = new URL(feedCall.url).searchParams;
    expect(q.get('locale')).toBe('ru');

    await until(() => rt.store.getState().operationResults['feed.list'] !== undefined);
    const entry = rt.store.getState().operationResults['feed.list'] as { data: { items: Array<{ id: string }> } };
    expect(entry.data.items[0].id).toBe('n1');
    rt.dispose();
  });

  it('8. отсутствие контракта (404) → TransportError', async () => {
    const { fetch } = makeFakeFetch(() => {
      return {
        ok: false,
        status: 404,
        async text() {
          return '';
        },
        async json() {
          return {};
        },
      };
    });
    await expect(boot('missing', 'production', { env: { fetch } })).rejects.toBeInstanceOf(TransportError);
    await expect(boot('missing', 'production', { env: { fetch } })).rejects.toMatchObject({ cause: { status: 404 } });
  });

  it('9. невалидный контракт (плохой cron в poll.schedule) → DescriptorValidationError', async () => {
    const badOp = { ...readFeedOp, poll: { schedule: 'not-a-cron', cache: true } };
    const { fetch } = makeFakeFetch(() => buildContract({ operations: [badOp] }));
    await expect(boot('acme', 'production', { env: { fetch } })).rejects.toBeInstanceOf(DescriptorValidationError);
  });

  it('10. boot с versionId шлёт контракт с этой версией', async () => {
    const { fetch, calls } = makeFakeFetch(() => buildContract());
    await boot('acme', 'production', { env: { fetch }, versionId: 'v42' });
    const q = new URL(contractCalls(calls)[0].url).searchParams;
    expect(q.get('versionId')).toBe('v42');
  });

  it('11. повторный boot (другая версия) → Registry.drop + сброс scheduler + полная перерегистрация', async () => {
    let contract = buildContract();
    const { fetch, calls } = makeFakeFetch((call) => {
      const u = new URL(call.url);
      if (u.pathname.endsWith('/feeds')) return { state: 'ok', items: [] };
      if (u.pathname.endsWith('/others')) return { state: 'ok', items: [] };
      return contract;
    });

    const rt1 = await boot('acme', 'production', { env: { fetch } });
    expect(rt1.registry.getOperation('feed.list')).not.toBeNull();
    expect(rt1.sync.activeSize).toBe(1);

    // v2: без feed.list, зато с other.list → drop/перерегистрация должны убрать feed.list
    contract = buildContract({ version: 'v2', operations: [otherOp] });
    const rt2 = await boot('acme', 'production', { env: { fetch }, versionId: 'v2' });
    expect(rt2.registry).toBe(rt1.registry);
    expect(rt2.store).toBe(rt1.store);
    expect(rt1.registry.getOperation('feed.list')).toBeNull();
    expect(rt1.registry.getOperation('other.list')).not.toBeNull();
    expect(rt1.registry.getOperation('content.list')).not.toBeNull();
    expect(rt1.sync.activeSize).toBe(0);
    expect(rt2.sync.activeSize).toBe(1);
    const vCalls = contractCalls(calls);
    expect(vCalls).toHaveLength(2);
    expect(new URL(vCalls[1].url).searchParams.get('versionId')).toBe('v2');
    rt2.dispose();
  });

  it('12. dev-mode: подписка на WS-канал DevTransport и rebuild при новой декларации', async () => {
    const { fetch } = makeFakeFetch(() => buildContract({ tree: treeA, dev: true }));
    const rt = await boot('acme', 'development', { env: { fetch, WebSocket: FakeWebSocket } });
    expect(FakeWebSocket.instances).toHaveLength(1);
    expect(FakeWebSocket.instances[0].url).toBe('http://localhost/runtime/dev');

    const ws = FakeWebSocket.instances[0];
    ws.emitMessage(JSON.stringify({ type: 'tree', tree: treeB }));
    expect(rt.store.getState().tree?.root.instanceId).toBe('root-b');
    expect(rt.store.getState().tree?.versionId).toBe('v2');
    rt.dispose();
  });

  it('12b. production: WS-канал DevTransport не создаётся', async () => {
    const { fetch } = makeFakeFetch(() => buildContract({ tree: treeA, dev: true }));
    const rt = await boot('acme', 'production', { env: { fetch, WebSocket: FakeWebSocket } });
    expect(FakeWebSocket.instances).toHaveLength(0);
    rt.dispose();
  });
});