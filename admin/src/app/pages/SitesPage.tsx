import { useState } from 'react';
import { Link } from 'react-router-dom';
import { Inline, Stack } from '@liapoldus/ui-kit';
import { runOperation, type Site } from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { ConfirmButton } from '../components/ConfirmButton';
import { EntityTable } from '../components/EntityTable';
import { Field } from '../components/Field';

const INPUT_CLASS =
  'rounded border border-neutral-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none';

export function SitesPage() {
  const { api, t } = useAdmin();
  const list = useOperation<Site[]>('listSites', {}, (d: unknown) => (Array.isArray(d) ? d : []));

  const [formOpen, setFormOpen] = useState(false);
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [locale, setLocale] = useState('ru');
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
    const res = await runOperation(api, 'createSite', { name, slug, defaultLocale: locale }, t);
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

  const remove = async (site: Site) => {
    const res = await runOperation(api, 'deleteSite', { siteId: site.id }, t);
    if (res.ok) list.reload();
  };

  const sites: Site[] = list.state.status === 'success' ? list.state.data : [];

  return (
    <Stack pad={8} gap={4}>
      <Inline justify="between" align="center">
        <h1 className="text-xl font-medium">{t('site.title')}</h1>
        <button
          type="button"
          onClick={() => setFormOpen((v) => !v)}
          className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700"
        >
          {t('site.new')}
        </button>
      </Inline>

      {formOpen && (
        <form onSubmit={create} className="rounded border border-neutral-200 bg-neutral-50 p-4">
          <Stack gap={3}>
            <Inline gap={3} align="end">
              <Field label={t('site.name')} required>
                <input className={INPUT_CLASS} value={name} onChange={(e) => setName(e.target.value)} />
              </Field>
              <Field label={t('site.slug')} required>
                <input className={INPUT_CLASS} value={slug} onChange={(e) => setSlug(e.target.value)} />
              </Field>
              <Field label={t('site.locale')}>
                <select className={INPUT_CLASS} value={locale} onChange={(e) => setLocale(e.target.value)}>
                  <option value="ru">ru</option>
                  <option value="en">en</option>
                </select>
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
        (sites.length === 0 ? (
          <p className="text-sm text-neutral-400">{t('site.none')}</p>
        ) : (
          <EntityTable<Site>
            gridClass="grid-cols-[minmax(0,3fr)_minmax(0,2fr)_minmax(0,1fr)_minmax(0,2fr)_minmax(0,8rem)]"
            getKey={(s: Site) => s.id}
            columns={[
              { key: 'name', label: t('site.name'), render: (s: Site) => <span className="font-medium">{s.name}</span> },
              { key: 'slug', label: t('site.slug'), render: (s: Site) => <code className="text-xs">{s.slug}</code> },
              { key: 'locale', label: t('site.locale'), render: (s: Site) => s.defaultLocale },
              { key: 'hosts', label: t('site.hosts'), render: (s: Site) => s.hosts.join(', ') || '—' },
              {
                key: 'actions',
                label: t('common.actions'),
                className: 'text-right',
                render: (s: Site) => (
                  <Inline gap={2} justify="end">
                    <Link
                      to={`/sites/${s.id}`}
                      className="rounded border border-neutral-300 px-2 py-1 text-xs text-neutral-600 hover:border-blue-300 hover:text-blue-600"
                    >
                      {t('site.open')}
                    </Link>
                    <ConfirmButton label={t('common.delete')} onConfirm={() => remove(s)} />
                  </Inline>
                ),
              },
            ]}
            rows={sites}
          />
        ))}
    </Stack>
  );
}