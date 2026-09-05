import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';
import type { ReactNode } from 'react';
import type { BootRuntime } from '../../src/core/boot';
import { ApiClient } from '../../src/core/api-client';
import { ContentController } from '../../src/core/content';
import { PageRenderer, RuntimeProvider, useContent, useForm, useMutation, useQuery, useT } from '../../src/react';
import { TransportFactory } from '../../src/core/transport/factory';
import { ScopeError } from '../../src/errors';
import type { TreeDeclaration } from '../../src/types/tree';
import { FakeWebSocket, makeFakeFetch, resetFakes, type FetchCall } from './helpers';
import { routeHome } from './react-harness';

function Scaffold({ runtime, children }: { runtime: BootRuntime; children: ReactNode }) {
  return <RuntimeProvider runtime={runtime}>{children}</RuntimeProvider>;
}

/** Серверная часть интеграционного fake-edge: контент base+overlay по локали, ассеты, форма, файлы, feeds, лайки. */
interface Backend {
  content: Record<string, { base: Record<string, unknown>; over: Record<string, unknown> }>;
  pollValue: { ver: number };
  submissions: Array<Record<string, unknown>>;
  likes: number;
  handler: (call: FetchCall) => unknown;
  files: Record<string, { bytes: string; contentType: string }>;
}

function createBackend(): Backend {
  const backend: Backend = {
    content: {
      post: {
        base: { title: 'Post title', image: { assetId: 'img1', variant: 'thumb' }, items: [{ id: 'k1', label: 'Base one' }] },
        over: { title: 'Заголовок', items: [{ id: 'k1', label: 'Первый' }] },
      },
      strings: {
        base: { cta: 'Press here' },
        over: { cta: 'Нажми сюда' },
      },
    },
    pollValue: { ver: 1 },
    submissions: [],
    likes: 0,
    files: { '/files/banner.png': { bytes: '<png-bytes>', contentType: 'image/png' } },
    handler: () => null,
  };
  backend.handler = (call: FetchCall) => {
    const url = new URL(call.url, 'http://localhost');
    const path = url.pathname;
    const method = call.init.method ?? 'GET';
    const locale = url.searchParams.get('locale') ?? 'ru';

    if (path === '/runtime/contract') return contract();

    if (path === '/api/contents/post') return mergeBy(backend.content.post, locale);
    if (path === '/api/contents/strings') return mergeBy(backend.content.strings, locale);

    if (path === '/api/assets/img1') {
      return {
        id: 'img1',
        name: 'Banner',
        type: 'image/webp',
        size: 4096,
        variants: [
          { name: 'master', url: 'https://cdn.test/img1.webp' },
          { name: 'thumb', url: 'https://cdn.test/img1-thumb.jpg' },
        ],
      };
    }

    if (path === '/api/forms/form.contact' && method === 'GET') {
      return {
        id: 'form.contact',
        fields: [{ name: 'email', type: 'email', required: true }],
        validation: [],
        submit: { target: 'endpoint.form.submit' },
      };
    }
    if (path === '/api/forms/form.contact/submissions' && method === 'POST') {
      backend.submissions.push(JSON.parse(String(call.init.body)) as Record<string, unknown>);
      return { submissionId: `sub-${backend.submissions.length}`, state: 'ok' };
    }

    if (path.endsWith('/feeds') && method === 'GET') return { state: 'ok', items: [{ id: 'n1' }] };
    if (path.endsWith('/likes') && method === 'POST') {
      backend.likes += 1;
      return { state: 'ok', likes: backend.likes };
    }
    if (path.endsWith('/content') && method === 'GET') return { state: 'ok', ...backend.pollValue };
    if (path === '/api/admin/secret' && method === 'POST') return { ok: true, secret: 's3cret' };

    if (backend.files[path]) {
      return { state: 'ok', bytes: backend.files[path].bytes, contentType: backend.files[path].contentType };
    }
    return null;
  };
  return backend;
}

