import { useState, type FormEvent } from 'react';
import { runOperation, type AssetMeta } from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { AssetThumb } from './AssetThumb';
import { formatBytes } from './asset-utils';

const INPUT_CLASS =
  'w-full rounded border border-neutral-300 px-2 py-1 text-sm focus:border-blue-500 focus:outline-none';

/** Модальная библиотека ассетов: поиск, загрузка (multipart), выбор id. */
export function AssetPicker({
  open,
  onClose,
  siteId,
  selectedId,
  onSelect,
}: {
  open: boolean;
  onClose: () => void;
  siteId: string;
  selectedId?: string;
  onSelect: (assetId: string) => void;
}) {
  const { api, t } = useAdmin();
  const list = useOperation<AssetMeta[]>('listAssets', { siteId }, (raw) =>
    Array.isArray(raw) ? (raw as AssetMeta[]) : [],
  );
  const [query, setQuery] = useState('');
  const [name, setName] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  if (!open) return null;

  const q = query.trim().toLowerCase();
  const items = q
    ? (list.state.status === 'success' ? list.state.data : []).filter((a) =>
        a.name.toLowerCase().includes(q),
      )
    : list.state.status === 'success'
      ? list.state.data
      : [];

  const upload = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const formEl = e.currentTarget;
    const fileInput = (formEl.elements.namedItem('file') as HTMLInputElement | null)?.files?.[0];
    if (!fileInput) return;
    setBusy(true);
    setError('');
    const form = new FormData();
    form.append('file', fileInput);
    if (name.trim()) form.append('name', name.trim());
    const res = await runOperation(api, 'uploadAsset', { siteId, form }, t);
    setBusy(false);
    if (!res.ok) {
      setError(t('assets.uploadError', { detail: res.detail }));
      return;
    }
    setName('');
    formEl.reset();
    list.reload();
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
      onClick={onClose}
      role="presentation"
    >
      <div
        role="dialog"
        aria-label={t('assets.picker.title')}
        className="flex max-h-[70vh] w-full max-w-2xl flex-col rounded-lg bg-white p-4 shadow-xl"
        onClick={(e) => e.stopPropagation()}
      >
        <header className="mb-3 flex items-center gap-2">
          <h2 className="flex-1 text-sm font-semibold text-neutral-800">{t('assets.picker.title')}</h2>
          <button
            type="button"
            aria-label="✕"
            onClick={onClose}
            className="rounded px-2 py-0.5 text-neutral-400 hover:bg-neutral-100 hover:text-neutral-700"
          >
            ✕
          </button>
        </header>

        <div className="mb-3 grid grid-cols-1 gap-2 sm:grid-cols-[1fr_auto]">
          <input
            aria-label={t('assets.picker.search')}
            className={INPUT_CLASS}
            value={query}
            placeholder={t('assets.picker.search')}
            onChange={(e) => setQuery(e.target.value)}
          />
          <form onSubmit={upload} className="flex items-center gap-2">
            <input
              aria-label={t('assets.file')}
              type="file"
              name="file"
              className="max-w-[140px] text-xs text-neutral-600"
              onChange={() => setError('')}
            />
            <input
              aria-label={t('assets.name')}
              className={INPUT_CLASS}
              value={name}
              placeholder={t('assets.name')}
              onChange={(e) => setName(e.target.value)}
            />
            <button
              type="submit"
              disabled={busy}
              className="rounded bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-700 disabled:opacity-50"
            >
              {busy ? t('assets.uploading') : t('assets.upload')}
            </button>
          </form>
        </div>

        {error && <p className="mb-2 text-xs text-red-600">{error}</p>}

        {list.state.status === 'loading' && <p className="py-6 text-center text-xs text-neutral-400">…</p>}
        {list.state.status === 'error' && (
          <p className="py-6 text-center text-xs text-red-600">{list.state.detail}</p>
        )}
        {list.state.status === 'success' && items.length === 0 && (
          <p className="py-6 text-center text-xs text-neutral-400">{t('assets.none')}</p>
        )}
        {items.length > 0 && (
          <ul className="grid grid-cols-2 gap-2 overflow-y-auto sm:grid-cols-3 lg:grid-cols-4">
            {items.map((asset) => {
              const selected = asset.id === selectedId;
              return (
                <li key={asset.id}>
                  <button
                    type="button"
                    data-testid="asset-picker-item"
                    onClick={() => {
                      onSelect(asset.id);
                      onClose();
                    }}
                    className={`flex w-full flex-col items-start gap-1 rounded border p-2 text-left hover:border-blue-400 ${
                      selected ? 'border-blue-500 bg-blue-50' : 'border-neutral-200'
                    }`}
                  >
                    <AssetThumb asset={asset} className="h-16 w-full" />
                    <span className="w-full truncate text-xs font-medium text-neutral-700">{asset.name}</span>
                    <span className="text-[10px] text-neutral-400">{formatBytes(asset.size)}</span>
                  </button>
                </li>
              );
            })}
          </ul>
        )}
      </div>
    </div>
  );
}