import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen } from '@testing-library/react';
import { renderApp, jsonResponse } from './test-utils';
import { resetTabs } from './tabs-store';

const DASHBOARD = {
  siteCount: 1,
  sites: [
    {
      siteId: 's1',
      name: 'Alpha',
      slug: 'alpha',
      defaultLocale: 'ru',
      hosts: ['alpha.test'],
      git: { dev: { existing: false }, main: { existing: false }, dirty: false },
    },
  ],
  recentBuilds: [],
  recentSnapshots: [],
  runtimeStatus: 'no-builds',
};

const SITE = { id: 's1', name: 'Alpha', slug: 'alpha', defaultLocale: 'ru', hosts: ['alpha.test'] };

const SETTINGS = { adminToken: '***', defaultLocale: 'ru', redirectDefaultStatus: 301 };

function routeHandler(url: string) {
  if (url === '/api/dashboard') return jsonResponse(200, DASHBOARD);
  if (url === '/api/sites/s1') return jsonResponse(200, SITE);
  if (url === '/api/settings') return jsonResponse(200, SETTINGS);
  return jsonResponse(404, { error: 'not found' });
}

describe('AppShell (R1: каркас + табы)', () => {
  beforeEach(() => resetTabs());

  it('рендерит бренд, трей вкладок и главную-карточки на /', async () => {
    await renderApp({ path: '/' });
    expect(await screen.findByRole('heading', { name: 'Сайты' })).toBeTruthy();
    expect(screen.getByText('Liapoldus')).toBeTruthy();
    expect(screen.getByRole('tablist')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Создать сайт' })).toBeTruthy();
  });

  it('клик по карточке сайта открывает вкладку /sites/:siteId', async () => {
    const { calls } = await renderApp({ path: '/', handler: routeHandler });
    fireEvent.click(await screen.findByRole('button', { name: /Alpha/ }));
    expect(await screen.findByRole('heading', { name: 'Alpha' })).toBeTruthy();
    expect(screen.getByRole('tab', { name: /Alpha/ })).toBeTruthy();
    expect(calls.some((c) => c.url === '/api/sites/s1')).toBe(true);
  });

  it('«Системные настройки» открывает вкладку /settings', async () => {
    await renderApp({ path: '/', handler: routeHandler });
    fireEvent.click(await screen.findByRole('button', { name: 'Системные настройки' }));
    expect(await screen.findByRole('tab', { name: /Настройки/ })).toBeTruthy();
    expect(screen.getByRole('heading', { name: 'Настройки' })).toBeTruthy();
  });

  it('прямой переход по /sites/:siteId восстанавливает вкладку и подтягивает имя', async () => {
    await renderApp({ path: '/sites/s1', handler: routeHandler });
    expect(await screen.findByRole('heading', { name: 'Alpha' })).toBeTruthy();
    expect(screen.getByRole('tab', { name: /Alpha/ })).toBeTruthy();
  });

  it('закрытие активной вкладки возвращает на главную', async () => {
    await renderApp({ path: '/sites/s1', handler: routeHandler });
    expect(await screen.findByRole('tab', { name: /Alpha/ })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Закрыть вкладку' }));
    expect(await screen.findByRole('heading', { name: 'Сайты' })).toBeTruthy();
  });

  it('без токена редиректит на /login', async () => {
    await renderApp({ path: '/', token: null });
    expect(await screen.findByText('Вход в админку')).toBeTruthy();
  });

  it('«Выйти» очищает токен и ведёт на /login', async () => {
    const { tokenStore } = await renderApp({ path: '/' });
    expect(tokenStore.getState().token).toBe('test-token');
    fireEvent.click(screen.getByRole('button', { name: 'Выйти' }));
    expect(await screen.findByText('Вход в админку')).toBeTruthy();
    expect(tokenStore.getState().token).toBe(null);
  });
});