/** base + overlay по локали: серверный фолбэк — нет перевода в локали → base. */
function mergeBy(doc: { base: Record<string, unknown>; over: Record<string, unknown> }, locale: string): Record<string, unknown> {
  return locale === 'ru' ? { ...doc.base, ...doc.over } : doc.base;
}

function feedOp(): Record<string, unknown> {
  return {
    id: 'feed.list',
    typeOp: 'query',
    providerId: 'cms',
    method: 'GET',
    path: '/api/sites/{siteId}/feeds',
    params: { in: 'query', fields: { siteId: { required: true }, locale: { required: false } } },
    cache: 'disabled',
  };
}

function likeOp(): Record<string, unknown> {
  return {
    id: 'feed.like',
    typeOp: 'mutation',
    providerId: 'cms',
    method: 'POST',
    path: '/api/sites/{siteId}/likes',
    params: { in: 'query', fields: { siteId: { required: true } } },
    cache: 'disabled',
  };
}

function contract(): Record<string, unknown> {
  return {
    siteId: 'acme',
    environment: 'prod',
    version: 'v1',
    locale: 'ru',
    providers: [{ id: 'cms', protocol: 'http', baseUrl: 'http://edge.test' }],
    operations: [
      feedOp(),
      likeOp(),
      {
        id: 'cms.content',
        typeOp: 'query',
        providerId: 'cms',
        method: 'GET',
        path: '/api/sites/{siteId}/content',
        params: { in: 'query', fields: { siteId: { required: true }, locale: { required: false } } },
        cache: 'disabled',
        poll: { schedule: '* * * * * *', cache: true },
      },
      {
        id: 'admin.secret',
        typeOp: 'query',
        scope: 'server',
        providerId: 'cms',
        method: 'POST',
        path: '/api/admin/secret',
        cache: 'disabled',
      },
    ],
    endpoints: [{ id: 'admin.secret', path: '/api/admin/secret', method: 'POST', operationId: 'admin.secret' }],
    routes: [
      routeHome(),
      {
        id: 'file',
        matcher: '^/files/(?<file>.*)$',
        priority: 5,
        action: { type: 'serveAsset', assetId: 'banner' },
      },
    ],
    themes: [{ themeId: 'default', tokens: { '--color-primary': '#111111' } }],
    enabledChannels: { ws: true, sse: true },
    capabilities: { formSubmissions: true, dev: false },
    tree: treeA(),
  };
}

function treeA(): TreeDeclaration {
  return {
    snapshotId: 's1',
    versionId: 'v1',
    root: {
      instanceId: 'root',
      definitionId: 'page.home',
      props: {},
      bindings: [],
      children: [
        {
          instanceId: 'title',
          definitionId: 'title',
          props: { text: '—' },
          bindings: [{ property: 'text', source: { type: 'content', contentId: 'post', path: 'title' } }],
          children: [],
        },
        {
          instanceId: 'image',
          definitionId: 'image',
          props: {},
          bindings: [{ property: 'src', source: { type: 'content', contentId: 'post', path: 'image' } }],
          children: [],
        },
        {
          instanceId: 'list',
          definitionId: 'list',
          props: {},
          bindings: [{ property: 'items', source: { type: 'content', contentId: 'post', path: 'items' } }],
          children: [
            { instanceId: 'k1', definitionId: 'item', props: {}, bindings: [], children: [] },
            { instanceId: 'k2', definitionId: 'item', props: {}, bindings: [], children: [] },
          ],
        },
      ],
    },
  };
}

function treeB(): TreeDeclaration {
  const a = treeA();
  const list = a.root.children[2];
  return {
    ...a,
    snapshotId: 's2',
    versionId: 'v2',
    root: {
      ...a.root,
      children: [
        ...a.root.children.slice(0, 2),
        {
          ...list,
          children: [
            { instanceId: 'k1', definitionId: 'item', props: {}, bindings: [], children: [] },
            { instanceId: 'k2', definitionId: 'item', props: {}, bindings: [], children: [] },
            { instanceId: 'k3', definitionId: 'item', props: {}, bindings: [], children: [] },
          ],
        },
      ],
    },
  };
}

