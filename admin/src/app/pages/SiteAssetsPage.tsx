import { useState, type FormEvent } from 'react';
import { useParams } from 'react-router-dom';
import { Inline, Stack } from '@liapoldus/ui-kit';
import { runOperation, type AssetMeta } from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { ConfirmButton } from '../components/ConfirmButton';
import { EntityTable } from '../components/EntityTable';
import { Field } from '../components/Field';
import { AssetThumb } from '../assets/AssetThumb';
import { assetUrl, formatBytes } from '../assets/asset-utils';

const INPUT_CLASS =
  'rounded border border-neutral-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none';

export function SiteAssetsPage() {
  const { siteId = '' } = useParams();
  const { api, t } = useAdmin();
  const list = useOperation<AssetMeta[]>(
    'listAssets',
    { siteId },
    (d) => (Array.isArray(d) ? (d as AssetMeta[]) : []),
  );

  const [formOpen, setFormOpen] = useState(false);
  const [name, setName] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState('');

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
    list.reload();
  };

  const remove = async (asset: AssetMeta) => {
    const res = await runOperation(api, 'deleteAsset', { assetId: asset.id }, t);
    if (res.ok) list.reload();
  };

  return (
    <Stack pad={8} gap={4}>
      <Inline justify="between" align="center">
        <h1 className="text-xl font-medium">{t('assets.title')}</h1>
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

      {list.state.status === 'success' &&
        (list.state.data.length === 0 ? (
          <p className="text-sm text-neutral-400">{t('assets.none')}</p>
        ) : (
          <EntityTable<AssetMeta>
            gridClass="grid-cols-[3rem_minmax(0,2.5fr)_minmax(0,1fr)_minmax(0,1fr)_minmax(0,6rem)]"
            getKey={(a) => a.id}
            columns={[
              {
                key: 'thumb',
                label: ' ',
                render: (a) => <AssetThumb asset={a} />,
              },
              {
                key: 'name',
                label: t('assets.name'),
                render: (a) => <span className="truncate font-medium">{a.name}</span>,
              },
              { key: 'mime', label: t('assets.type'), render: (a) => <code className="text-xs">{a.mime}</code> },
              { key: 'size', label: t('assets.size'), render: (a) => formatBytes(a.size) },
              {
                key: 'actions',
                label: t('common.actions'),
                className: 'text-right',
                render: (a) => (
                  <Inline gap={2} justify="end">
                    <a
                      href={assetUrl(a.id)}
                      target="_blank"
                      rel="noreferrer"
                      className="rounded border border-neutral-300 px-2 py-1 text-xs text-neutral-600 hover:border-blue-300 hover:text-blue-600"
                    >
                      {t('assets.open')}
                    </a>
                    <ConfirmButton label={t('assets.delete.confirm')} onConfirm={() => remove(a)} />
                  </Inline>
                ),
              },
            ]}
            rows={list.state.data}
          />
        ))}
    </Stack>
  );
}