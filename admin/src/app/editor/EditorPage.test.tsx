import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import type { ElementNode, Page } from '../../runtime';
import { jsonResponse, renderApp } from '../test-utils';
import { setDevSocketFactory, type WsLike } from '../ws-client';
import { __lastRichTextEditor } from '../rich-text/RichTextEditor';

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
  list: [
    { id: 't1', componentId: 'Text', props: { text: { kind: 'literal', value: 'Привет' }, size: { kind: 'literal', value: 'md' }, align: { kind: 'literal', value: 'left' } } },
  ],
};

const handler = (put: { list: unknown; version: number }) => (url: string, init: RequestInit) => {
  if (url === '/api/pages/p1' && init.method === 'GET') return jsonResponse(200, PAGE);
  if (url === '/api/pages/p1' && init.method === 'PUT') {
    put.list = JSON.parse(String(init.body))['list'];
    put.version += 1;
    return jsonResponse(200, { version: put.version });
  }
  return jsonResponse(404, { error: 'not found' });
};

describe('EditorPage', () => {
  it('загружает страницу: лист, инспектор, канвас', async () => {
    const put = { list: null as unknown, version: 3 };
    await renderApp({ path: '/sites/s1/pages/p1', handler: handler(put) });

    expect(await screen.findByText('· Привет')).toBeTruthy();
    expect(screen.getByText('Text', { exact: true })).toBeTruthy();
    expect(screen.getByText('· Привет')).toBeTruthy();
    expect(screen.getByText('Свойства · Text')).toBeTruthy();
  });

  it('правка пропа → автосейв PUT updatePage + обновление версии', async () => {
    const put = { list: null as unknown, version: 3 };
    const { calls } = await renderApp({ path: '/sites/s1/pages/p1', handler: handler(put) });

    fireEvent.click(await screen.findByText('· Привет'));
    await waitFor(() => expect(__lastRichTextEditor()).toBeTruthy());
    const editor = __lastRichTextEditor();
    expect(editor).toBeTruthy();
    editor?.chain().focus().selectAll().deleteSelection().insertContent('Новый заголовок').run();

    await waitFor(
      () => {
        const save = calls.find((c) => c.method === 'PUT' && c.url === '/api/pages/p1');
        expect(save).toBeTruthy();
        const body = JSON.parse(String(save?.init.body)) as { list: ElementNode[] };
        expect(body.list[0]?.props['text']).toEqual({ kind: 'literal', value: expect.stringContaining('Новый заголовок') });
        expect(String((body.list[0]?.props['text'] as { value?: unknown }).value)).toMatch(/^<p>/);
      },
      { timeout: 3000 },
    );
    expect(await screen.findByText('Сохранено, v4')).toBeTruthy();
    expect(screen.queryByText('Есть несохранённые изменения')).toBeNull();
  });

  it('добавление элемента через меню +, undo возвращает назад, redo повторяет', async () => {
    const put = { list: null as unknown, version: 3 };
    await renderApp({ path: '/sites/s1/pages/p1', handler: handler(put) });

    fireEvent.click(await screen.findByTitle('Добавить'));
    fireEvent.click(await screen.findByText('+ Текст'));

    // добавленный элемент появился на канвасе вторым data-node="Text"
    await waitFor(() => expect(document.querySelectorAll('[data-node="Text"]')).toHaveLength(2));

    fireEvent.click(screen.getByTitle('Назад'));
    expect(screen.queryByText('· Текст')).toBeNull();
    await waitFor(() => expect(document.querySelectorAll('[data-node="Text"]')).toHaveLength(1));

    fireEvent.click(screen.getByTitle('Вперёд'));
    await waitFor(() => expect(document.querySelectorAll('[data-node="Text"]')).toHaveLength(2));
    expect(screen.getByText('· Текст')).toBeTruthy();
  });

  it('binding к контенту: выбирается источник + путь, уходит в PUT', async () => {
    const put = { list: null as unknown, version: 3 };
    const { calls } = await renderApp({ path: '/sites/s1/pages/p1', handler: handler(put) });

    fireEvent.click(await screen.findByText('· Привет'));
    const propsPanel = screen.getByRole('region', { name: 'Свойства' });
    const sourceSelect = within(propsPanel).getAllByRole('combobox')[0]!;
    fireEvent.change(sourceSelect, { target: { value: 'content' } });
    const pathInput = within(propsPanel).getByLabelText('Путь к данным');
    fireEvent.change(pathInput, { target: { value: 'strings.hero' } });

    await waitFor(
      () => {
        const save = calls.find((c) => c.method === 'PUT' && c.url === '/api/pages/p1');
        expect(save).toBeTruthy();
        const body = JSON.parse(String(save?.init.body)) as { list: ElementNode[] };
        expect(body.list[0]?.props['text']).toEqual({
          kind: 'binding',
          source: { kind: 'content', contentId: 'strings', field: 'hero' },
        });
      },
      { timeout: 3000 },
    );
  });

  it('удаление элемента убирает его из листа и канваса', async () => {
    const put = { list: null as unknown, version: 3 };
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
    expect(await screen.findByText('· Привет')).toBeTruthy();
  });

  it('выбор ассета через пикер в инспекторе пишет assetId и уходит в PUT', async () => {
    const put = { list: null as unknown, version: 3 };
    const imagePage: Page = {
      ...PAGE,
      list: [
        { id: 'i1', componentId: 'Image', props: { assetId: { kind: 'literal', value: '' }, alt: { kind: 'literal', value: '' }, width: { kind: 'literal', value: 320 } } },
      ],
    };
    const { calls } = await renderApp({
      path: '/sites/s1/pages/p1',
      handler: (url, init) => {
        if (url === '/api/pages/p1' && init.method === 'GET') return jsonResponse(200, imagePage);
        if (url === '/api/pages/p1' && init.method === 'PUT') {
          put.list = JSON.parse(String(init.body))['list'];
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
        const save = calls.find((c) => c.method === 'PUT' && c.url === '/api/pages/p1');
        expect(save).toBeTruthy();
        const body = JSON.parse(String(save?.init.body)) as { list: ElementNode[] };
        expect(body.list[0]?.props['assetId']).toEqual({ kind: 'literal', value: 'logo1' });
      },
      { timeout: 3000 },
    );
    // превью выбранного ассета в инспекторе (getAsset → имя + миниатюра)
    expect(await screen.findByAltText('logo.png')).toBeTruthy();
    void put;
  });

  it('панель Страница: выбор каркаса и title уходят в PUT как layoutSectionId/head', async () => {
    const put = { list: null as unknown, version: 3 };
    const { calls } = await renderApp({
      path: '/sites/s1/pages/p1',
      handler: (url, init) => {
        if (url === '/api/pages/p1' && init.method === 'GET') return jsonResponse(200, PAGE);
        if (url === '/api/pages/p1' && init.method === 'PUT') {
          put.list = JSON.parse(String(init.body))['list'];
          put.version += 1;
          return jsonResponse(200, { version: put.version });
        }
        if (url === '/api/sites/s1/components') {
          return jsonResponse(200, [
            { type: 'Section', label: 'Секция', container: true, acceptsPageContent: true, schema: { type: 'object', properties: {} } },
          ]);
        }
        return jsonResponse(404, { error: 'not found' });
      },
    });

    await screen.findByText('· Привет');
    const layoutSelect = screen.getByLabelText('Каркас') as HTMLSelectElement;
    await waitFor(() =>
      expect(Array.from(layoutSelect.options).some((o) => o.value === 'Section')).toBe(true),
    );
    fireEvent.change(layoutSelect, { target: { value: 'Section' } });

    const titleInput = screen.getByLabelText('Title (SEO)');
    fireEvent.change(titleInput, { target: { value: 'Мой заголовок' } });

    await waitFor(
      () => {
        const save = calls.find((c) => c.method === 'PUT' && c.url === '/api/pages/p1');
        expect(save).toBeTruthy();
        const body = JSON.parse(String(save?.init.body)) as {
          list: ElementNode[];
          layoutSectionId: string;
          head: { title: string };
        };
        expect(body.layoutSectionId).toBe('Section');
        expect(body.head.title).toBe('Мой заголовок');
      },
      { timeout: 3000 },
    );
    void put;
  });

  it('после автосейва собирается dev build; таб Превью показывает iframe', async () => {
    const put = { list: null as unknown, version: 3 };
    const { calls } = await renderApp({
      path: '/sites/s1/pages/p1',
      handler: (url, init) => {
        if (url === '/api/pages/p1' && init.method === 'GET') return jsonResponse(200, PAGE);
        if (url === '/api/pages/p1' && init.method === 'PUT') {
          put.list = JSON.parse(String(init.body))['list'];
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
    await waitFor(() => expect(__lastRichTextEditor()).toBeTruthy());
    __lastRichTextEditor()?.chain().focus().selectAll().deleteSelection().insertContent('Заголовок').run();
    await waitFor(
      () => expect(calls.some((c) => c.method === 'PUT' && c.url === '/api/pages/p1')).toBe(true),
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