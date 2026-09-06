import { describe, expect, it } from 'vitest';
import { fireEvent, screen } from '@testing-library/react';
import { renderApp, jsonResponse } from '../test-utils';

describe('LoginPage', () => {
  it('пустой токен → «Введите токен» без запроса', async () => {
    const { calls } = await renderApp({ path: '/login', token: null });
    fireEvent.click(screen.getByRole('button', { name: 'Войти' }));
    expect(await screen.findByText('Введите токен')).toBeTruthy();
    expect(calls).toHaveLength(0);
  });

  it('невалидный токен → «Неверный токен»', async () => {
    const { calls } = await renderApp({
      path: '/login',
      token: null,
      handler: () => jsonResponse(401, { error: 'unauthorized' }),
    });
    fireEvent.change(screen.getByLabelText('Токен'), { target: { value: 'bad' } });
    fireEvent.click(screen.getByRole('button', { name: 'Войти' }));
    expect(await screen.findByText('Неверный токен')).toBeTruthy();
    expect(calls.some((c) => c.url === '/api/auth/validate' && c.method === 'POST')).toBe(true);
  });

  it('валидный токен → вход на дашборд, токен сохранён', async () => {
    const { tokenStore } = await renderApp({
      path: '/login',
      token: null,
      handler: (url) =>
        url === '/api/auth/validate'
          ? jsonResponse(200, { valid: true })
          : jsonResponse(200, { siteCount: 0, sites: [], recentBuilds: [], recentSnapshots: [], runtimeStatus: 'no-builds' }),
    });
    fireEvent.change(screen.getByLabelText('Токен'), { target: { value: 'secret' } });
    fireEvent.click(screen.getByRole('button', { name: 'Войти' }));
    expect(await screen.findByRole('heading', { name: 'Сайты' })).toBeTruthy();
    expect(tokenStore.getState().token).toBe('secret');
  });
});