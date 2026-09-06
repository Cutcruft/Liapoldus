import { describe, expect, it } from 'vitest';
import { fireEvent, screen } from '@testing-library/react';
import { renderApp } from './test-utils';

describe('AppShell', () => {
  it('рендерит навигацию и дашборд на /', async () => {
    await renderApp({ path: '/' });
    expect(screen.getByRole('navigation', { name: 'Главное меню' })).toBeTruthy();
    expect(screen.getByText('Сайты')).toBeTruthy();
    expect(screen.getByRole('heading', { name: 'Обзор' })).toBeTruthy();
  });

  it('на /sites показывает заголовок «Сайты»', async () => {
    await renderApp({ path: '/sites' });
    expect(screen.getByRole('heading', { name: 'Сайты' })).toBeTruthy();
  });

  it('site-скоп маршруты открываются (pages)', async () => {
    await renderApp({ path: '/sites/s1/pages' });
    expect(screen.getByRole('heading', { name: 'Страницы' })).toBeTruthy();
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