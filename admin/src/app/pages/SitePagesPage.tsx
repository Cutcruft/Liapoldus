import { useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { Inline, Stack } from '@liapoldus/ui-kit';
import { runOperation, type Page } from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { ConfirmButton } from '../components/ConfirmButton';
import { EntityTable } from '../components/EntityTable';
import { Field } from '../components/Field';

const INPUT_CLASS =
  'rounded border border-neutral-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none';

export function SitePagesPage() {
  const { siteId = '' } = useParams();
  const { api, t } = useAdmin();
  const list = useOperation<Page[]>('listPages', { siteId }, (d: unknown) => (Array.isArray(d) ? d : []));

  const [formOpen, setFormOpen] = useState(false);
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState('');

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim() || !slug.trim()) {
      setFormError(t('common.required'));
      return;
    }
    setSubmitting(true);
    setFormError('');
    const res = await runOperation(api, 'createPage', { siteId, name, slug }, t);
    setSubmitting(false);
    if (!res.ok) {
      setFormError(res.detail);
      return;
    }
    setName('');
    setSlug('');
    setFormOpen(false);
    list.reload();
  };

  const remove = async (page: Page) => {
    const res = await runOperation(api, 'deletePage', { pageId: page.id }, t);
    if (res.ok) list.reload();
  };

  const pages: Page[] = list.state.status === 'success' ? list.state.data : [];

  return (
    <Stack pad={8} gap={4}>
      <Inline justify="between" align="center">
        <h1 className="text-xl font-medium">{t('page.title')}</h1>
        <button
          type="button"
          onClick={() => setFormOpen((v) => !v)}
          className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700"
        >
          {t('page.new')}
        </button>
      </Inline>

      {formOpen && (
        <form onSubmit={create} className="rounded border border-neutral-200 bg-neutral-50 p-4">
          <Stack gap={3}>
            <Inline gap={3} align="end">
              <Field label={t('page.name')} required>
                <input className={INPUT_CLASS} value={name} onChange={(e) => setName(e.target.value)} />
              </Field>
              <Field label={t('page.slug')} required>
                <input className={INPUT_CLASS} value={slug} onChange={(e) => setSlug(e.target.value)} />
              </Field>
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
        (pages.length === 0 ? (
          <p className="text-sm text-neutral-400">{t('page.none')}</p>
        ) : (
          <EntityTable<Page>
            gridClass="grid-cols-[minmax(0,3fr)_minmax(0,2fr)_minmax(0,5rem)_minmax(0,10rem)]"
            getKey={(p: Page) => p.id}
            columns={[
              { key: 'name', label: t('page.name'), render: (p: Page) => <span className="font-medium">{p.name}</span> },
              { key: 'slug', label: t('page.slug'), render: (p: Page) => <code className="text-xs">{p.slug}</code> },
              { key: 'version', label: t('page.version'), render: (p: Page) => `v${p.version}` },
              {
                key: 'actions',
                label: t('common.actions'),
                className: 'text-right',
                render: (p: Page) => (
                  <Inline gap={2} justify="end">
                    <Link
                      to={`/sites/${siteId}/pages/${p.id}`}
                      className="rounded border border-neutral-300 px-2 py-1 text-xs text-neutral-600 hover:border-blue-300 hover:text-blue-600"
                    >
                      {t('page.open')}
                    </Link>
                    <ConfirmButton label={t('common.delete')} onConfirm={() => remove(p)} />
                  </Inline>
                ),
              },
            ]}
            rows={pages}
          />
        ))}
    </Stack>
  );
}