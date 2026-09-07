import { afterEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { renderApp, jsonResponse } from '../test-utils';
import { resetContentDrafts } from './content-drafts';

const SITE = { id: 's1', name: 'Alpha', slug: 'alpha', defaultLocale: 'ru', hosts: [] as string[] };

const CONTENTS = [
  {
    id: 'c1',
    siteId: 's1',
    collectionId: 'page',
    fields: { title: 'Home' },
    translations: { en: { fields: { title: 'Главная' } } },
    createdAt: '2025-01-01T00:00:00Z',
    updatedAt: '2026-01-02T00:00:00Z',
  },
  {
    id: 'c2',
    siteId: 's1',
    collectionId: 'region',
    fields: { title: 'Contacts' },
    translations: {},
    createdAt: '2025-01-01T00:00:00Z',
    updatedAt: '2026-02-01T00:00:00Z',
  },
];

const LOCALES = { baseLocale: 'ru', locales: [{ locale: 'en', contentCount: 1 }], total: 1 };

function contentHandler() {
  return (url: string, init: RequestInit) => {
    const u = new URL(url, 'http://localhost');
    const path = u.pathname;
    const method = (init.method ?? 'GET').toUpperCase();
    if (method === 'GET' && path === '/api/sites/s1') return jsonResponse(200, SITE);
    if (method === 'GET' && path === '/api/sites/s1/locales') return jsonResponse(200, LOCALES);
    if (method === 'GET' && path === '/api/sites/s1/contents') return jsonResponse(200, CONTENTS);
    if (method === 'GET' && path === '/api/sites/s1/contents/c1') return jsonResponse(200, CONTENTS[0]);
    if (method === 'PUT' && path === '/api/sites/s1/contents/c1') return jsonResponse(200, {});
    if (method === 'PUT' && path === '/api/sites/s1/contents/c1/translations/en')
      return jsonResponse(200, {});
    if (method === 'DELETE' && path === '/api/sites/s1/contents/c1/translations/en')
      return jsonResponse(200, {});
    return jsonResponse(404, { error: `no mock: ${method} ${path}` });
  };
}

afterEach(() => {
  resetContentDrafts();
});

describe('Контент — список (R3)', () => {
  it('показывает колонки, чипы коллекций, статусы переводов и селектор языка', async () => {
    const { router } = await renderApp({ path: '/sites/s1', handler: contentHandler() });

    expect(await screen.findByRole('heading', { name: 'Контент' })).toBeTruthy();
    expect(await screen.findByText('Home')).toBeTruthy();
    expect(screen.getByText('Contacts')).toBeTruthy();

    expect(screen.getByRole('button', { name: 'page' })).toBeTruthy();
    expect(screen.getByRole('button', { name: 'region' })).toBeTruthy();

    expect(screen.getByText('Переведено на 1')).toBeTruthy();
    expect(screen.getByText('Не переведено')).toBeTruthy();

    const langSelect = screen.getByRole('combobox', { name: 'Язык' });
    expect(within(langSelect).getByRole('option', { name: 'en' })).toBeTruthy();

    expect(router.state.location.search).toBe('');
  });

  it('чип коллекции фильтрует строки, селектор языка — по переводам', async () => {
    await renderApp({ path: '/sites/s1', handler: contentHandler() });
    await screen.findByText('Home');

    fireEvent.click(screen.getByRole('button', { name: 'region' }));
    expect(screen.queryByText('Home')).toBeNull();
    expect(screen.getByText('Contacts')).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: 'Все коллекции' }));
    fireEvent.change(screen.getByRole('combobox', { name: 'Язык' }), { target: { value: 'en' } });
    expect(screen.getByText('Home')).toBeTruthy();
    expect(screen.queryByText('Contacts')).toBeNull();
  });

  it('открытие item ведёт в редактор через ?mode=content&contentId=...', async () => {
    const { router } = await renderApp({ path: '/sites/s1', handler: contentHandler() });
    await screen.findByText('Home');

    fireEvent.click(screen.getByRole('button', { name: 'Home' }));

    expect(router.state.location.search).toBe('?mode=content&contentId=c1');
    expect(await screen.findByRole('heading', { name: 'Home' })).toBeTruthy();
  });
});

