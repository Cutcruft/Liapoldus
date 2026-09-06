import { describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import type { ContentSummary } from '../../runtime';
import { jsonResponse, renderApp } from '../test-utils';

type Row = { id: string; collectionId: string; fields: Record<string, unknown> };

function asSummary(r: Row): ContentSummary {
  return r;
}

describe('SiteContentsPage', () => {
  it('показывает контент списком и фильтрует по коллекции', async () => {
    renderApp({
      path: '/sites/s1/contents',
      handler: (url) =>
        url === '/api/sites/s1/contents'
          ? jsonResponse(200, [
              { id: 'nav.home', collectionId: 'strings', fields: { title: 'Главная' } },
              { id: 'footer', collectionId: 'strings', fields: { title: 'Подвал' } },
              { id: 'banner', collectionId: 'banners', fields: { text: 'Лето' } },
            ])
          : jsonResponse(200, []),
    });

    expect(await screen.findByText('nav.home')).toBeTruthy();
    expect(screen.getByText('banner')).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: 'strings' }));
    expect(screen.queryByText('banner')).toBeNull();
    expect(screen.getByText('nav.home')).toBeTruthy();
  });

  it('создаёт item: POST /api/sites/s1/contents с коллекцией/id/полями', async () => {
    const rows: Row[] = [];
    const { calls } = await renderApp({
      path: '/sites/s1/contents',
      handler: (url, init) => {
        if (url === '/api/sites/s1/contents' && init.method === 'POST') {
          const body = JSON.parse(String(init.body)) as Record<string, unknown>;
          rows.push({
            id: String(body.id),
            collectionId: String(body.collectionId),
            fields: (body.fields ?? {}) as Record<string, unknown>,
          });
          return jsonResponse(201, rows[rows.length - 1]);
        }
        return jsonResponse(200, rows.map(asSummary));
      },
    });

    fireEvent.click(await screen.findByRole('button', { name: 'Создать item' }));
    fireEvent.change(screen.getByLabelText('Коллекция *'), { target: { value: 'strings' } });
    fireEvent.change(screen.getByLabelText('ID'), { target: { value: 'nav.home' } });

    fireEvent.click(screen.getByRole('button', { name: /\+\s*Добавить поле/ }));
    fireEvent.change(screen.getByLabelText('Ключ'), { target: { value: 'title' } });
    fireEvent.change(screen.getByLabelText('Значение'), { target: { value: 'Главная' } });

    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    expect(await screen.findByText('nav.home')).toBeTruthy();
    const post = calls.find((c) => c.method === 'POST' && c.url === '/api/sites/s1/contents');
    expect(JSON.parse(String(post?.init.body))).toEqual({
      collectionId: 'strings',
      id: 'nav.home',
      fields: { title: 'Главная' },
    });
  });

  it('удаляет item через подтверждение', async () => {
    let rows: Row[] = [
      { id: 'nav.home', collectionId: 'strings', fields: { title: 'Главная' } },
      { id: 'nav.about', collectionId: 'strings', fields: { title: 'О нас' } },
    ];
    const { calls } = await renderApp({
      path: '/sites/s1/contents',
      handler: (url, init) => {
        if (init.method === 'DELETE' && url === '/api/sites/s1/contents/nav.home') {
          rows = rows.filter((r) => r.id !== 'nav.home');
          return jsonResponse(204, undefined);
        }
        return jsonResponse(200, rows.map(asSummary));
      },
    });

    expect(await screen.findByText('nav.home')).toBeTruthy();
    fireEvent.click(screen.getAllByRole('button', { name: 'Удалить item' })[0]!);
    fireEvent.click(screen.getByRole('button', { name: 'Подтвердить' }));

    await waitFor(() => expect(screen.queryByText('nav.home')).toBeNull());
    expect(calls.some((c) => c.method === 'DELETE' && c.url === '/api/sites/s1/contents/nav.home')).toBe(true);
  });

  it('ссылка «Открыть» ведёт в редактор item', async () => {
    await renderApp({
      path: '/sites/s1/contents',
      handler: (url) =>
        url === '/api/sites/s1/contents'
          ? jsonResponse(200, [{ id: 'nav.home', collectionId: 'strings', fields: { title: 'Главная' } }])
          : jsonResponse(200, []),
    });

    const link = (await screen.findAllByRole('link', { name: 'Открыть' }))[0]!;
    expect(link.getAttribute('href')).toBe('/sites/s1/contents/nav.home');
  });
});