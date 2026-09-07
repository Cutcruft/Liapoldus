import { afterEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import type { AssetMeta } from '../../runtime';
import { jsonResponse, renderApp } from '../test-utils';
import { resetSiteTabStore } from '../site-tab-store';

const SITE = { id: 's1', name: 'Alpha', slug: 'alpha', defaultLocale: 'ru', hosts: [] as string[] };

const ASSETS: AssetMeta[] = [
  { id: 'a1', siteId: 's1', name: 'logo.png', mime: 'image/png', size: 1024, variants: [] },
  { id: 'a2', siteId: 's1', name: 'data.json', mime: 'application/json', size: 2048, variants: [] },
];

const USAGE = {
  assetId: 'a1',
  contents: [{ id: 'c1', collectionId: 'page' }],
  forms: [{ id: 'form.contact', name: 'Contact' }],
};

function mediaHandler() {
  let assets = [...ASSETS];
  return (url: string, init: RequestInit) => {
    const u = new URL(url, 'http://localhost');
    const path = u.pathname;
    const method = (init.method ?? 'GET').toUpperCase();
    if (method === 'GET' && path === '/api/sites/s1') return jsonResponse(200, SITE);
    if (method === 'GET' && path === '/api/sites/s1/assets') return jsonResponse(200, assets);
    if (method === 'POST' && path === '/api/sites/s1/assets') {
      const created: AssetMeta = { id: 'a3', siteId: 's1', name: 'upload.png', mime: 'image/png', size: 10, variants: [] };
      assets = [...assets, created];
      return jsonResponse(201, created);
    }
    if (method === 'GET' && path === '/api/sites/s1/assets/a1/usage') return jsonResponse(200, USAGE);
    if (method === 'DELETE' && path.startsWith('/api/assets/')) {
      const id = path.split('/').pop()!;
      assets = assets.filter((a) => a.id !== id);
      return jsonResponse(204, undefined);
    }
    return jsonResponse(404, { error: `no mock: ${method} ${path}` });
  };
}

afterEach(() => {
  resetSiteTabStore();
});

function cards() {
  return screen.queryAllByTestId('media-card');
}

describe('Медиа — грид и фильтры (R4)', () => {
  it('показывает грид ассетов: имя, тип, размер', async () => {
    await renderApp({ path: '/sites/s1?mode=media', handler: mediaHandler() });

    expect(await screen.findByRole('heading', { name: 'Медиа' })).toBeTruthy();
    expect(await screen.findByText('logo.png')).toBeTruthy();
    expect(screen.getByText('data.json')).toBeTruthy();
    const card = cards()[0]!;
    expect(card.textContent).toContain('image/png');
    expect(card.textContent).toContain('1.0 kB');
    expect(cards().length).toBe(2);
  });

  it('поиск фильтрует по имени', async () => {
    await renderApp({ path: '/sites/s1?mode=media', handler: mediaHandler() });
    await screen.findByText('logo.png');

    fireEvent.change(screen.getByLabelText('Поиск'), { target: { value: 'data' } });
    expect(screen.queryByText('logo.png')).toBeNull();
    expect(screen.getByText('data.json')).toBeTruthy();
  });

  it('фильтр по типу (image) оставляет только картинки', async () => {
    await renderApp({ path: '/sites/s1?mode=media', handler: mediaHandler() });
    await screen.findByText('logo.png');

    fireEvent.change(screen.getByRole('combobox', { name: 'Тип' }), { target: { value: 'image' } });
    expect(screen.getByText('logo.png')).toBeTruthy();
    expect(screen.queryByText('data.json')).toBeNull();
  });
});

describe('Медиа — загрузка и действия (R4)', () => {
  it('загрузка файла шлёт POST /assets с FormData и добавляет карточку', async () => {
    const { calls } = await renderApp({ path: '/sites/s1?mode=media', handler: mediaHandler() });

    fireEvent.click(await screen.findByRole('button', { name: 'Загрузить' }));
    const file = new File(['bytes'], 'upload.png', { type: 'image/png' });
    fireEvent.change(screen.getByLabelText('Файл *'), { target: { files: [file] } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    expect(await screen.findByText('upload.png')).toBeTruthy();
    const post = calls.find((c) => c.method === 'POST' && c.url === '/api/sites/s1/assets');
    expect(post).toBeTruthy();
    expect(post!.init.body).toBeInstanceOf(FormData);
    expect((post!.init.body as FormData).get('file')).toBe(file);
  });

  it('удаление ассета: DELETE и карточка исчезает', async () => {
    const { calls } = await renderApp({ path: '/sites/s1?mode=media', handler: mediaHandler() });

    fireEvent.click((await screen.findAllByRole('button', { name: 'Удалить ассет' }))[0]!);
    fireEvent.click(screen.getByRole('button', { name: 'Подтвердить' }));

    await waitFor(() => expect(screen.queryByText('logo.png')).toBeNull());
    expect(calls.some((c) => c.method === 'DELETE' && c.url === '/api/assets/a1')).toBe(true);
  });

  it('«Где используется» показывает контент и формы по /usage', async () => {
    const { calls } = await renderApp({ path: '/sites/s1?mode=media', handler: mediaHandler() });
    await screen.findByText('logo.png');

    fireEvent.click(screen.getAllByRole('button', { name: 'Где используется' })[0]!);

    expect(await screen.findByText(/Где используется: logo.png/)).toBeTruthy();
    const panel = screen.getByText(/Где используется: logo.png/).closest('div')?.parentElement ?? document.body;
    expect(within(panel).getByText('Контент')).toBeTruthy();
    expect(within(panel).getByText('c1')).toBeTruthy();
    expect(within(panel).getByText('page')).toBeTruthy();
    expect(within(panel).getByText('Формы')).toBeTruthy();
    expect(within(panel).getByText('Contact')).toBeTruthy();
    expect(calls.some((c) => c.method === 'GET' && c.url === '/api/sites/s1/assets/a1/usage')).toBe(true);

    fireEvent.click(screen.getByRole('button', { name: 'Скрыть' }));
    expect(screen.queryByText(/Где используется: logo.png/)).toBeNull();
  });

  it('копирование URL меняет подпись кнопки на «URL скопирован»', async () => {
    await renderApp({ path: '/sites/s1?mode=media', handler: mediaHandler() });
    await screen.findByText('logo.png');

    fireEvent.click(screen.getAllByRole('button', { name: 'Копировать ссылку' })[0]!);

    await waitFor(() => expect(screen.getAllByRole('button', { name: 'URL скопирован' }).length).toBe(1));
  });
});