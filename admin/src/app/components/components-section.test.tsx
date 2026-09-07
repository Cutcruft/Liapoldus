import { afterEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import type { AdminComponent, ComponentRegistry, ComponentUsage } from '../../runtime';
import { jsonResponse, renderApp } from '../test-utils';
import { resetSiteTabStore } from '../site-tab-store';
import { __lastTsxEditor } from './TsxEditor';

const SITE = { id: 's1', name: 'Alpha', slug: 'alpha', defaultLocale: 'ru', hosts: [] as string[] };

let REGISTRY: ComponentRegistry = {
  devSha: 'abc123def456deadbeef',
  components: [
    {
      id: 'comp:card',
      name: 'Card',
      kind: 'component',
      currentSha: 'abc123def456',
      committed: true,
      usageCount: 2,
      updatedAt: '2026-01-02T00:00:00Z',
    },
    {
      id: 'comp:hero',
      name: 'Hero',
      kind: 'section',
      currentSha: null as unknown as undefined,
      committed: false,
      usageCount: 0,
      updatedAt: '2026-01-03T00:00:00Z',
    },
  ],
};

const CARD_SOURCE = [
  'export interface Props {',
  '  title: string',
  '}',
  'export default function Card({ title }: Props) {',
  '  return <section><h1>{title}</h1></section>',
  '}',
].join('\n');

const CARD: AdminComponent = {
  id: 'comp:card',
  siteId: 's1',
  name: 'Card',
  kind: 'component',
  source: CARD_SOURCE,
  schema: {},
  currentSha: 'abc123def456',
  committed: true,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-02T00:00:00Z',
};

const USAGE: ComponentUsage = {
  componentId: 'comp:card',
  pages: [{ id: 'home', name: 'Главная', count: 2 }],
};

function componentsHandler() {
  let registry = { ...REGISTRY, components: [...REGISTRY.components] };
  let card = { ...CARD };
  const extras = new Map<string, AdminComponent>();
  const history = [
    { sha: 'abc123def456', message: 'Начальная версия', time: '2026-01-01T00:00:00Z', source: CARD_SOURCE },
  ];
  return (url: string, init: RequestInit): Response => {
    const u = new URL(url, 'http://localhost');
    const path = u.pathname;
    const method = (init.method ?? 'GET').toUpperCase();
    if (method === 'GET' && path === '/api/sites/s1') return jsonResponse(200, SITE);
    if (method === 'GET' && path === '/api/sites/s1/components/registry')
      return jsonResponse(200, registry);
    if (method === 'POST' && path === '/api/sites/s1/components') {
      const body = JSON.parse(String(init.body)) as { name: string; kind: string; source: string };
      const created: AdminComponent = {
        id: `comp:${body.name.toLowerCase()}`,
        siteId: 's1',
        name: body.name,
        kind: body.kind,
        source: body.source,
        schema: {},
        createdAt: '2026-01-03T00:00:00Z',
        updatedAt: '2026-01-03T00:00:00Z',
      };
      registry = { ...registry, components: [...registry.components, { ...created, committed: false, usageCount: 0 }] };
      extras.set(created.id, created);
      return jsonResponse(201, created);
    }
    if (path === '/api/sites/s1/components/comp:card') {
      if (method === 'PUT') {
        const body = JSON.parse(String(init.body)) as Partial<AdminComponent>;
        card = { ...CARD, ...body };
        return jsonResponse(200, card);
      }
      const sha = u.searchParams.get('sha');
      if (sha) return jsonResponse(200, { ...CARD, source: CARD_SOURCE, currentSha: sha });
      return jsonResponse(200, card);
    }
    const extraMatch = /^\/api\/sites\/s1\/components\/([^/]+)$/.exec(path);
    if (extraMatch) {
      const extraId = extraMatch[1] ?? '';
      if (extras.has(extraId)) return jsonResponse(200, extras.get(extraId)!);
    }
    if (method === 'GET' && path === '/api/sites/s1/components/comp:card/history')
      return jsonResponse(200, history);
    if (method === 'GET' && path === '/api/sites/s1/components/comp:card/usage')
      return jsonResponse(200, USAGE);
    if (method === 'POST' && path === '/api/sites/s1/components/comp:card/commit') {
      card = { ...card, committed: true, currentSha: 'newsha001122' };
      registry = {
        ...registry,
        components: registry.components.map((c) =>
          c.id === 'comp:card' ? { ...c, committed: true, currentSha: 'newsha001122' } : c,
        ),
      };
      return jsonResponse(200, { sha: 'newsha001122', count: 1 });
    }
    return jsonResponse(404, { error: `no mock: ${method} ${path}` });
  };
}

afterEach(() => {
  resetSiteTabStore();
  vi.restoreAllMocks();
});

describe('Компоненты — реестр (R5)', () => {
  it('показывает заголовок, версию dev-HEAD и строки реестра', async () => {
    await renderApp({ path: '/sites/s1?view=editor&section=components', handler: componentsHandler() });

    expect(await screen.findByRole('heading', { name: 'Компоненты' })).toBeTruthy();
    expect(await screen.findByText('Card')).toBeTruthy();
    expect(screen.getByText('Hero')).toBeTruthy();
    expect(screen.getByText('abc123def4')).toBeTruthy(); // devSha
    expect(screen.getByText('сохранён')).toBeTruthy();
    expect(screen.getByText('не сохранён')).toBeTruthy();
    expect(screen.getByText('Страницы: 2')).toBeTruthy();
    expect(screen.getByText('Нигде не используется')).toBeTruthy();
  });

  it('без данных из API — заголовок и пустое состояние', async () => {
    await renderApp({ path: '/sites/s1?view=editor&section=components' });

    expect(await screen.findByRole('heading', { name: 'Компоненты' })).toBeTruthy();
    expect(await screen.findByText('Компонентов пока нет')).toBeTruthy();
  });

  it('клик по строке ведёт в detail (?componentId=...)', async () => {
    const { router } = await renderApp({ path: '/sites/s1?view=editor&section=components', handler: componentsHandler() });

    fireEvent.click(await screen.findByRole('button', { name: 'Card' }));

    expect(decodeURIComponent(router.state.location.search)).toContain('componentId=comp:card');
    expect(await screen.findByText('Schema — авто-вывод из props')).toBeTruthy();
    expect(screen.getByText('title')).toBeTruthy();
    expect(screen.getByText('обязательный')).toBeTruthy();
  });

  it('создание компонента: POST дефолтного сниппета и переход в detail', async () => {
    const { calls, router } = await renderApp({ path: '/sites/s1?view=editor&section=components', handler: componentsHandler() });

    fireEvent.click(await screen.findByRole('button', { name: 'Создать компонент' }));
    fireEvent.change(screen.getByLabelText('Название *'), { target: { value: 'Banner' } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    await waitFor(() =>
      expect(decodeURIComponent(router.state.location.search)).toContain('componentId=comp:banner'),
    );
    expect(await screen.findByRole('heading', { name: 'Banner' })).toBeTruthy();

    const post = calls.find((c) => c.method === 'POST' && c.url === '/api/sites/s1/components');
    const body = JSON.parse(String(post?.init.body)) as { name: string; kind: string; source: string };
    expect(body.name).toBe('Banner');
    expect(body.kind).toBe('component');
    expect(body.source).toContain('export interface Props');
  });
});

describe('Компоненты — detail (R5)', () => {
  it('показывает Schema, Usage и History; кнопка «К списку» возвращается', async () => {
    const { router } = await renderApp({
      path: '/sites/s1?view=editor&section=components&componentId=comp:card',
      handler: componentsHandler(),
    });

expect(await screen.findByText('Schema — авто-вывод из props')).toBeTruthy();
    expect(screen.getAllByText('title').length).toBeGreaterThan(0);
    expect(screen.getByText('обязательный')).toBeTruthy();
    expect(screen.getByText('Главная')).toBeTruthy();
    expect(screen.getByText('2')).toBeTruthy();
    expect(screen.getByText('Начальная версия')).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: '← К списку' }));
    await waitFor(() => expect(router.state.location.search).not.toContain('componentId'));
    expect(await screen.findByRole('heading', { name: 'Компоненты' })).toBeTruthy();
  });

  it('правка исходника помечает «не сохранён» и Сохранить шлёт PUT с source', async () => {
    const { calls } = await renderApp({
      path: '/sites/s1?view=editor&section=components&componentId=comp:card',
      handler: componentsHandler(),
    });

    await screen.findByText('Schema — авто-вывод из props');
    await screen.findByText('сохранён');

    await waitFor(() => expect(__lastTsxEditor()).toBeTruthy());
    const ed = __lastTsxEditor()!;
    const next = `${CARD_SOURCE}\n\n// правка R5`;
    ed.chain()
      .focus()
      .selectAll()
      .deleteSelection()
      .command(({ tr }) => {
        tr.insertText(next);
        return true;
      })
      .run();
    await waitFor(() => expect(ed.getText()).toBe(next));

    const saveBtn = await screen.findByRole('button', { name: 'Сохранить' });
    fireEvent.click(saveBtn);

    await waitFor(() => expect(calls.some((c) => c.method === 'PUT')).toBe(true));
    const put = calls.find((c) => c.method === 'PUT' && c.url === '/api/sites/s1/components/comp:card')!;
    const body = JSON.parse(String(put.init.body)) as { source: string };
    expect(body.source.indexOf('// правка R5') >= 0).toBe(true);
  });

  it('коммит шлёт POST /commit с сообщением', async () => {
    const { calls } = await renderApp({
      path: '/sites/s1?view=editor&section=components&componentId=comp:card',
      handler: componentsHandler(),
    });

    await screen.findByText('Начальная версия');
    fireEvent.change(screen.getByLabelText('Сообщение коммита'), { target: { value: 'Вторая итерация' } });
    fireEvent.click(screen.getByRole('button', { name: 'Коммит' }));

    await waitFor(() => {
      const post = calls.find((c) => c.method === 'POST' && c.url === '/api/sites/s1/components/comp:card/commit');
      expect(JSON.parse(String(post?.init.body))).toEqual({ message: 'Вторая итерация' });
    });
  });

  it('просмотр истории: запрос с ?sha= и read-only баннер', async () => {
    const { calls } = await renderApp({
      path: '/sites/s1?view=editor&section=components&componentId=comp:card',
      handler: componentsHandler(),
    });

    fireEvent.click(await screen.findByRole('button', { name: 'Открыть' }));

    const banners = await screen.findAllByText('Исходник на abc123def4 (read-only)');
    expect(banners.length).toBeGreaterThan(0);
    await waitFor(() =>
      expect(
        calls.some((c) => c.method === 'GET' && c.url === '/api/sites/s1/components/comp:card?sha=abc123def456'),
      ).toBe(true),
    );
  });
});