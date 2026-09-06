import type { AssetMeta } from '../../runtime';
import { assetUrl, extensionOf, isImageMeta } from './asset-utils';

/** Превью ассета: картинка по байтам, иначе бейдж расширения. */
export function AssetThumb({ asset, className }: { asset?: AssetMeta | null; className?: string }) {
  const cls = `flex items-center justify-center overflow-hidden rounded bg-neutral-100 ${
    className ?? 'h-12 w-12'
  }`;
  if (asset && isImageMeta(asset)) {
    return (
      <img
        src={assetUrl(asset.id)}
        alt={asset.name}
        className={`${cls} object-cover`}
        data-testid="asset-thumb-img"
      />
    );
  }
  return (
    <div className={cls} data-testid="asset-thumb-badge" aria-label={asset?.mime ?? ''}>
      <span className="text-[10px] font-medium text-neutral-500">
        {asset ? extensionOf(asset.name) : '?'}
      </span>
    </div>
  );
}