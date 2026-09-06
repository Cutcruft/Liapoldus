import { describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import type { Route } from '../../runtime';
import { jsonResponse, renderApp } from '../test-utils';

const ROUTES: Route[] = [
  { id: 'r1', siteId: 's1', matcher: '^/$', priority: 1, action: { type: 'renderPage', pageId: 'p1' } },
  { id: 'r2', siteId: 's1', matcher: '^/docs', priority: 2, action: { type: 'redirect', target: '/wiki', keepQuery: true } },
  { id: 'r3', siteId: 's1', matcher: '^/img/(.+)', priority: 3, action: { type: 'serveAsset', assetId: 'a1' } },
];

describe('SiteRoutesPage', () => {
  it('показывает роуты с описанием действия', async () => {
    renderApp({
      path: '/sites/s1/routes',
      handler: (url) => (url.startsWith('/api/sites/s1/routes') ? jsonResponse(200, ROUTES) : jsonResponse(200, [])),
    });

    expect(await screen.findByText('Показ страницы → p1')).toBeTruthy();
    expect(screen.getByText('Редирект → /wiki')).toBeTruthy();
    expect(screen.getByText('Выдача ассета → a1')).toBeTruthy();
  });

  it('создаёт роут с действием redirect', async () => {
    let list: Route[] = [...ROUTES];
    const { calls } = await renderApp({
      path: '/sites/s1/routes',
      handler: (url, init) => {
        if (url === '/api/sites/s1/routes' && init.method === 'POST') {
          const body = JSON.parse(String(init.body)) as Record<string, unknown>;
          const created: Route = {
            id: 'r4',
            siteId: 's1',
            matcher: String(body.matcher),
            priority: Number(body.priority),
            action: body.action as Route['action'],
          };
          list = [...list, created];
          return jsonResponse(201, created);
        }
        return jsonResponse(200, list);
      },
    });

    fireEvent.click(await screen.findByRole('button', { name: 'Создать роут' }));
    fireEvent.change(screen.getByLabelText('Matcher (regex) *'), { target: { value: '^/blog' } });
    fireEvent.change(screen.getByLabelText('Приоритет'), { target: { value: '5' } });
    fireEvent.change(screen.getByLabelText('Действие'), { target: { value: 'redirect' } });
    fireEvent.change(screen.getByLabelText('Цель *'), { target: { value: '/blog' } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    expect(await screen.findByText('Редирект → /blog')).toBeTruthy();

    const post = calls.find((c) => c.method === 'POST' && c.url === '/api/sites/s1/routes');
    expect(JSON.parse(String(post?.init.body))).toEqual({
      matcher: '^/blog',
      priority: 5,
      action: { type: 'redirect', target: '/blog' },
    });
  });

  it('удаляет роут через подтверждение', async () => {
    let list: Route[] = [...ROUTES];
    const { calls } = await renderApp({
      path: '/sites/s1/routes',
      handler: (url, init) => {
        if (init.method === 'DELETE' && url === '/api/sites/s1/routes/r1') {
          list = list.filter((r) => r.id !== 'r1');
          return jsonResponse(204, undefined);
        }
        return jsonResponse(200, list);
      },
    });

    expect(await screen.findByText('Показ страницы → p1')).toBeTruthy();
    fireEvent.click(screen.getAllByRole('button', { name: 'Удалить' })[0]!);
    fireEvent.click(screen.getByRole('button', { name: 'Подтвердить' }));

    await waitFor(() => expect(screen.queryByText('Показ страницы → p1')).toBeNull());
    expect(calls.some((c) => c.method === 'DELETE' && c.url === '/api/sites/s1/routes/r1')).toBe(true);
  });
});