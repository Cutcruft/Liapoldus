import { describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import type { Route } from '../../runtime';
import { jsonResponse, renderApp } from '../test-utils';

const ROUTES: Route[] = [
  { id: 'r1', siteId: 's1', matcher: '/about', priority: 1, action: { type: 'renderPage', pageId: 'p1' } },
  { id: 'r2', siteId: 's1', matcher: '/old', priority: 2, action: { type: 'redirect', target: '/new', status: 302, keepQuery: true } },
];

describe('SiteRoutesPage', () => {
  it('показывает роуты: matcher, приоритет, действие (включая status/keepQuery)', async () => {
    renderApp({
      path: '/sites/s1/routes',
      handler: (url) => (url === '/api/sites/s1/routes' ? jsonResponse(200, ROUTES) : jsonResponse(200, [])),
    });

    expect(await screen.findByText('/about')).toBeTruthy();
    expect(screen.getByText('Показ страницы → p1')).toBeTruthy();
    expect(screen.getByText('Редирект → /new 302 · keepQuery')).toBeTruthy();
    expect(screen.getByText('2')).toBeTruthy();
    expect(screen.getAllByRole('link', { name: 'Редактировать' }).length).toBe(2);
  });

  it('создаёт роут: POST {matcher, priority, action}', async () => {
    let list: Route[] = [...ROUTES];
    const { calls } = await renderApp({
      path: '/sites/s1/routes',
      handler: (url, init) => {
        if (url === '/api/sites/s1/routes' && init.method === 'POST') {
          const body = JSON.parse(String(init.body)) as { matcher: string; priority: number; action: Route['action'] };
          const created: Route = { id: 'r3', siteId: 's1', matcher: body.matcher, priority: body.priority, action: body.action };
          list = [...list, created];
          return jsonResponse(201, created);
        }
        return jsonResponse(200, list);
      },
    });

    fireEvent.click(await screen.findByRole('button', { name: 'Создать роут' }));
    fireEvent.change(screen.getByLabelText('Matcher (regex) *'), { target: { value: '/blog' } });
    fireEvent.change(screen.getByLabelText('Цель *'), { target: { value: '/posts' } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    await waitFor(() => expect(screen.getByText('/blog')).toBeTruthy());
    const post = calls.find((c) => c.method === 'POST' && c.url === '/api/sites/s1/routes');
    expect(JSON.parse(String(post?.init.body))).toEqual({
      matcher: '/blog',
      priority: 1,
      action: { type: 'renderPage', pageId: '/posts' },
    });
  });

  it('redirect: статус 308 + keepQuery в action', async () => {
    let list: Route[] = [...ROUTES];
    const { calls } = await renderApp({
      path: '/sites/s1/routes',
      handler: (url, init) => {
        if (url === '/api/sites/s1/routes' && init.method === 'POST') {
          const body = JSON.parse(String(init.body)) as { matcher: string; priority: number; action: Route['action'] };
          list = [...list, { id: 'r4', siteId: 's1', matcher: body.matcher, priority: body.priority, action: body.action }];
          return jsonResponse(201, body.action);
        }
        return jsonResponse(200, list);
      },
    });

    fireEvent.click(await screen.findByRole('button', { name: 'Создать роут' }));
    fireEvent.change(screen.getByLabelText('Matcher (regex) *'), { target: { value: '/legacy' } });
    fireEvent.change(screen.getByLabelText('Действие'), { target: { value: 'redirect' } });
    fireEvent.change(screen.getByLabelText('Цель *'), { target: { value: '/moved' } });
    fireEvent.change(screen.getByLabelText('Код ответа *'), { target: { value: '308' } });
    fireEvent.click(screen.getByRole('checkbox'));
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    await waitFor(() => expect(screen.getByText('/legacy')).toBeTruthy());
    const post = calls.find((c) => c.method === 'POST' && c.url === '/api/sites/s1/routes');
    expect(JSON.parse(String(post?.init.body))).toEqual({
      matcher: '/legacy',
      priority: 1,
      action: { type: 'redirect', target: '/moved', status: 308, keepQuery: true },
    });
  });

  it('невалидный regex блокирует submit и показывает ошибку под полем', async () => {
    const { calls } = await renderApp({
      path: '/sites/s1/routes',
      handler: () => jsonResponse(200, ROUTES),
    });

    fireEvent.click(await screen.findByRole('button', { name: 'Создать роут' }));
    fireEvent.change(screen.getByLabelText('Matcher (regex) *'), { target: { value: '(' } });
    fireEvent.change(screen.getByLabelText('Цель *'), { target: { value: '/posts' } });
    fireEvent.blur(screen.getByLabelText('Matcher (regex) *'));
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    expect(await screen.findByText('Matcher — невалидный regex')).toBeTruthy();
    expect(calls.some((c) => c.method === 'POST')).toBe(false);
  });

  it('предупреждение о дубле matcher не блокирует создание', async () => {
    let list: Route[] = [...ROUTES];
    const { calls } = await renderApp({
      path: '/sites/s1/routes',
      handler: (url, init) => {
        if (url === '/api/sites/s1/routes' && init.method === 'POST') {
          const body = JSON.parse(String(init.body)) as { matcher: string; priority: number; action: Route['action'] };
          list = [...list, { id: 'dup', siteId: 's1', matcher: body.matcher, priority: body.priority, action: body.action }];
          return jsonResponse(201, body);
        }
        return jsonResponse(200, list);
      },
    });

    fireEvent.click(await screen.findByRole('button', { name: 'Создать роут' }));
    fireEvent.change(screen.getByLabelText('Matcher (regex) *'), { target: { value: '/about' } });
    fireEvent.change(screen.getByLabelText('Цель *'), { target: { value: '/new-about' } });
    fireEvent.blur(screen.getByLabelText('Matcher (regex) *'));

    expect(await screen.findByText('Matcher совпадает с существующим роутом')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    await waitFor(() => expect(calls.some((c) => c.method === 'POST' && c.url === '/api/sites/s1/routes')).toBe(true));
  });

  it('ссылка «Редактировать» ведёт в редактор роута', async () => {
    renderApp({
      path: '/sites/s1/routes',
      handler: (url) => {
        if (url === '/api/sites/s1/routes/r1') return jsonResponse(200, ROUTES[0]);
        return jsonResponse(200, ROUTES);
      },
    });

    fireEvent.click((await screen.findAllByRole('link', { name: 'Редактировать' }))[0]!);
    expect(await screen.findByText('К списку роутов')).toBeTruthy();
    expect(screen.getByDisplayValue('/about')).toBeTruthy();
  });
});