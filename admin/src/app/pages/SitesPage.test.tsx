import { describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import type { Site } from '../../runtime';
import { jsonResponse, renderApp } from '../test-utils';

const SITES: Site[] = [
  { id: 's1', name: 'Корпоративный портал', slug: 'portal', defaultLocale: 'ru', hosts: ['portal.example'] },
  { id: 's2', name: 'Блог', slug: 'blog', defaultLocale: 'en', hosts: [] },
];

describe('SitesPage', () => {
  it('показывает список сайтов с колонками', async () => {
    renderApp({
      path: '/sites',
      handler: () => jsonResponse(200, SITES),
    });

    expect(await screen.findByText('Корпоративный портал')).toBeTruthy();
    expect(screen.getByText('portal')).toBeTruthy();
    expect(screen.getByText('Блог')).toBeTruthy();
    expect(screen.getByText('en')).toBeTruthy();
  });

  it('пустой список → подсказка', async () => {
    renderApp({
      path: '/sites',
      handler: () => jsonResponse(200, []),
    });
    expect(await screen.findByText('Сайтов пока нет')).toBeTruthy();
  });

  it('создаёт сайт: POST + перезагрузка списка', async () => {
    let list: Site[] = [...SITES];
    const { calls } = await renderApp({
      path: '/sites',
      handler: (url, init) => {
        if (url === '/api/sites' && init.method === 'POST') {
          const body = JSON.parse(String(init.body)) as Record<string, unknown>;
          const created: Site = {
            id: 's3',
            name: String(body.name),
            slug: String(body.slug),
            defaultLocale: String(body.defaultLocale),
            hosts: [],
          };
          list = [...list, created];
          return jsonResponse(201, created);
        }
        return jsonResponse(200, list);
      },
    });

    fireEvent.click(screen.getByRole('button', { name: 'Создать сайт' }));
    fireEvent.change(screen.getByLabelText('Название *'), { target: { value: 'Новый' } });
    fireEvent.change(screen.getByLabelText('Slug *'), { target: { value: 'new' } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    expect(await screen.findByText('Новый')).toBeTruthy();

    const post = calls.find((c) => c.method === 'POST' && c.url === '/api/sites');
    expect(post).toBeTruthy();
    expect(JSON.parse(String(post?.init.body))).toEqual({ name: 'Новый', slug: 'new', defaultLocale: 'ru', hosts: [] });
    expect(calls.filter((c) => c.url === '/api/sites' && c.method === 'GET').length).toBeGreaterThanOrEqual(2);
  });

  it('удаляет сайт через двухшаговый confirm', async () => {
    let list: Site[] = [...SITES];
    const { calls } = await renderApp({
      path: '/sites',
      handler: (url, init) => {
        if (init.method === 'DELETE' && url === '/api/sites/s1') {
          list = list.filter((s) => s.id !== 's1');
          return jsonResponse(204, undefined);
        }
        return jsonResponse(200, list);
      },
    });

    expect(await screen.findByText('Корпоративный портал')).toBeTruthy();
    fireEvent.click(screen.getAllByRole('button', { name: 'Удалить' })[0]!);
    fireEvent.click(screen.getByRole('button', { name: 'Подтвердить' }));

    await waitFor(() => expect(screen.queryByText('Корпоративный портал')).toBeNull());
    const del = calls.find((c) => c.method === 'DELETE' && c.url === '/api/sites/s1');
    expect(del).toBeTruthy();
  });

  it('ошибка загрузки показывает detail + кнопку «Обновить»', async () => {
    let attempts = 0;
    await renderApp({
      path: '/sites',
      handler: () => {
        attempts += 1;
        if (attempts === 1) return jsonResponse(500, { error: 'boom' });
        return jsonResponse(200, SITES);
      },
    });

    expect(await screen.findByText(/Ошибка/)).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Обновить' }));
    expect(await screen.findByText('Корпоративный портал')).toBeTruthy();
  });

  it('валидность имени/слага в форме', async () => {
    await renderApp({ path: '/sites', handler: () => jsonResponse(200, []) });
    fireEvent.click(await screen.findByRole('button', { name: 'Создать сайт' }));
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));
    expect(await screen.findByText('Обязательно')).toBeTruthy();
  });
});