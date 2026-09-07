import { afterEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import type { AssetMeta, TokenSet } from '../../runtime';
import { jsonResponse, renderApp } from '../test-utils';
import { resetSiteTabStore } from '../site-tab-store';

const SITE = { id: 's1', name: 'Alpha', slug: 'alpha', defaultLocale: 'ru', hosts: [] as string[] };
const SETTINGS = {
  siteId: 's1',
  defaultLocale: 'ru',
  head: { titleTemplate: '', description: '', faviconAssetId: '', meta: {} },
};
const ASSETS: AssetMeta[] = [
  { id: 'asset:inter', siteId: 's1', name: 'Inter.ttf', mime: 'font/ttf', size: 128000, variants: [] },
];

const INITIAL: TokenSet = {
  colors: [
    { name: 'accent', value: { light: '#0b2e4f', dark: '#7fd0ff' } },
    { name: 'danger', value: { light: '#cc0000', dark: '' } },
  ],
  typography: { 'text-xl': '2rem' },
  spacing: { pad: '1rem' },
};

interface SavedBody {
  tokens: TokenSet | null;
  settings: unknown | null;
}

async function renderBase(onRequest: (url: string, init: RequestInit) => Response | Promise<Response>) {
  const utils = await renderApp({
    path: '/sites/s1?view=editor&section=base',
    handler: (url, init) => {
      if (url === '/api/sites/s1') return jsonResponse(200, SITE);
      if (url === '/api/sites/s1/settings') return jsonResponse(200, SETTINGS);
      if (url === '/api/sites/s1/components') return jsonResponse(200, []);
      if (url === '/api/sites/s1/tokens') return onRequest(url, init);
      if (url === '/api/sites/s1/assets') return jsonResponse(200, ASSETS);
      return jsonResponse(404, { detail: 'not found' });
    },
  });
  return utils;
}

function tokenHandler(saved: SavedBody) {
  return (url: string, init: RequestInit) => {
    if ((init.method ?? 'GET') === 'PUT') {
      saved.tokens = JSON.parse(String(init.body)) as TokenSet;
      return jsonResponse(200, saved.tokens);
    }
    return jsonResponse(200, INITIAL);
  };
}

async function openTab(name: string) {
  fireEvent.click(await screen.findByRole('tab', { name }));
}

describe('BaseSection — токены/шрифты', () => {
  it('грузит черновик и показывает цвета + скалярные группы + превью-swatches', async () => {
    const saved: SavedBody = { tokens: null, settings: null };
    await renderBase(tokenHandler(saved));

    await openTab('Токены');
    expect(await screen.findByDisplayValue('accent')).toBeTruthy();
    expect(screen.getByDisplayValue('#0b2e4f')).toBeTruthy();
    expect(screen.getByDisplayValue('#7fd0ff')).toBeTruthy();
    expect(screen.getByDisplayValue('2rem')).toBeTruthy();
    expect(screen.getByDisplayValue('text-xl')).toBeTruthy();
    expect(screen.getByDisplayValue('1rem')).toBeTruthy();
    expect(screen.getByDisplayValue('pad')).toBeTruthy();
    // названия групп рендерятся переведёнными, а не сырыми ключами
    expect(screen.getAllByText('Типографика').length).toBeGreaterThan(0);
    const iframe = document.querySelector('iframe') as HTMLIFrameElement;
    expect(iframe).toBeTruthy();
    expect(iframe.srcdoc).toContain('--lia-accent-light: #0b2e4f');
  });

  it('не пишет PUT на mount без изменений (single source: табы Настройки/Токены не клобберят)', async () => {
    const saved: SavedBody = { tokens: null, settings: null };
    await renderBase(tokenHandler(saved));
    await openTab('Токены');
    await screen.findByDisplayValue('accent');
    // ждём дольше дебаунса и убеждаемся, что автсохранение без правок не трогает бекенд
    await new Promise((r) => setTimeout(r, 700));
    expect(saved.tokens).toBeNull();
  });

  it('добавляет цвет и автосейвит весь набор (PUT /tokens) после дебаунса', async () => {
    const saved: SavedBody = { tokens: null, settings: null };
    await renderBase(tokenHandler(saved));

    await openTab('Токены');
    await screen.findByDisplayValue('accent');
    fireEvent.click(screen.getByRole('button', { name: 'Добавить цвет' }));
    await waitFor(() => expect(screen.getAllByLabelText('Light')).toHaveLength(3));
    fireEvent.change(screen.getAllByLabelText('Light')[2]!, { target: { value: '#123456' } });
    fireEvent.change(screen.getAllByLabelText('Роль')[2]!, { target: { value: 'primary' } });

    await waitFor(
      () => {
        expect(saved.tokens?.colors.length).toBe(3);
      },
      { timeout: 1500 },
    );
    expect(saved.tokens!.colors[2]!.value.light).toBe('#123456');
  });

  it('удаляет цвет: не уходит в сохранённый набор', async () => {
    const saved: SavedBody = { tokens: null, settings: null };
    await renderBase(tokenHandler(saved));

    await openTab('Токены');
    await screen.findByDisplayValue('accent');
    fireEvent.click(screen.getAllByRole('button', { name: 'Удалить' })[0]!);

    await waitFor(
      () => {
        expect(saved.tokens?.colors.length).toBe(1);
      },
      { timeout: 1500 },
    );
    expect(saved.tokens!.colors[0]!.name).toBe('danger');
  });

  it('добавляет значение в скалярную группу', async () => {
    const saved: SavedBody = { tokens: null, settings: null };
    await renderBase(tokenHandler(saved));

    await openTab('Токены');
    await screen.findByDisplayValue('accent');
    fireEvent.change(screen.getByLabelText('Добавить в группу…'), { target: { value: 'shadows' } });

    const keyInputs = screen.getAllByLabelText('Ключ');
    fireEvent.change(keyInputs[keyInputs.length - 1]!, { target: { value: 'sm' } });
    const valueInputs = screen.getAllByLabelText('Значение');
    fireEvent.change(valueInputs[valueInputs.length - 1]!, { target: { value: '0 2px 4px rgba(0,0,0,.1)' } });

    await waitFor(
      () => {
        expect(saved.tokens?.shadows?.sm).toBe('0 2px 4px rgba(0,0,0,.1)');
      },
      { timeout: 1500 },
    );
  });

  it('добавляет шрифт, привязывает ассет и шлёт его в том же TokenSet', async () => {
    const saved: SavedBody = { tokens: null, settings: null };
    await renderBase(tokenHandler(saved));

    await openTab('Шрифты');
    fireEvent.click(await screen.findByRole('button', { name: 'Добавить шрифт' }));
    const family = await screen.findByPlaceholderText('Семейство');
    fireEvent.change(family, { target: { value: 'Inter' } });
    fireEvent.change(screen.getByPlaceholderText('Начертание'), { target: { value: '400' } });
    fireEvent.change(screen.getByLabelText('Стиль'), { target: { value: 'italic' } });

    fireEvent.click(screen.getByRole('button', { name: 'Выбрать' }));
    const item = await screen.findByTestId('asset-picker-item');
    fireEvent.click(item);
    expect(screen.queryByLabelText('Файл')).toBeTruthy();

    await waitFor(
      () => {
        expect(saved.tokens?.fonts).toHaveLength(1);
      },
      { timeout: 1500 },
    );
    expect(saved.tokens!.fonts![0]).toEqual({
      family: 'Inter',
      assetId: 'asset:inter',
      weight: '400',
      style: 'italic',
    });
  });
});

afterEach(() => {
  resetSiteTabStore();
});