const components = {
  'page.home': ({ children }: { children?: ReactNode }) => <section data-testid="root">{children}</section>,
  title: ({ text }: { text?: unknown }) => <h1 data-testid="title">{String(text)}</h1>,
  image: ({ src }: { src?: unknown }) => <img data-testid="image" src={String(src)} alt="" />,
  list: ({ items, children }: { items?: unknown; children?: ReactNode }) => (
    <ul data-testid="list" data-count={Array.isArray(items) ? items.length : 0}>
      {children}
    </ul>
  ),
  item: () => <li data-testid="item">x</li>,
};

let backendSeq = 0;

/** Полный интеграционный стенд: fake-edge + boot + ContentController (апп-слой контента). */
async function setup(window?: { location: { assign(url: string): void } }): Promise<{
  runtime: BootRuntime;
  backend: Backend;
  calls: FetchCall[];
}> {
  backendSeq += 1;
  const backend = createBackend();
  const { fetch, calls } = makeFakeFetch(backend.handler);
  const { boot } = await import('../../src/core/boot');
  const runtime = await boot(`int-${backendSeq}`, 'production', {
    i18n: {
      syncContent: async (locale: string) => {
        const post = (await runtime.client.builtin.content.get('post', { locale })) as Record<string, unknown>;
        const strings = (await runtime.client.builtin.content.get('strings', { locale })) as Record<string, unknown>;
        runtime.store.getState().setContent({ post, strings });
      },
    },
    env: {
      fetch,
      navigatorLanguage: 'ru',
      WebSocket: FakeWebSocket,
      window: window ?? { location: { assign: () => undefined } },
    },
  });
  return { runtime, backend, calls };
}

function contentController(runtime: BootRuntime): ContentController {
  return new ContentController(runtime.store, runtime.client.builtin.content, {
    siteId: runtime.siteId,
    assetResolver: runtime.assets,
  });
}

async function until(fn: () => boolean, ms = 2500): Promise<void> {
  const start = Date.now();
  while (!fn()) {
    if (Date.now() - start > ms) throw new Error('until(): таймаут');
    await new Promise((r) => setTimeout(r, 5));
  }
}

