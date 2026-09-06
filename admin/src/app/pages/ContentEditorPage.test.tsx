import { describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { jsonResponse, renderApp } from '../test-utils';

const SITE = { id: 's1', name: 'Demo', slug: 'demo', defaultLocale: 'ru', hosts: [] };

const DETAIL = {
  id: 'abc1',
  siteId: 's1',
  collectionId: 'strings',
  fields: { title: 'Главная', count: 1 },
  translations: { en: { fields: { title: 'Home' } } },
};

describe('ContentEditorPage', () => {
  it('показывает базовые поля и локаль перевода', async () => {
    renderApp({
      path: '/sites/s1/contents/abc1',
      handler: (url) => {
        if (url === '/api/sites/s1') return jsonResponse(200, SITE);
        if (url === '/api/sites/s1/contents/abc1') return jsonResponse(200, DETAIL);
        return jsonResponse(200, []);
      },
    });

    expect(await screen.findByText('abc1')).toBeTruthy();
    expect(screen.getByText('Язык по умолчанию (ru) — база для остальных локлей')).toBeTruthy();

    await waitFor(() => {
      const values = screen.getAllByLabelText('Значение');
      expect((values[0] as HTMLInputElement).value).toBe('Главная');
      expect((values[1] as HTMLInputElement).value).toBe('1');
    });

    fireEvent.click(screen.getByRole('button', { name: 'en' }));
    const enValues = screen.getAllByLabelText('Значение');
    expect((enValues[enValues.length - 1] as HTMLInputElement).value).toBe('Home');
  });

  it('правка базового поля → PUT /api/sites/s1/contents/abc1 {fields}', async () => {
    const detail = { id: 'abc1', siteId: 's1', collectionId: 'strings', fields: { title: 'Главная', count: 1 }, translations: {} };
    const { calls } = await renderApp({
      path: '/sites/s1/contents/abc1',
      handler: (url) => {
        if (url === '/api/sites/s1') return jsonResponse(200, SITE);
        if (url === '/api/sites/s1/contents/abc1') return jsonResponse(200, detail);
        return jsonResponse(200, []);
      },
    });

    expect(await screen.findByText('abc1')).toBeTruthy();
    await waitFor(() => expect(screen.getAllByLabelText('Значение').length).toBe(2));
    const baseValue = screen.getAllByLabelText('Значение')[0] as HTMLInputElement;
    fireEvent.change(baseValue, { target: { value: 'Главная v2' } });
    fireEvent.click(screen.getAllByRole('button', { name: 'Сохранить поля' })[0]!);

    const put = calls.find((c) => c.method === 'PUT' && c.url === '/api/sites/s1/contents/abc1');
    expect(JSON.parse(String(put?.init.body))).toEqual({ fields: { title: 'Главная v2', count: 1 } });
    expect(await screen.findByText('Сохранено')).toBeTruthy();
  });

  it('добавляет локаль и сохраняет перевод', async () => {
    const { calls } = await renderApp({
      path: '/sites/s1/contents/abc1',
      handler: (url) => {
        if (url === '/api/sites/s1') return jsonResponse(200, SITE);
        if (url === '/api/sites/s1/contents/abc1') return jsonResponse(200, DETAIL);
        return jsonResponse(200, []);
      },
    });

    expect(await screen.findByText('abc1')).toBeTruthy();
    fireEvent.change(screen.getByLabelText('Локаль'), { target: { value: 'de' } });
    fireEvent.click(screen.getByRole('button', { name: 'Добавить локаль' }));

    expect(screen.getByRole('button', { name: 'de' })).toBeTruthy();
    fireEvent.click(screen.getAllByRole('button', { name: 'Сохранить поля' }).at(-1)!);

    const put = calls.find((c) => c.method === 'PUT' && c.url === '/api/sites/s1/contents/abc1/translations/de');
    expect(JSON.parse(String(put?.init.body))).toEqual({ fields: {} });
  });

  it('удаляет локаль через подтверждение', async () => {
    const { calls } = await renderApp({
      path: '/sites/s1/contents/abc1',
      handler: (url) => {
        if (url === '/api/sites/s1') return jsonResponse(200, SITE);
        if (url === '/api/sites/s1/contents/abc1') return jsonResponse(200, DETAIL);
        return jsonResponse(200, []);
      },
    });

    expect(await screen.findByText('abc1')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Удалить перевод' }));
    fireEvent.click(screen.getByRole('button', { name: 'Подтвердить' }));

    expect(calls.some((c) => c.method === 'DELETE' && c.url === '/api/sites/s1/contents/abc1/translations/en')).toBe(true);
  });
});