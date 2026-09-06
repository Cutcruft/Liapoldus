import { describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import type { Page } from '../../runtime';
import { jsonResponse, renderApp } from '../test-utils';

const PAGES: Page[] = [
  {
    id: 'p1',
    siteId: 's1',
    name: 'Главная',
    slug: 'index',
    root: { id: 'root', type: 'Container', children: [] },
    version: 3,
  },
  {
    id: 'p2',
    siteId: 's1',
    name: 'О нас',
    slug: 'about',
    root: { id: 'root', type: 'Container', children: [] },
    version: 1,
  },
];

describe('SitePagesPage', () => {
  it('показывает страницы сайта', async () => {
    renderApp({
      path: '/sites/s1/pages',
      handler: (url) => (url.startsWith('/api/sites/s1/pages') ? jsonResponse(200, PAGES) : jsonResponse(200, [])),
    });

    expect(await screen.findByText('Главная')).toBeTruthy();
    expect(screen.getByText('О нас')).toBeTruthy();
    expect(screen.getByText('v3')).toBeTruthy();
  });

  it('создаёт страницу: POST /api/sites/s1/pages', async () => {
    let list: Page[] = [...PAGES];
    const { calls } = await renderApp({
      path: '/sites/s1/pages',
      handler: (url, init) => {
        if (url === '/api/sites/s1/pages' && init.method === 'POST') {
          const body = JSON.parse(String(init.body)) as Record<string, unknown>;
          const created: Page = {
            id: 'p3',
            siteId: 's1',
            name: String(body.name),
            slug: String(body.slug),
            root: { id: 'root', type: 'Container', children: [] },
            version: 1,
          };
          list = [...list, created];
          return jsonResponse(201, created);
        }
        return jsonResponse(200, list);
      },
    });

    fireEvent.click(await screen.findByRole('button', { name: 'Создать страницу' }));
    fireEvent.change(screen.getByLabelText('Название *'), { target: { value: 'Контакты' } });
    fireEvent.change(screen.getByLabelText('Slug *'), { target: { value: 'contacts' } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    expect(await screen.findByText('Контакты')).toBeTruthy();
    const post = calls.find((c) => c.method === 'POST' && c.url === '/api/sites/s1/pages');
    expect(JSON.parse(String(post?.init.body))).toEqual({
      name: 'Контакты',
      slug: 'contacts',
      root: { id: 'root', type: 'Container', children: [] },
    });
  });

  it('удаляет страницу через подтверждение', async () => {
    let list: Page[] = [...PAGES];
    const { calls } = await renderApp({
      path: '/sites/s1/pages',
      handler: (url, init) => {
        if (init.method === 'DELETE' && url === '/api/pages/p1') {
          list = list.filter((p) => p.id !== 'p1');
          return jsonResponse(204, undefined);
        }
        return jsonResponse(200, list);
      },
    });

    expect(await screen.findByText('Главная')).toBeTruthy();
    fireEvent.click(screen.getAllByRole('button', { name: 'Удалить' })[0]!);
    fireEvent.click(screen.getByRole('button', { name: 'Подтвердить' }));

    await waitFor(() => expect(screen.queryByText('Главная')).toBeNull());
    expect(calls.some((c) => c.method === 'DELETE' && c.url === '/api/pages/p1')).toBe(true);
  });

  it('ссылка «Открыть» ведёт в редактор', async () => {
    await renderApp({
      path: '/sites/s1/pages',
      handler: (url) => (url.startsWith('/api/sites/s1/pages') ? jsonResponse(200, PAGES) : jsonResponse(200, [])),
    });

    const link = (await screen.findAllByRole('link', { name: 'Открыть' }))[0]!;
    expect(link.getAttribute('href')).toBe('/sites/s1/pages/p1');
  });
});