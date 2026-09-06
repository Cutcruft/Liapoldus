import { describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import type { TokenSet } from '../../runtime';
import { jsonResponse, renderApp } from '../test-utils';

const INITIAL: TokenSet = {
  colors: [
    { name: 'accent', value: { light: '#0b2e4f', dark: '#7fd0ff' } },
    { name: 'danger', value: { light: '#cc0000', dark: '' } },
  ],
  typography: { 'text-xl': '2rem' },
  spacing: { pad: '1rem' },
};

describe('SiteTokensPage', () => {
  it('грузит и показывает цвета + скалярные группы + превью-swatches', async () => {
    renderApp({
      path: '/sites/s1/tokens',
      handler: (url) => (url === '/api/sites/s1/tokens' ? jsonResponse(200, INITIAL) : jsonResponse(200, [])),
    });

    expect(await screen.findByDisplayValue('accent')).toBeTruthy();
    expect(screen.getByDisplayValue('#0b2e4f')).toBeTruthy();
    expect(screen.getByDisplayValue('#7fd0ff')).toBeTruthy();
    expect(screen.getByDisplayValue('2rem')).toBeTruthy();
    expect(screen.getByDisplayValue('text-xl')).toBeTruthy();
    expect(screen.getByDisplayValue('1rem')).toBeTruthy();
    expect(screen.getByDisplayValue('pad')).toBeTruthy();
    const iframe = document.querySelector('iframe') as HTMLIFrameElement;
    expect(iframe).toBeTruthy();
    expect(iframe.srcdoc).toContain('--lia-accent-light: #0b2e4f');
  });

  it('добавляет цвет и автосейвит весь набор (PUT /tokens) после дебаунса', async () => {
    let saved: TokenSet | null = null;
    await renderApp({
      path: '/sites/s1/tokens',
      handler: (url, init) => {
        if (url === '/api/sites/s1/tokens' && init.method === 'PUT') {
          saved = JSON.parse(String(init.body)) as TokenSet;
          return jsonResponse(200, saved);
        }
        return jsonResponse(200, INITIAL);
      },
    });

    await screen.findByDisplayValue('accent');
    fireEvent.click(screen.getByRole('button', { name: 'Добавить цвет' }));
    await waitFor(() => expect(screen.getAllByLabelText('Light')).toHaveLength(3));
    fireEvent.change(screen.getAllByLabelText('Light')[2]!, { target: { value: '#123456' } });
    fireEvent.change(screen.getAllByLabelText('Роль')[2]!, { target: { value: 'primary' } });

    await waitFor(
      () => {
        expect(saved?.colors.length).toBe(3);
      },
      { timeout: 1500 },
    );
    expect(saved!.colors[2]!.value.light).toBe('#123456');
  });

  it('удаляет цвет: не уходит в сохранённый набор', async () => {
    let saved: TokenSet | null = null;
    await renderApp({
      path: '/sites/s1/tokens',
      handler: (url, init) => {
        if (url === '/api/sites/s1/tokens' && init.method === 'PUT') {
          saved = JSON.parse(String(init.body)) as TokenSet;
          return jsonResponse(200, saved);
        }
        return jsonResponse(200, INITIAL);
      },
    });

    await screen.findByDisplayValue('accent');
    fireEvent.click(screen.getAllByRole('button', { name: 'Удалить' })[0]!);

    await waitFor(
      () => {
        expect(saved?.colors.length).toBe(1);
      },
      { timeout: 1500 },
    );
    expect(saved!.colors[0]!.name).toBe('danger');
  });

  it('добавляет значение в скалярную группу', async () => {
    let saved: TokenSet | null = null;
    await renderApp({
      path: '/sites/s1/tokens',
      handler: (url, init) => {
        if (url === '/api/sites/s1/tokens' && init.method === 'PUT') {
          saved = JSON.parse(String(init.body)) as TokenSet;
          return jsonResponse(200, saved);
        }
        return jsonResponse(200, INITIAL);
      },
    });

    await screen.findByDisplayValue('accent');
    fireEvent.change(screen.getByLabelText('Добавить в группу…'), { target: { value: 'shadows' } });

    const keyInputs = screen.getAllByLabelText('Ключ');
    fireEvent.change(keyInputs[keyInputs.length - 1]!, { target: { value: 'sm' } });
    const valueInputs = screen.getAllByLabelText('Значение');
    fireEvent.change(valueInputs[valueInputs.length - 1]!, { target: { value: '0 2px 4px rgba(0,0,0,.1)' } });

    await waitFor(
      () => {
        expect(saved?.shadows?.sm).toBe('0 2px 4px rgba(0,0,0,.1)');
      },
      { timeout: 1500 },
    );
  });
});
