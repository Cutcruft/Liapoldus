import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { AdminProvider } from './admin-context';
import { appRoutes } from './AppRoutes';
import { createAdminApi, createTokenStore } from '../runtime';

function renderApp(path: string) {
  const router = createMemoryRouter(appRoutes, { initialEntries: [path] });
  const tokenStore = createTokenStore(null);
  const api = createAdminApi({
    baseUrl: '',
    getToken: () => null,
    fetchFn: async () => new Response('{}', { status: 200, headers: { 'Content-Type': 'application/json' } }),
  });
  return render(
    <AdminProvider api={api} tokenStore={tokenStore}>
      <RouterProvider router={router} />
    </AdminProvider>,
  );
}

describe('AppShell', () => {
  it('рендерит навигацию и дашборд на /', () => {
    renderApp('/');
    expect(screen.getByRole('navigation', { name: 'Главное меню' })).toBeTruthy();
    expect(screen.getByText('Сайты')).toBeTruthy();
    expect(screen.getByRole('heading', { name: 'Обзор' })).toBeTruthy();
  });

  it('на /sites показывает заголовок «Сайты»', () => {
    renderApp('/sites');
    expect(screen.getByRole('heading', { name: 'Сайты' })).toBeTruthy();
  });

  it('site-скоп маршруты открываются (pages)', () => {
    renderApp('/sites/s1/pages');
    expect(screen.getByRole('heading', { name: 'Страницы' })).toBeTruthy();
  });
});