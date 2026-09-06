import { useMemo, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { Inline, Stack } from '@liapoldus/ui-kit';
import { runOperation, type ContentSummary } from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { ConfirmButton } from '../components/ConfirmButton';
import { EntityTable } from '../components/EntityTable';
import { Field } from '../components/Field';
import { JsonFieldsEditor } from '../content/JsonFieldsEditor';

const INPUT_CLASS =
  'rounded border border-neutral-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none';

function previewFields(fields: Record<string, unknown>): string {
  const first = Object.entries(fields)[0];
  if (!first) return '—';
  const [key, value] = first;
  return `${key}: ${typeof value === 'object' && value !== null ? JSON.stringify(value) : String(value)}`;
}

export function SiteContentsPage() {
  const { siteId = '' } = useParams();
  const { api, t } = useAdmin();
  const list = useOperation<ContentSummary[]>(
    'listContents',
    { siteId },
    (d) => (Array.isArray(d) ? (d as ContentSummary[]) : []),
  );

  const [collectionFilter, setCollectionFilter] = useState<string | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [collectionId, setCollectionId] = useState('');
  const [itemId, setItemId] = useState('');
  const [fields, setFields] = useState<Record<string, unknown>>({});
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState('');

  const collections = useMemo(() => {
    const arr = list.state.status === 'success' ? list.state.data : [];
    const set = new Set(arr.map((c) => c.collectionId));
    if (collectionFilter && !set.has(collectionFilter)) set.add(collectionFilter);
    return [...set].sort();
  }, [list.state, collectionFilter]);

  const rows =
    list.state.status === 'success'
      ? (collectionFilter
          ? list.state.data.filter((c) => c.collectionId === collectionFilter)
          : list.state.data)
      : [];

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!collectionId.trim()) {
      setFormError(t('common.required'));
      return;
    }
    setSubmitting(true);
    setFormError('');
    const args: Record<string, unknown> = { siteId, collectionId: collectionId.trim(), fields };
    if (itemId.trim()) args.id = itemId.trim();
    const res = await runOperation(api, 'createContent', args, t);
    setSubmitting(false);
    if (!res.ok) {
      setFormError(t('content.createError', { detail: res.detail }));
      return;
    }
    setCollectionId('');
    setItemId('');
    setFields({});
    setFormOpen(false);
    list.reload();
  };

  const remove = async (item: ContentSummary) => {
    const res = await runOperation(api, 'deleteContent', { siteId, contentId: item.id }, t);
    if (res.ok) list.reload();
  };

  return (
    <Stack pad={8} gap={4}>
      <Inline justify="between" align="center">
        <h1 className="text-xl font-medium">{t('content.title')}</h1>
        <button
          type="button"
          onClick={() => setFormOpen((v) => !v)}
          className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700"
        >
          {t('content.new')}
        </button>
      </Inline>

      {formOpen && (
        <form onSubmit={create} className="rounded border border-neutral-200 bg-neutral-50 p-4">
          <Stack gap={3}>
            <Inline gap={3} align="end">
              <Field label={t('content.collection')} required>
                <input
                  className={INPUT_CLASS}
                  value={collectionId}
                  placeholder="strings"
                  onChange={(e) => setCollectionId(e.target.value)}
                />
              </Field>
              <Field label={t('content.id')}>
                <input
                  className={INPUT_CLASS}
                  value={itemId}
                  placeholder="nav.home"
                  onChange={(e) => setItemId(e.target.value)}
                />
              </Field>
            </Inline>
            <div className="flex flex-col gap-1 text-sm">
              <span className="text-neutral-500">{t('content.fields')}</span>
              <JsonFieldsEditor value={fields} onChange={setFields} t={t} />
            </div>
            <Inline gap={2}>
              <button
                type="submit"
                disabled={submitting}
                className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700 disabled:opacity-50"
              >
                {t('common.create')}
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

      {list.state.status === 'success' && collections.length > 0 && (
        <Inline gap={2} wrap>
          <button
            type="button"
            onClick={() => setCollectionFilter(null)}
            className={`rounded-full border px-3 py-1 text-xs ${
              collectionFilter === null ? 'border-blue-400 bg-blue-50 text-blue-700' : 'border-neutral-300 text-neutral-600'
            }`}
          >
            {t('content.collection.all')}
          </button>
          {collections.map((col) => (
            <button
              key={col}
              type="button"
              onClick={() => setCollectionFilter(col)}
              className={`rounded-full border px-3 py-1 text-xs ${
                collectionFilter === col ? 'border-blue-400 bg-blue-50 text-blue-700' : 'border-neutral-300 text-neutral-600'
              }`}
            >
              {col}
            </button>
          ))}
        </Inline>
      )}

      {list.state.status === 'success' &&
        (rows.length === 0 ? (
          <p className="text-sm text-neutral-400">{t('content.none')}</p>
        ) : (
          <EntityTable<ContentSummary>
            gridClass="grid-cols-[minmax(0,2fr)_minmax(0,2fr)_minmax(0,3fr)_minmax(0,8rem)]"
            getKey={(c) => c.id}
            columns={[
              {
                key: 'id',
                label: t('content.id'),
                render: (c) => <span className="font-mono text-xs font-medium">{c.id}</span>,
              },
              {
                key: 'collection',
                label: t('content.collection'),
                render: (c) => <code className="text-xs">{c.collectionId}</code>,
              },
              { key: 'fields', label: t('content.fields'), render: (c) => previewFields(c.fields) },
              {
                key: 'actions',
                label: t('common.actions'),
                className: 'text-right',
                render: (c) => (
                  <Inline gap={2} justify="end">
                    <Link
                      to={`/sites/${siteId}/contents/${c.id}`}
                      className="rounded border border-neutral-300 px-2 py-1 text-xs text-neutral-600 hover:border-blue-300 hover:text-blue-600"
                    >
                      {t('content.open')}
                    </Link>
                    <ConfirmButton label={t('content.delete.confirm')} onConfirm={() => remove(c)} />
                  </Inline>
                ),
              },
            ]}
            rows={rows}
          />
        ))}
    </Stack>
  );
}