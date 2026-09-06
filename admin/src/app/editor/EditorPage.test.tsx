import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import type { Page } from '../../runtime';
import { jsonResponse, renderApp } from '../test-utils';
import { setDevSocketFactory, type WsLike } from './dev-ws';

function inertSocket(): WsLike {
  return { onmessage: null, onopen: null, onclose: null, onerror: null, close: () => {} };
}

beforeEach(() => {
  setDevSocketFactory(() => inertSocket());
  localStorage.clear();
});

const PAGE: Page = {
  id: 'p1',
  siteId: 's1',
  name: 'Главная',
  slug: 'index',
  version: 3,
  root: {
    id: 'root',
    type: 'Container',
    props: { layout: 'stack', gap: 8 },
    children: [{ id: 't1', type: 'Text', props: { text: 'Привет', size: 'md', align: 'left' }, bindings: {} }],
  },
};

const handler = (put: { root: unknown; version: number }) => (url: string, init: RequestInit) => {
  if (url === '/api/pages/p1' && init.method === 'GET') return jsonResponse(200, PAGE);
  if (url === '/api/pages/p1/tree' && init.method === 'PUT') {
    put.root = JSON.parse(String(init.body))['root'];
    put.version += 1;
    return jsonResponse(200, { version: put.version });
  }
  return jsonResponse(404, { error: 'not found' });
};