describe('19. integration (один слой сквозной)', () => {
  beforeEach(() => resetFakes());

  it('1. boot → дерево → content из fake-сервера через binding; image {assetId,variant} → URL', async () => {
    const { runtime } = await setup();
    const decl = treeA();
    const rebuilds: number[] = [0];
    runtime.tree.onRebuild(() => {
      rebuilds[0] += 1;
    });

    render(
      <Scaffold runtime={runtime}>
        <PageRenderer components={components} />
      </Scaffold>,
    );
    await waitFor(() => expect(screen.getByTestId('title').textContent).toBe('—'));
    expect(screen.getByTestId('image').getAttribute('src')).toBe('undefined');

    // app: метаданные ассета → контент в стор → дерево резолвится заново (load, не rebuild)
    await runtime.assets.get('img1');
    await contentController(runtime).get('post', { locale: 'ru' });
    act(() => runtime.tree.load(decl));

    expect(screen.getByTestId('title').textContent).toBe('Заголовок');
    expect(screen.getByTestId('image').getAttribute('src')).toBe('https://cdn.test/img1-thumb.jpg');
    expect(rebuilds[0]).toBe(0);
  });

  it('2. поллинг обновил стор; дерево НЕ пересобиралось (onRebuild=0, DOM по key без изменений)', async () => {
    const { runtime, backend } = await setup();
    const rebuilds: number[] = [0];
    runtime.tree.onRebuild(() => {
      rebuilds[0] += 1;
    });

    render(
      <Scaffold runtime={runtime}>
        <PageRenderer components={components} />
      </Scaffold>,
    );
    await waitFor(() => expect(screen.getAllByTestId('item')).toHaveLength(2));
    const itemsBefore = screen.getAllByTestId('item');

    // «поменяли контент на сервере» → поллинг (cron) пишет в стор
    backend.pollValue = { ver: 2 };
    await until(() => {
      const res = runtime.store.getState().operationResults['cms.content'] as { data?: { ver?: number } } | undefined;
      return res?.data?.ver === 2;
    });

    expect(rebuilds[0]).toBe(0);
    expect(screen.getAllByTestId('item')).toHaveLength(2);
    expect(screen.getAllByTestId('item')[0]).toBe(itemsBefore[0]);
    expect(screen.getAllByTestId('item')[1]).toBe(itemsBefore[1]);
  });

  it('3. новая декларация → rebuild → DOM обновился', async () => {
    const { runtime } = await setup();
    const rebuilds: number[] = [0];
    runtime.tree.onRebuild(() => {
      rebuilds[0] += 1;
    });

    render(
      <Scaffold runtime={runtime}>
        <PageRenderer components={components} />
      </Scaffold>,
    );
    await waitFor(() => expect(screen.getAllByTestId('item')).toHaveLength(2));
    const k1 = screen.getAllByTestId('item')[0];

    act(() => runtime.tree.rebuild(treeB()));

    await waitFor(() => expect(screen.getAllByTestId('item')).toHaveLength(3));
    expect(screen.getAllByTestId('item')[0]).toBe(k1);
    expect(rebuilds[0]).toBe(1);
  });

  it('4. server-only endpoint из публичного контекста → ScopeError; из edge (server) — работает', async () => {
    const { runtime, calls } = await setup();
    const factory = new TransportFactory();
    const provider = runtime.registry.getProvider('cms');
    if (!provider) throw new Error('provider cms отсутствует');
    const pubClient = new ApiClient(runtime.registry, { transport: factory.create(provider), scope: 'public' });

    const before = calls.length;
    await expect(pubClient.callEndpoint('admin.secret', { q: 1 })).rejects.toBeInstanceOf(ScopeError);
    expect(calls.length).toBe(before);

    const res = await runtime.client.callEndpoint('admin.secret', { q: 1 });
    expect(res).toMatchObject({ ok: true, secret: 's3cret' });
  });

  it('5. форма: form.get → валидация → form.submit пишет raw JSON в fake-сервер', async () => {
    const { runtime, backend } = await setup();

    function Form() {
      const f = useForm<{ email: string }>('form.contact');
      return (
        <div>
          <input data-testid="email" {...f.register('email')} />
          <button data-testid="submit" onClick={() => void f.handleSubmit()}>
            {f.status}
          </button>
          <span data-testid="errors">{JSON.stringify(f.errors)}</span>
        </div>
      );
    }

    render(
      <Scaffold runtime={runtime}>
        <Form />
      </Scaffold>,
    );

    fireEvent.change(screen.getByTestId('email'), { target: { value: '' } });
    fireEvent.click(screen.getByTestId('submit'));
    await waitFor(() => expect(screen.getByTestId('errors').textContent).toContain('email'));
    expect(backend.submissions).toHaveLength(0);

    fireEvent.change(screen.getByTestId('email'), { target: { value: 'user@example.com' } });
    fireEvent.click(screen.getByTestId('submit'));
    await waitFor(() => expect(backend.submissions).toHaveLength(1));
    expect(backend.submissions[0]).toMatchObject({ formId: 'form.contact', locale: 'ru' });
    expect((backend.submissions[0].values as Record<string, unknown>).email).toBe('user@example.com');
  });

  it('6. setLocale → контент и UI-строки перечитаны по новой локали (серверный фолбэк); дерево НЕ пересобирается', async () => {
    const { runtime } = await setup();
    const decl = treeA();
    const rebuilds: number[] = [0];
    runtime.tree.onRebuild(() => {
      rebuilds[0] += 1;
    });
    const cc = contentController(runtime);

    function View() {
      const post = useContent<{ title: string }>('post');
      const t = useT();
      return (
        <div>
          <span data-testid="title">{post?.title}</span>
          <span data-testid="cta">{t('strings.cta')}</span>
        </div>
      );
    }
    render(
      <Scaffold runtime={runtime}>
        <View />
      </Scaffold>,
    );

    await act(async () => {
      await runtime.assets.get('img1');
      await cc.get('post', { locale: 'ru' });
      await cc.get('strings', { locale: 'ru' });
    });
    act(() => runtime.tree.load(decl));

    await waitFor(() => expect(screen.getByTestId('title').textContent).toBe('Заголовок'));
    expect(screen.getByTestId('cta').textContent).toBe('Нажми сюда');

    await act(async () => {
      await runtime.i18n.setLocale('en');
    });

    expect(runtime.store.getState().content.post.title).toBe('Post title');
    expect(screen.getByTestId('cta').textContent).toBe('Press here');
    expect(rebuilds[0]).toBe(0);

    // дерево пере-резолвится только явным load; rebuild не создаётся
    act(() => runtime.tree.load(decl));
    await waitFor(() => expect(screen.getByTestId('title').textContent).toBe('Post title'));
    expect(rebuilds[0]).toBe(0);
  });

  it('7. контент без перевода в локали → показывается base (серверный фолбэк)', async () => {
    const { runtime } = await setup();
    await runtime.assets.get('img1');
    const doc = await contentController(runtime).get('post', { locale: 'en' });
    expect(doc.title).toBe('Post title');

    await runtime.i18n.setLocale('en');
    expect(runtime.store.getState().content.post.title).toBe('Post title');
  });

  it('8. serveAsset: клиент делает полный переход; fake-edge отдаёт байты файла с content-type', async () => {
    const assigned: Array<{ url: string }> = [];
    const { runtime, backend } = await setup({ location: { assign: (url: string) => assigned.push({ url }) } });
    expect(runtime.router.match('/files/banner.png')?.route.action).toMatchObject({
      type: 'serveAsset',
      assetId: 'banner',
    });

    runtime.router.navigate('/files/banner.png');
    expect(assigned).toEqual([{ url: '/files/banner.png' }]);

    const res = backend.handler({ url: 'http://edge.test/files/banner.png', init: { method: 'GET' } }) as {
      bytes: string;
      contentType: string;
    };
    expect(res.bytes).toBe('<png-bytes>');
    expect(res.contentType).toBe('image/png');
  });

  it('9. query + mutation + builtin + поллинг + ассеты в одном флоу — согласованно', async () => {
    const { runtime } = await setup();

    function View() {
      const q = useQuery<{ items: Array<{ id: string }> }>('feed.list', { siteId: 'acme' });
      const m = useMutation<{ likes?: number }>('feed.like');
      const data = q.status === 'success' ? (q.data?.items ?? []).map((i) => i.id).join(',') : q.status;
      return (
        <div>
          <div data-testid="q">{data}</div>
          <button data-testid="like" onClick={() => void m.mutate({ siteId: 'acme' })}>
            {m.status}
          </button>
        </div>
      );
    }

    render(
      <Scaffold runtime={runtime}>
        <View />
      </Scaffold>,
    );
    await waitFor(() => expect(screen.getByTestId('q').textContent).toContain('n1'));

    fireEvent.click(screen.getByTestId('like'));
    await waitFor(() => expect(screen.getByTestId('like').textContent).toBe('success'));

    // builtin: ассеты + контент
    await runtime.assets.get('img1');
    expect(runtime.assets.url({ assetId: 'img1', variant: 'thumb' })).toBe('https://cdn.test/img1-thumb.jpg');
    const doc = await contentController(runtime).get('post', { locale: 'ru' });
    expect(doc.title).toBe('Заголовок');

    // поллинг (cron) пишет в стор
    await until(() => typeof runtime.store.getState().operationResults['cms.content'] !== 'undefined');

    // server-only endpoint из edge-контекста
    expect(await runtime.client.callEndpoint('admin.secret', {})).toMatchObject({ ok: true });
  });
});