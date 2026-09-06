import { useState } from 'react';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import type { AssetMeta } from '../../runtime';
import { assetUrl } from './asset-utils';
import { AssetPicker } from './AssetPicker';

/** Trigger + превью выбранного ассета в инспекторе; открывает AssetPicker. */
export function AssetFieldControl({
  siteId,
  value,
  onChange,
}: {
  siteId: string;
  value: unknown;
  onChange: (id: string) => void;
}) {
  const { t } = useAdmin();
  const [open, setOpen] = useState(false);
  const assetId = typeof value === 'string' && value ? value : '';

  return (
    <div className="flex items-center gap-2">
      {assetId ? (
        <AssetValue assetId={assetId} />
      ) : (
        <span className="rounded bg-neutral-100 px-2 py-1 text-[10px] text-neutral-400">—</span>
      )}
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="rounded border border-neutral-300 px-2 py-1 text-xs text-neutral-700 hover:border-blue-400 hover:text-blue-600"
      >
        {t('assets.select')}
      </button>
      {assetId && (
        <button
          type="button"
          onClick={() => onChange('')}
          className="text-xs text-neutral-400 hover:text-red-600"
        >
          {t('assets.clear')}
        </button>
      )}
      <AssetPicker open={open} onClose={() => setOpen(false)} siteId={siteId} selectedId={assetId} onSelect={onChange} />
    </div>
  );
}

/** Миниатюра выбранного ассета: thumbnail + имя, подтягиваются по id. */
function AssetValue({ assetId }: { assetId: string }) {
  const { state } = useOperation<AssetMeta | null>(
    'getAsset',
    { assetId },
    (raw) => (raw && typeof raw === 'object' ? (raw as AssetMeta) : null),
  );
  if (state.status !== 'success') return <span className="text-xs text-neutral-400">{assetId}</span>;
  const asset = state.data;
  if (!asset) return <span className="text-xs text-neutral-400">{assetId}</span>;
  const image = asset.mime.startsWith('image/');
  return (
    <div className="flex min-w-0 items-center gap-2">
      {image ? (
        <img src={assetUrl(asset.id)} alt={asset.name} className="h-8 w-8 rounded object-cover" />
      ) : (
        <span className="flex h-8 w-8 items-center justify-center rounded bg-neutral-100 text-[9px] font-medium text-neutral-500">
          {asset.name.includes('.') ? asset.name.split('.').pop()!.toUpperCase().slice(0, 4) : 'FILE'}
        </span>
      )}
      <span className="truncate text-xs text-neutral-600">{asset.name}</span>
    </div>
  );
}