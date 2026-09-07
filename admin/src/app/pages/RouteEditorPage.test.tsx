import { describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import type { Route } from '../../runtime';
import { jsonResponse, renderApp } from '../test-utils';

const REDIRECT: Route = {
  id: 'r2',
  siteId: 's1',
  matcher: '/old',
  priority: 2,
  action: { type: 'redirect', target: '/new', status: 302, keepQuery: true },
};

describe('RouteEditorPage', () => {
  it('загружает роут и сохраняет patch (matcher+priority+action)', async () => {
    const { calls } = await renderApp({
      path: '/sites/s1/routes/r2',
      handler: (url, init) => {
        if (url === '/api/sites/s1/routes' && init.method === 'PUT') return jsonResponse(200, REDIRECT);
        if (url === '/api/sites/s1/routes/r2') return jsonResponse(200, REDIRECT);
        return jsonResponse(200, []);
      },
    });

    expect(await screen.findByText('К списку роутов')).toBeTruthy();
    expect(await screen.findByDisplayValue('/old')).toBeTruthy();
    expect(screen.getByDisplayValue('/new')).toBeTruthy();
    expect(screen.getByDisplayValue('302')).toBeTruthy();
    expect((screen.getByRole('checkbox') as HTMLInputElement).checked).toBe(true);

    fireEvent.change(screen.getByDisplayValue('/old'), { target: { value: '^/legacy$' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

    await waitFor(() => expect(screen.getByText('Сохранено')).toBeTruthy());
    const put = calls.find((c) => c.method === 'PUT' && c.url === '/api/sites/s1/routes/r2');
    expect(JSON.parse(String(put?.init.body))).toEqual({
      matcher: '^/legacy$',
      priority: 2,
      action: { type: 'redirect', target: '/new', status: 302, keepQuery: true },
    });
  });

  it('удаляет роут и возвращается к списку', async () => {
    const { calls } = await renderApp({
      path: '/sites/s1/routes/r1',
      handler: (url, init) => {
        if (url === '/api/sites/s1/routes/r1' && init.method === 'DELETE') return jsonResponse(204, undefined);
        if (url === '/api/sites/s1/routes/r1') return jsonResponse(200, { ...ROUTE_PAGE, id: 'r1' });
        return jsonResponse(200, []);
      },
    });

    expect(await screen.findByText('К списку роутов')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Удалить роут' }));
    fireEvent.click(screen.getByRole('button', { name: 'Подтвердить' }));

    await waitFor(() => expect(screen.getByText('Роутов пока нет')).toBeTruthy());
    expect(calls.some((c) => c.method === 'DELETE' && c.url === '/api/sites/s1/routes/r1')).toBe(true);
  });

  it('ошибка загрузки: сообщение и reload', async () => {
    renderApp({
      path: '/sites/s1/routes/r1',
      handler: (url) => {
        if (url === '/api/sites/s1/routes/r1') return jsonResponse(500, { error: 'backend down' });
        return jsonResponse(200, []);
      },
    });

    expect(await screen.findByText(/backend down/)).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Обновить' })).toBeTruthy();
  });
});

const ROUTE_PAGE: Route = {
  id: 'r1',
  siteId: 's1',
  matcher: '/page',
  priority: 1,
  action: { type: 'renderPage', pageId: 'p1' },
};