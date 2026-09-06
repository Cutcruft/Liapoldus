import { describe, expect, it } from 'vitest';
import { fireEvent, screen } from '@testing-library/react';
import { renderApp, jsonResponse } from '../test-utils';

const SETTINGS = { adminToken: '····abcd', defaultLocale: 'ru', redirectDefaultStatus: 301 };

describe('SettingsPage', () => {
  it('показывает маскированный токен, локаль, код редиректа и версию', async () => {
    await renderApp({ path: '/settings', handler: () => jsonResponse(200, SETTINGS) });
    expect(await screen.findByText('····abcd')).toBeTruthy();
    expect(screen.getByLabelText('Локаль по умолчанию')).toBeTruthy();
    expect((screen.getByLabelText('Локаль по умолчанию') as HTMLSelectElement).disabled).toBe(true);
    const status = screen.getByLabelText('Код редиректа по умолчанию') as HTMLSelectElement;
    expect(status.value).toBe('301');
    expect(screen.getByText('Управляется конфигурацией сервера')).toBeTruthy();
    expect(screen.getAllByText(/0\.1\.0 \(MVP\)/).length).toBeGreaterThan(0);
  });

  it('ротация токена: невалидный → ошибка', async () => {
    await renderApp({
      path: '/settings',
      handler: (url) =>
        url === '/api/auth/validate' ? jsonResponse(401, { error: 'unauthorized' }) : jsonResponse(200, SETTINGS),
    });
    await screen.findByText('····abcd');
    fireEvent.change(screen.getByLabelText('Новый токен'), { target: { value: 'nope' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));
    expect(await screen.findByText('Токен не подходит')).toBeTruthy();
  });

  it('ротация токена: валидный → сохранён в store', async () => {
    const { tokenStore } = await renderApp({
      path: '/settings',
      handler: (url) =>
        url === '/api/auth/validate' ? jsonResponse(200, { valid: true }) : jsonResponse(200, SETTINGS),
    });
    await screen.findByText('····abcd');
    fireEvent.change(screen.getByLabelText('Новый токен'), { target: { value: 'fresh' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));
    expect(await screen.findByText('Токен обновлён')).toBeTruthy();
    expect(tokenStore.getState().token).toBe('fresh');
  });
});