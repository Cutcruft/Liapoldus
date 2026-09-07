import { useMemo, useState, type FormEvent } from 'react';
import { useParams } from 'react-router-dom';
import { Inline, Stack } from '@liapoldus/ui-kit';
import { runOperation, type AssetMeta, type AssetUsage } from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { ConfirmButton } from '../components/ConfirmButton';
import { Field } from '../components/Field';
import { AssetThumb } from '../assets/AssetThumb';
import { assetUrl, formatBytes } from '../assets/asset-utils';

const INPUT_CLASS =
  'rounded border border-neutral-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none';

/**
 * Раздел «Обслуживание → Медиа» (R4): грид ассетов с превью, поиск и фильтр
 * по типу, загрузка, контекст: скачать/копировать URL/«где используется»/удалить.
 * Панель usage — ответ нового `GET /sites/{siteId}/assets/{assetId}/usage`.
 */
export function MediaSection() {
  const { siteId = '' } = useParams();
  const { api, t } = useAdmin();
  const list = useOperation<AssetMeta[]>(
    'listAssets',
    { siteId },
    (d) => (Array.isArray(d) ? (d as AssetMeta[]) : []),
  );

  const [query, setQuery] = useState('');
  const [typeFilter, setTypeFilter] = useState<string>('all');
  const [formOpen, setFormOpen] = useState(false);
  const [name, setName] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState('');
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const [usage, setUsage] = useState<{ asset: AssetMeta; data: AssetUsage | null } | null>(null);
  const [usageLoading, setUsageLoading] = useState(false);

  const assets: AssetMeta[] = list.state.status === 'success' ? list.state.data : [];

  const types = useMemo(() => {
    const set = new Set(assets.map((a) => a.mime.split('/')[0] || 'file'));
    if (typeFilter !== 'all' && !set.has(typeFilter)) set.add(typeFilter);
    return [...set].sort();
  }, [assets, typeFilter]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return assets.filter((a) => {
      if (typeFilter !== 'all' && (a.mime.split('/')[0] || 'file') !== typeFilter) return false;
      if (q && !a.name.toLowerCase().includes(q)) return false;
      return true;
    });
  }, [assets, query, typeFilter]);

  const upload = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const formEl = e.currentTarget;
    const fileInput = (formEl.elements.namedItem('file') as HTMLInputElement | null)?.files?.[0];
    if (!fileInput) {
      setFormError(t('common.required'));
      return;
    }
    setSubmitting(true);
    setFormError('');
    const form = new FormData();
    form.append('file', fileInput);
    if (name.trim()) form.append('name', name.trim());
    const res = await runOperation(api, 'uploadAsset', { siteId, form }, t);
    setSubmitting(false);
    if (!res.ok) {
      setFormError(t('assets.uploadError', { detail: res.detail }));
      return;
    }
    setName('');
    formEl.reset();
    setFormOpen(false);
    setUsage(null);
    list.reload();
  };

  const remove = async (asset: AssetMeta) => {
    const res = await runOperation(api, 'deleteAsset', { assetId: asset.id }, t);
    if (res.ok) {
      if (usage?.asset.id === asset.id) setUsage(null);
      list.reload();
    }
  };

  const copyUrl = async (asset: AssetMeta) => {
    const url = assetUrl(asset.id);
    try {
      await navigator.clipboard?.writeText(url);
      setCopiedId(asset.id);
    } catch {
      // буфер недоступен (тесты/https-ограничения) — просто пропускаем feedback
    }
  };

  const openUsage = async (asset: AssetMeta) => {
    setUsageLoading(true);
    setUsage({ asset, data: null });
    const res = await runOperation(api, 'getAssetUsage', { siteId, assetId: asset.id }, t);
    setUsageLoading(false);
    setUsage({
      asset,
      data:
        res.ok && typeof res.data === 'object' && res.data !== null ? (res.data as AssetUsage) : null,
    });
  };

  const usageTitle = usage ? t('media.usage') + ': ' + usage.asset.name : '';

  return (
    <Stack gap={4}>
      <Inline justify="between" align="center">
        <h1 className="text-xl font-medium">{t('media.title')}</h1>
        <button
          type="button"
          onClick={() => setFormOpen((v) => !v)}
          className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700"
        >
          {t('assets.upload')}
        </button>
      </Inline>

      {formOpen && (
        <form onSubmit={upload} className="rounded border border-neutral-200 bg-neutral-50 p-4">
          <Stack gap={3}>
            <Inline gap={3} align="end">
              <Field label={t('assets.file')} required>
                <input
                  type="file"
                  name="file"
                  aria-label={t('assets.file')}
                  className="text-sm text-neutral-700"
                  onChange={() => setFormError('')}
                />
              </Field>
              <Field label={t('assets.name')}>
                <input
                  className={INPUT_CLASS}
                  value={name}
                  placeholder="logo.png"
                  onChange={(e) => setName(e.target.value)}
                />
              </Field>
            </Inline>
            <Inline gap={2}>
              <button
                type="submit"
                disabled={submitting}
                className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700 disabled:opacity-50"
              >
                {submitting ? t('assets.uploading') : t('common.create')}
              </button>
              <button
                type="button"
                onClick={() => setFormOpen(false)}
                className="rounded border border-neutral-300 px-3 py-1.5 text-sm text-neutral-600"
              >
                {t('common.cancel')}
              </button>
            </Inline>
            {formError && <p className="text-sm text-red-600">{formError}</p>}
          </Stack>
        </form>
      )}

      {list.state.status === 'loading' && <p className="text-sm text-neutral-400">{t('common.loading')}</p>}

      {list.state.status === 'error' && (
        <Stack gap={2}>
          <p className="text-sm text-red-600">
            {t('common.error')}: {list.state.detail}
          </p>
          <button type="button" onClick={list.reload} className="w-fit rounded border border-neutral-300 px-3 py-1 text-sm">
            {t('common.reload')}
          </button>
        </Stack>
      )}

      {list.state.status === 'success' && assets.length > 0 && (
        <Inline gap={3} wrap>
          <label className="flex items-center gap-2 text-sm text-neutral-600">
            {t('media.search')}
            <input
              className={INPUT_CLASS}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="logo.png"
            />
          </label>
          <label className="flex items-center gap-2 text-sm text-neutral-600">
            {t('media.filter')}
            <select
              className={INPUT_CLASS}
              value={typeFilter}
              onChange={(e) => setTypeFilter(e.target.value || 'all')}
            >
              <option value="all">{t('media.filter.all')}</option>
              {types.map((type) => (
                <option key={type} value={type}>
                  {type}
                </option>
              ))}
            </select>
          </label>
        </Inline>
      )}

      {usage && (
        <div className="rounded border border-neutral-200 bg-neutral-50 p-3">
          <Inline justify="between" align="center">
            <h2 className="text-sm font-medium text-neutral-800">{usageTitle}</h2>
            <button
              type="button"
              onClick={() => setUsage(null)}
              className="rounded border border-neutral-300 px-2 py-1 text-xs text-neutral-600"
            >
              {t('media.usage.close')}
            </button>
          </Inline>
          {usageLoading ? (
            <p className="mt-2 text-sm text-neutral-400">{t('common.loading')}</p>
          ) : usage.data ? (
            usage.data.contents.length === 0 && usage.data.forms.length === 0 ? (
              <p className="mt-2 text-sm text-neutral-400">{t('media.usage.none')}</p>
            ) : (
              <Stack gap={2}>
                {usage.data.contents.length > 0 && (
                  <div>
                    <p className="mt-2 text-xs font-medium text-neutral-500">{t('media.usage.contents')}</p>
                    <ul className="mt-1 space-y-1 text-sm">
                      {usage.data.contents.map((c) => (
                        <li key={c.id}>
                          <code className="text-xs text-neutral-700">{c.id}</code>
                          <span className="ml-2 text-xs text-neutral-400">{c.collectionId}</span>
                        </li>
                      ))}
                    </ul>
                  </div>
                )}
                {usage.data.forms.length > 0 && (
                  <div>
                    <p className="mt-2 text-xs font-medium text-neutral-500">{t('media.usage.forms')}</p>
                    <ul className="mt-1 space-y-1 text-sm">
                      {usage.data.forms.map((f) => (
                        <li key={f.id}>
                          <span className="text-xs text-neutral-700">{f.name}</span>
                          <code className="ml-2 text-xs text-neutral-400">{f.id}</code>
                        </li>
                      ))}
                    </ul>
                  </div>
                )}
              </Stack>
            )
          ) : (
            <p className="mt-2 text-sm text-red-600">{t('common.error')}</p>
          )}
        </div>
      )}

      {list.state.status === 'success' &&
        (assets.length === 0 ? (
          <p className="text-sm text-neutral-400">{t('media.none')}</p>
        ) : filtered.length === 0 ? (
          <p className="text-sm text-neutral-400">{t('common.empty')}</p>
        ) : (
          <div className="grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-4">
            {filtered.map((a) => (
              <div
                key={a.id}
                className="flex flex-col gap-2 rounded border border-neutral-200 p-2"
                data-testid="media-card"
              >
                <div className="flex h-24 items-center justify-center overflow-hidden rounded bg-neutral-100">
                  <AssetThumb asset={a} className="h-full w-full" />
                </div>
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium text-neutral-900" title={a.name}>
                    {a.name}
                  </p>
                  <p className="truncate text-xs text-neutral-400">
                    <code>{a.mime}</code> · {formatBytes(a.size)}
                  </p>
                </div>
                <Inline gap={1} wrap>
                  <a
                    href={assetUrl(a.id)}
                    target="_blank"
                    rel="noreferrer"
                    className="rounded border border-neutral-300 px-2 py-1 text-xs text-neutral-600 hover:border-blue-300 hover:text-blue-600"
                  >
                    {t('assets.open')}
                  </a>
                  <button
                    type="button"
                    onClick={() => void copyUrl(a)}
                    className="rounded border border-neutral-300 px-2 py-1 text-xs text-neutral-600 hover:border-blue-300 hover:text-blue-600"
                  >
                    {copiedId === a.id ? t('media.copied') : t('media.copyUrl')}
                  </button>
                  <button
                    type="button"
                    onClick={() => void openUsage(a)}
                    aria-pressed={usage?.asset.id === a.id}
                    className="rounded border border-neutral-300 px-2 py-1 text-xs text-neutral-600 hover:border-blue-300 hover:text-blue-600"
                  >
                    {t('media.usage')}
                  </button>
                  <ConfirmButton label={t('assets.delete.confirm')} onConfirm={() => remove(a)} />
                </Inline>
              </div>
            ))}
          </div>
        ))}
    </Stack>
  );
}