import { describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import type { AssetMeta } from '../../runtime';
import { jsonResponse, renderApp } from '../test-utils';

const IMAGE: AssetMeta = {
  id: 'a1',
  siteId: 's1',
  name: 'logo.png',
  mime: 'image/png',
  size: 1536,
  variants: [{ name: 'logo.png', url: '/assets/a1/file', mime: 'image/png', size: 1536 }],
};

const PDF: AssetMeta = {
  id: 'a2',
  siteId: 's1',
  name: 'spec.pdf',
  mime: 'application/pdf',
  size: 2048 * 2048,
  variants: [],
};

describe('SiteAssetsPage', () => {
  it('показывает ассеты списком: превью, имя, mime, размер', async () => {
    const { calls } = await renderApp({
      path: '/sites/s1/assets',
      handler: (url) =>
        url === '/api/sites/s1/assets' ? jsonResponse(200, [IMAGE, PDF]) : jsonResponse(200, []),
    });

    expect(await screen.findByText('logo.png')).toBeTruthy();
    expect(screen.getByText('spec.pdf')).toBeTruthy();
    expect(screen.getByText('image/png')).toBeTruthy();
    expect(screen.getByText('1.5 kB')).toBeTruthy();
    expect(screen.getByText('4.0 MB')).toBeTruthy();

    expect(calls.some((c) => c.url === '/api/sites/s1/assets' && c.method === 'GET')).toBe(true);
    const img = screen.getByAltText('logo.png') as HTMLImageElement;
    expect(img.src.endsWith('/api/assets/a1/file')).toBe(true);
  });

  it('загружает файл: POST multipart с полями file и name', async () => {
    let rows: AssetMeta[] = [];
    const { calls } = await renderApp({
      path: '/sites/s1/assets',
      handler: (url, init) => {
        if (url === '/api/sites/s1/assets' && init.method === 'POST') {
          const form = init.body as FormData;
          const file = form.get('file') as File;
          rows = [
            ...rows,
            {
              id: 'a9',
              siteId: 's1',
              name: String(form.get('name') ?? file.name),
              mime: file.type || 'application/octet-stream',
              size: file.size,
              variants: [],
            },
          ];
          return jsonResponse(201, rows[rows.length - 1]!);
        }
        return jsonResponse(200, rows);
      },
    });

    fireEvent.click(await screen.findByRole('button', { name: 'Загрузить' }));
    const input = screen.getByLabelText('Файл');
    const file = new File(['data'], 'logo.png', { type: 'image/png' });
    Object.defineProperty(input, 'files', { value: [file], configurable: true });
    fireEvent.change(input);
    fireEvent.change(screen.getByLabelText('Название'), { target: { value: 'Мой логотип' } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    expect(await screen.findByText('Мой логотип')).toBeTruthy();
    const post = calls.find((c) => c.method === 'POST' && c.url === '/api/sites/s1/assets');
    const form = post?.init.body as FormData;
    expect((form.get('file') as File).name).toBe('logo.png');
    expect(form.get('name')).toBe('Мой логотип');
    expect((post?.init.headers as Record<string, string> | undefined)?.['Content-Type']).toBeUndefined();
  });

  it('ошибка загрузки показывается в форме', async () => {
    await renderApp({
      path: '/sites/s1/assets',
      handler: (url, init) => {
        if (init.method === 'POST') return jsonResponse(413, { error: 'too large' });
        return jsonResponse(200, []);
      },
    });

    fireEvent.click(await screen.findByRole('button', { name: 'Загрузить' }));
    const input = screen.getByLabelText('Файл');
    const file = new File(['x'], 'big.bin', { type: 'application/octet-stream' });
    Object.defineProperty(input, 'files', { value: [file], configurable: true });
    fireEvent.change(input);
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    expect(await screen.findByText(/Ошибка загрузки/)).toBeTruthy();
  });

  it('удаляет ассет через подтверждение', async () => {
    let rows: AssetMeta[] = [IMAGE, PDF];
    const { calls } = await renderApp({
      path: '/sites/s1/assets',
      handler: (url, init) => {
        if (init.method === 'DELETE' && url === '/api/assets/a1') {
          rows = rows.filter((a) => a.id !== 'a1');
          return jsonResponse(204, undefined);
        }
        return jsonResponse(200, rows);
      },
    });

    expect(await screen.findByText('logo.png')).toBeTruthy();
    const deleteButtons = screen.getAllByRole('button', { name: 'Удалить ассет' });
    fireEvent.click(deleteButtons[0]!);
    fireEvent.click(screen.getByRole('button', { name: 'Подтвердить' }));

    await waitFor(() => expect(screen.queryByText('logo.png')).toBeNull());
    expect(calls.some((c) => c.method === 'DELETE' && c.url === '/api/assets/a1')).toBe(true);
  });
});