describe('EditorPage', () => {
  it('загружает страницу: дерево, инспектор, канвас', async () => {
    const put = { root: null as unknown, version: 3 };
    await renderApp({ path: '/sites/s1/pages/p1', handler: handler(put) });

    expect(await screen.findByText('Привет')).toBeTruthy();
    expect(screen.getByText('Container')).toBeTruthy();
    expect(screen.getByText('Text', { exact: true })).toBeTruthy();
    expect(screen.getByText('· Привет')).toBeTruthy();
    expect(screen.getByText('Свойства · Container')).toBeTruthy();

    fireEvent.click(screen.getByText('· Привет'));
    expect(await screen.findByText('Свойства · Text')).toBeTruthy();
  });

  it('правка пропа → автосейв PUT saveTree + обновление версии', async () => {
    const put = { root: null as unknown, version: 3 };
    const { calls } = await renderApp({ path: '/sites/s1/pages/p1', handler: handler(put) });

    fireEvent.click(await screen.findByText('· Привет'));
    const input = screen.getByDisplayValue('Привет');
    fireEvent.change(input, { target: { value: 'Новый заголовок' } });

    await waitFor(
      () => {
        const save = calls.find((c) => c.method === 'PUT' && c.url === '/api/pages/p1/tree');
        expect(save).toBeTruthy();
        const body = JSON.parse(String(save?.init.body)) as { root: Page['root'] };
        expect(body.root.children?.[0]?.props?.['text']).toBe('Новый заголовок');
      },
      { timeout: 3000 },
    );
    expect(await screen.findByText('Сохранено, v4')).toBeTruthy();
    expect(screen.queryByText('Есть несохранённые изменения')).toBeNull();
  });

  it('добавление узла через меню +, undo возвращает назад, redo повторяет', async () => {
    const put = { root: null as unknown, version: 3 };
    await renderApp({ path: '/sites/s1/pages/p1', handler: handler(put) });

    fireEvent.click(await screen.findByTitle('Добавить'));
    fireEvent.click(await screen.findByText('+ Текст'));

    // canvas отражает живое дерево (design-mode)
    expect(screen.getAllByText('Текст').length).toBeGreaterThanOrEqual(2);

    fireEvent.click(screen.getByTitle('Назад'));
    expect(screen.getAllByText('Привет').length).toBe(1);
    expect(screen.queryByText('· Текст')).toBeNull();

    fireEvent.click(screen.getByTitle('Вперёд'));
    expect(screen.getByText('· Текст')).toBeTruthy();
  });

  it('binding к контенту: выбирается источник + путь, уходит в PUT', async () => {
    const put = { root: null as unknown, version: 3 };
    const { calls } = await renderApp({ path: '/sites/s1/pages/p1', handler: handler(put) });

    fireEvent.click(await screen.findByText('· Привет'));
    const sourceSelect = screen.getAllByRole('combobox')[0]!;
    fireEvent.change(sourceSelect, { target: { value: 'content' } });
    const pathInput = screen.getByLabelText('Путь к данным');
    fireEvent.change(pathInput, { target: { value: 'strings.hero' } });

    await waitFor(
      () => {
        const save = calls.find((c) => c.method === 'PUT' && c.url === '/api/pages/p1/tree');
        expect(save).toBeTruthy();
        const body = JSON.parse(String(save?.init.body)) as { root: Page['root'] };
        expect(body.root.children?.[0]?.bindings?.['text']).toEqual({ source: 'content', path: 'strings.hero' });
      },
      { timeout: 3000 },
    );
  });

  it('удаление узла убирает его из дерева и канваса', async () => {
    const put = { root: null as unknown, version: 3 };
    await renderApp({ path: '/sites/s1/pages/p1', handler: handler(put) });

    const removeButton = (await screen.findAllByTitle('Удалить'))[0]!;
    fireEvent.click(removeButton);
    await waitFor(() => expect(screen.queryByText('Привет')).toBeNull());
    expect(screen.queryByText('· Привет')).toBeNull();
    void put;
  });

  it('ошибка загрузки → сообщение + reload', async () => {
    let attempts = 0;
    await renderApp({
      path: '/sites/s1/pages/p1',
      handler: () => {
        attempts += 1;
        if (attempts === 1) return jsonResponse(500, { error: 'boom' });
        return jsonResponse(200, PAGE);
      },
    });

    expect(await screen.findByText(/Ошибка/)).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Обновить' }));
    expect(await screen.findByText('Привет')).toBeTruthy();
  });

  it('выбор ассета через пикер в инспекторе пишет assetId и уходит в PUT', async () => {
    const put = { root: null as unknown, version: 3 };
    const imagePage: Page = {
      ...PAGE,
      root: {
        id: 'root',
        type: 'Container',
        props: {},
        children: [{ id: 'i1', type: 'Image', props: { assetId: '', alt: '', width: 320 }, bindings: {} }],
      },
    };
    const { calls } = await renderApp({
      path: '/sites/s1/pages/p1',
      handler: (url, init) => {
        if (url === '/api/pages/p1' && init.method === 'GET') return jsonResponse(200, imagePage);
        if (url === '/api/pages/p1/tree' && init.method === 'PUT') {
          put.root = JSON.parse(String(init.body))['root'];
          put.version += 1;
          return jsonResponse(200, { version: put.version });
        }
        if (url === '/api/sites/s1/assets') {
          return jsonResponse(200, [
            {
              id: 'logo1',
              siteId: 's1',
              name: 'logo.png',
              mime: 'image/png',
              size: 1024,
              variants: [],
            },
          ]);
        }
        if (url === '/api/assets/logo1') {
          return jsonResponse(200, {
            id: 'logo1',
            siteId: 's1',
            name: 'logo.png',
            mime: 'image/png',
            size: 1024,
            variants: [],
          });
        }
        return jsonResponse(404, { error: 'not found' });
      },
    });

    fireEvent.click(await screen.findByText('Image'));
    fireEvent.click(screen.getByRole('button', { name: 'Выбрать ассет' }));

    const item = await screen.findByTestId('asset-picker-item');
    fireEvent.click(item);

    await waitFor(
      () => {
        const save = calls.find((c) => c.method === 'PUT' && c.url === '/api/pages/p1/tree');
        expect(save).toBeTruthy();
        const body = JSON.parse(String(save?.init.body)) as { root: Page['root'] };
        expect(body.root.children?.[0]?.props?.['assetId']).toBe('logo1');
      },
      { timeout: 3000 },
    );
    // превью выбранного ассета в инспекторе (getAsset → имя + миниатюра)
    expect(await screen.findByAltText('logo.png')).toBeTruthy();
    void put;
  });

  it('после автосейва собирается dev build; таб Превью показывает iframe', async () => {
    const put = { root: null as unknown, version: 3 };
    const { calls } = await renderApp({
      path: '/sites/s1/pages/p1',
      handler: (url, init) => {
        if (url === '/api/pages/p1' && init.method === 'GET') return jsonResponse(200, PAGE);
        if (url === '/api/pages/p1/tree' && init.method === 'PUT') {
          put.root = JSON.parse(String(init.body))['root'];
          put.version += 1;
          return jsonResponse(200, { version: put.version });
        }
        if (url === '/api/sites/s1/snapshots' && init.method === 'POST') {
          const body = JSON.parse(String(init.body)) as { name: string };
          return jsonResponse(201, { id: 'snap1', siteId: 's1', name: body.name });
        }
        if (url === '/api/sites/s1/builds' && init.method === 'POST') {
          return jsonResponse(201, {
            id: 'b1',
            siteId: 's1',
            snapshotId: 'snap1',
            environment: 'development',
            status: 'ready',
            log: [],
            artifactDir: '',
            createdAt: new Date().toISOString(),
          });
        }
        if (init.method === 'DELETE') return jsonResponse(204, undefined);
        return jsonResponse(404, { error: 'not found' });
      },
    });

    fireEvent.click(await screen.findByText('· Привет'));
    fireEvent.change(screen.getByDisplayValue('Привет'), { target: { value: 'Заголовок' } });
    await waitFor(
      () => expect(calls.some((c) => c.method === 'PUT' && c.url === '/api/pages/p1/tree')).toBe(true),
      { timeout: 3000 },
    );
    await waitFor(
      () => expect(calls.some((c) => c.method === 'POST' && c.url === '/api/sites/s1/builds')).toBe(true),
      { timeout: 3000 },
    );

    fireEvent.click(screen.getByRole('button', { name: 'Превью' }));
    const iframe = (await screen.findByTitle('Превью готово')) as unknown as HTMLElement;
    void iframe;
    await waitFor(
      () => {
        const node = document.querySelector('iframe') as HTMLIFrameElement | null;
        expect(node?.getAttribute('src')).toBe('/build/s1/development/snap1/dist/index.html');
      },
      { timeout: 3000 },
    );
    void put;
  });
});