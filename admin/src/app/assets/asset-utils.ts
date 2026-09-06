import type { AssetMeta } from '../../runtime';

/** Адрес байтов ассета через admin-прокси `/api`. */
export function assetUrl(assetId: string): string {
  return `/api/assets/${encodeURIComponent(assetId)}/file`;
}

export function isImageMeta(asset: AssetMeta | undefined | null): boolean {
  return Boolean(asset?.mime.startsWith('image/'));
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} kB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

/** Расширение из имени файла (без точки), для бейджа не-картинок. */
export function extensionOf(name: string): string {
  const dot = name.lastIndexOf('.');
  return dot >= 0 ? name.slice(dot + 1).toUpperCase() : 'FILE';
}