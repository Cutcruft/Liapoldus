import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { AdminProvider } from '../admin-context';
import { AssetPicker } from './AssetPicker';
import { createAdminApi, createTokenStore, makeTranslate, STRINGS } from '../../runtime';
import type { AssetMeta } from '../../runtime';

afterEach(cleanup);

const t = makeTranslate(STRINGS);

const ASSETS: AssetMeta[] = [
  {
    id: 'a1',
    siteId: 's1',
    name: 'logo.png',
    mime: 'image/png',
    size: 1024,
    variants: [],
  },
  {
    id: 'a2',
    siteId: 's1',
    name: 'doc.pdf',
    mime: 'application/pdf',
    size: 2048,
    variants: [],
  },
];

function renderPicker(options: {
  selectedId?: string;
  onSelect?: (id: string) => void;
  assets?: AssetMeta[];
  onUpload?: (init: RequestInit) => Response;
}) {
  const onSelect = options.onSelect ?? vi.fn();
  const items = [...(options.assets ?? ASSETS)];
  const api = createAdminApi({
    baseUrl: '',
    getToken: () => null,
    fetchFn: async (input: string | URL | RequestInfo, init?: RequestInit) => {
      const url = String(input);
      if (url === '/api/sites/s1/assets' && init?.method === 'POST') return options.onUpload?.(init ?? {}) ?? new Response(null, { status: 500 });
      if (url === '/api/sites/s1/assets') {
        return new Response(JSON.stringify(items), { status: 200, headers: { 'Content-Type': 'application/json' } });
      }
      return new Response(null, { status: 404 });
    },
  });
  render(
    <AdminProvider api={api} tokenStore={createTokenStore(null)}>
      <AssetPicker open siteId="s1" selectedId={options.selectedId} onSelect={onSelect} onClose={vi.fn()} />
    </AdminProvider>,
  );
  const { pushAsset } = { pushAsset: (a: AssetMeta) => items.push(a) };
  return { onSelect, pushAsset };
}

describe('AssetPicker', () => {
  it('список + поиск фильтрует по имени', async () => {
    renderPicker({});
    expect(await screen.findByText('logo.png')).toBeTruthy();

    fireEvent.change(screen.getByLabelText(t('assets.picker.search')), { target: { value: 'pdf' } });
    await waitFor(() => expect(screen.queryByText('logo.png')).toBeNull());
    expect(screen.getByText('doc.pdf')).toBeTruthy();
  });

  it('клик по карточке выбирает id и закрывается', async () => {
    const { onSelect } = renderPicker({ selectedId: 'a1' });
    fireEvent.click((await screen.findAllByTestId('asset-picker-item'))[0]!);
    expect(onSelect).toHaveBeenCalledWith('a1');
  });

  it('upload отправляет FormData и обновляет список', async () => {
    let uploaded: FormData | undefined;
    const { onSelect, pushAsset } = renderPicker({
      onUpload: (init) => {
        uploaded = init.body as FormData;
        const file = uploaded.get('file') as File;
        pushAsset({
          id: 'a9',
          siteId: 's1',
          name: String(uploaded.get('name') ?? file.name),
          mime: file.type,
          size: file.size,
          variants: [],
        });
        return new Response(JSON.stringify(uploaded), { status: 201, headers: { 'Content-Type': 'application/json' } });
      },
    });
    void onSelect;

    const input = screen.getByLabelText(t('assets.file'));
    const file = new File(['x'], 'new.png', { type: 'image/png' });
    Object.defineProperty(input, 'files', { value: [file], configurable: true });
    fireEvent.change(input);
    fireEvent.change(screen.getByLabelText(t('assets.name')), { target: { value: 'Новый' } });
    fireEvent.click(screen.getByRole('button', { name: t('assets.upload') }));

    expect(await screen.findByText('Новый')).toBeTruthy();
    expect(uploaded?.get('name')).toBe('Новый');
    expect((uploaded?.get('file') as File).name).toBe('new.png');
  });
});