describe('Контент — редактор (R3)', () => {
  it('показывает селектор языка (базовый + переводы), поля и блок «Переводы»', async () => {
    await renderApp({ path: '/sites/s1?mode=content&contentId=c1', handler: contentHandler() });

    expect(await screen.findByRole('heading', { name: 'Home' })).toBeTruthy();

    const tabs = screen.getByRole('tablist', { name: 'Язык' });
    expect(within(tabs).getByRole('tab', { name: 'ru' }).getAttribute('aria-selected')).toBe('true');
    expect(within(tabs).getByRole('tab', { name: 'en' })).toBeTruthy();

    const value = screen.getByLabelText('Значение');
    expect((value as HTMLInputElement).value).toBe('Home');

    expect(screen.getAllByText('en').length).toBeGreaterThan(0);
    expect(screen.getByText('Переведено')).toBeTruthy();
  });

  it('сохранение базовых полей шлёт PUT /contents/{contentId}', async () => {
    const { calls } = await renderApp({ path: '/sites/s1?mode=content&contentId=c1', handler: contentHandler() });
    await screen.findByRole('heading', { name: 'Home' });

    const value = screen.getByLabelText('Значение');
    fireEvent.change(value, { target: { value: 'New Home' } });
    expect(screen.getByText('Не сохранено')).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: 'Сохранить поля' }));

    const put = calls.find((c) => c.method === 'PUT' && c.url === '/api/sites/s1/contents/c1');
    expect(put).toBeTruthy();
    expect(JSON.parse(String(put!.init.body))).toEqual({ fields: { title: 'New Home' } });
    expect(await screen.findByText('Сохранено')).toBeTruthy();
  });

  it('draft сохраняется при переключении языка и возвращается обратно', async () => {
    await renderApp({ path: '/sites/s1?mode=content&contentId=c1', handler: contentHandler() });
    await screen.findByRole('heading', { name: 'Home' });

    fireEvent.change(screen.getByLabelText('Значение'), { target: { value: 'Дом' } });
    const tabs = screen.getByRole('tablist', { name: 'Язык' });

    fireEvent.click(within(tabs).getByRole('tab', { name: 'en' }));
    expect((screen.getByLabelText('Значение') as HTMLInputElement).value).toBe('Главная');

    fireEvent.click(within(tabs).getByRole('tab', { name: 'ru' }));
    expect((screen.getByLabelText('Значение') as HTMLInputElement).value).toBe('Дом');
  });

  it('сохранение перевода шлёт PUT .../translations/{locale}', async () => {
    const { calls } = await renderApp({ path: '/sites/s1?mode=content&contentId=c1', handler: contentHandler() });
    await screen.findByRole('heading', { name: 'Home' });

    const tabs = screen.getByRole('tablist', { name: 'Язык' });
    fireEvent.click(within(tabs).getByRole('tab', { name: 'en' }));
    fireEvent.change(await screen.findByLabelText('Значение'), { target: { value: 'Главная страница' } });

    fireEvent.click(screen.getByRole('button', { name: 'Сохранить поля' }));

    const put = calls.find(
      (c) => c.method === 'PUT' && c.url === '/api/sites/s1/contents/c1/translations/en',
    );
    expect(put).toBeTruthy();
    expect(JSON.parse(String(put!.init.body))).toEqual({ fields: { title: 'Главная страница' } });
  });

  it('удаление перевода шлёт DELETE .../translations/{locale} и возвращает к базовому языку', async () => {
    const { calls } = await renderApp({ path: '/sites/s1?mode=content&contentId=c1', handler: contentHandler() });
    await screen.findByRole('heading', { name: 'Home' });

    const tabs = screen.getByRole('tablist', { name: 'Язык' });
    fireEvent.click(within(tabs).getByRole('tab', { name: 'en' }));

    fireEvent.click(screen.getAllByRole('button', { name: 'Удалить перевод' })[0]!);
    fireEvent.click(await screen.findByRole('button', { name: 'Подтвердить' }));

    await waitFor(() =>
      expect(calls.some((c) => c.method === 'DELETE' && c.url === '/api/sites/s1/contents/c1/translations/en')).toBe(true),
    );
    expect(within(screen.getByRole('tablist', { name: 'Язык' })).getByRole('tab', { name: 'ru' }).getAttribute('aria-selected')).toBe('true');
  });

  it('кнопка «К списку» возвращает в список (?mode=content)', async () => {
    const { router } = await renderApp({ path: '/sites/s1?mode=content&contentId=c1', handler: contentHandler() });
    await screen.findByRole('heading', { name: 'Home' });

    fireEvent.click(screen.getByRole('button', { name: '← К списку' }));

    expect(router.state.location.search).toBe('?mode=content');
    expect(await screen.findByRole('heading', { name: 'Контент' })).toBeTruthy();
  });
});