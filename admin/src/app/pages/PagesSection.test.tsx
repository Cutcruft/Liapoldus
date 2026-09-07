import { describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import type { Route } from '../../runtime';
import { jsonResponse, renderApp } from '../test-utils';

const ROUTES: Route[] = [
  { id: 'r1', siteId: 's1', matcher: '^/pricing$', priority: 1, action: { type: 'renderPage', pageId: 'page_price' } },
  { id: 'r2', siteId: 's1', matcher: '^/old$', priority: 2, action: { type: 'redirect', target: '/new', status: 302, keepQuery: true } },
];

describe('PagesSection (R8 — реестр роутов)', () => {
  it('раздел «Страницы» показывает реестр роутов с колонкой страницы', async () => {
    renderApp({
      path: '/sites/s1?view=editor&section=pages',
      handler: (url) => (url === '/api/sites/s1/routes' ? jsonResponse(200, ROUTES) : jsonResponse(200, ROUTES)),
    });

    expect(await screen.findByRole('heading', { name: 'Страницы' })).toBeTruthy();
    expect(await screen.findByText('^/pricing$')).toBeTruthy();
    expect(screen.getByText('^/old$')).toBeTruthy();
    expect(screen.getByText('Показ страницы → page_price')).toBeTruthy();
    expect(screen.getByText('page_price')).toBeTruthy();
    expect(screen.getByText('Редирект → /new 302 · keepQuery')).toBeTruthy();
  });

  it('создаёт роут: POST {matcher, priority, action}', async () => {
    let list: Route[] = [...ROUTES];
    const { calls } = await renderApp({
      path: '/sites/s1?view=editor&section=pages',
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
    fireEvent.change(screen.getByLabelText('Matcher (regex) *'), { target: { value: '^/blog$' } });
    fireEvent.change(screen.getByLabelText('Цель *'), { target: { value: '/posts' } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    await waitFor(() => expect(screen.getByText('^/blog$')).toBeTruthy());
    const post = calls.find((c) => c.method === 'POST' && c.url === '/api/sites/s1/routes');
    expect(JSON.parse(String(post?.init.body))).toEqual({
      matcher: '^/blog$',
      priority: 1,
      action: { type: 'renderPage', pageId: '/posts' },
    });
  });

  it('валидация: невалидный regex показывает ошибку и не шлёт POST', async () => {
    const { calls } = await renderApp({
      path: '/sites/s1?view=editor&section=pages',
      handler: () => jsonResponse(200, ROUTES),
    });

    fireEvent.click(await screen.findByRole('button', { name: 'Создать роут' }));
    fireEvent.change(await screen.findByLabelText('Matcher (regex) *'), { target: { value: '((' } });
    fireEvent.change(screen.getByLabelText('Цель *'), { target: { value: '/posts' } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    expect(await screen.findByText('Matcher — невалидный regex')).toBeTruthy();
    expect(calls.some((c) => c.method === 'POST')).toBe(false);
  });

  it('валидация: неякорный matcher показывает ошибку и не шлёт POST', async () => {
    const { calls } = await renderApp({
      path: '/sites/s1?view=editor&section=pages',
      handler: () => jsonResponse(200, ROUTES),
    });

    fireEvent.click(await screen.findByRole('button', { name: 'Создать роут' }));
    fireEvent.change(await screen.findByLabelText('Matcher (regex) *'), { target: { value: '/blog' } });
    fireEvent.change(screen.getByLabelText('Цель *'), { target: { value: '/posts' } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    expect(await screen.findByText('Matcher должен быть якорным (^…$)')).toBeTruthy();
    expect(calls.some((c) => c.method === 'POST')).toBe(false);
  });

  it('превью regex-групп: тестовый path раскрывает $1 → значение', async () => {
    renderApp({
      path: '/sites/s1?view=editor&section=pages',
      handler: () => jsonResponse(200, ROUTES),
    });

    fireEvent.click(await screen.findByRole('button', { name: 'Создать роут' }));
    fireEvent.change(await screen.findByLabelText('Matcher (regex) *'), {
      target: { value: '^/articles/([0-9]+)$' },
    });
    fireEvent.change(screen.getByLabelText('Тестовый path (превью групп)'), { target: { value: '/articles/42' } });

    const group = await screen.findByText(
      (_content, node) => node?.tagName === 'CODE' && node?.textContent === '42',
    );
    expect(group).toBeTruthy();
  });

  it('предупреждение о дубле matcher не блокирует создание; delete уходит DELETE', async () => {
    let list: Route[] = [...ROUTES];
    const { calls } = await renderApp({
      path: '/sites/s1?view=editor&section=pages',
      handler: (url, init) => {
        if (url === '/api/sites/s1/routes' && init.method === 'POST') {
          const body = JSON.parse(String(init.body)) as { matcher: string; priority: number; action: Route['action'] };
          list = [...list, { id: 'dup', siteId: 's1', matcher: body.matcher, priority: body.priority, action: body.action }];
          return jsonResponse(201, body);
        }
        if (url.match(/\/routes\/r1$/) && init.method === 'DELETE') {
          list = list.filter((r) => r.id !== 'r1');
          return jsonResponse(204, undefined);
        }
        return jsonResponse(200, list);
      },
    });

    fireEvent.click(await screen.findByRole('button', { name: 'Создать роут' }));
    fireEvent.change(await screen.findByLabelText('Matcher (regex) *'), { target: { value: '^/pricing$' } });
    fireEvent.change(screen.getByLabelText('Цель *'), { target: { value: '/new-pricing' } });
    fireEvent.blur(screen.getByLabelText('Matcher (regex) *'));
    expect(await screen.findByText('Matcher совпадает с существующим роутом')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));
    await waitFor(() =>
      expect(calls.some((c) => c.method === 'POST' && c.url === '/api/sites/s1/routes')).toBe(true),
    );

    const deletes = await screen.findAllByRole('button', { name: 'Удалить' });
    fireEvent.click(deletes[0]!);
    const modal = await screen.findByRole('alertdialog');
    fireEvent.click(within(modal).getByRole('button', { name: 'Подтвердить' }));
    await waitFor(() =>
      expect(calls.some((c) => c.method === 'DELETE' && c.url === '/api/sites/s1/routes/r1')).toBe(true),
    );
  });
});