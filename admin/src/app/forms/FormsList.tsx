import { useState } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import { Inline, Stack } from '@liapoldus/ui-kit';
import { runOperation, type AdminForm } from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { ConfirmButton } from '../components/ConfirmButton';
import { EntityTable } from '../components/EntityTable';
import { Field } from '../components/Field';
import { createDefaultDefinition, slugifyFormId } from './form-utils';

const INPUT_CLASS =
  'rounded border border-neutral-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none';

/**
 * Список форм сайта (R4): create/delete по определению (description в разделе
 * «Инфраструктура»), открытие переводит URL в `?mode=forms&formId=...`
 * (таблица сабмитов). Перенос из legacy SiteFormsPage.
 */
export function FormsList() {
  const { siteId = '' } = useParams();
  const [, setSearchParams] = useSearchParams();
  const { api, t } = useAdmin();
  const list = useOperation<AdminForm[]>('listForms', { siteId }, (d) => (Array.isArray(d) ? d : []));

  const [formOpen, setFormOpen] = useState(false);
  const [name, setName] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState('');

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    const clean = name.trim();
    if (!clean) {
      setFormError(t('common.required'));
      return;
    }
    setSubmitting(true);
    setFormError('');
    const res = await runOperation(
      api,
      'createForm',
      { siteId, name: clean, definition: createDefaultDefinition(slugifyFormId(clean)) },
      t,
    );
    setSubmitting(false);
    if (!res.ok) {
      setFormError(res.detail);
      return;
    }
    setName('');
    setFormOpen(false);
    list.reload();
  };

  const remove = async (form: AdminForm) => {
    const res = await runOperation(api, 'deleteForm', { siteId, formId: form.id }, t);
    if (res.ok) list.reload();
  };

  const open = (form: AdminForm) => {
    setSearchParams({ mode: 'forms', formId: form.id });
  };

  const forms: AdminForm[] = list.state.status === 'success' ? list.state.data : [];

  return (
    <Stack gap={4}>
      <Inline justify="between" align="center">
        <h1 className="text-xl font-medium">{t('form.title')}</h1>
        <button
          type="button"
          onClick={() => setFormOpen((v) => !v)}
          className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700"
        >
          {t('form.new')}
        </button>
      </Inline>

      {formOpen && (
        <form onSubmit={create} className="rounded border border-neutral-200 bg-neutral-50 p-4">
          <Inline gap={3} align="end">
            <Field label={t('form.name')} required>
              <input
                className={INPUT_CLASS}
                value={name}
                placeholder="Contact"
                onChange={(e) => setName(e.target.value)}
              />
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
          <p className="mt-1 text-xs text-neutral-400">{t('form.formIdHint')}</p>
          {formError && <p className="mt-2 text-sm text-red-600">{formError}</p>}
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
        (forms.length === 0 ? (
          <p className="text-sm text-neutral-400">{t('form.none')}</p>
        ) : (
          <EntityTable<AdminForm>
            gridClass="grid-cols-[minmax(0,3fr)_minmax(0,10rem)_minmax(0,5rem)_minmax(0,9rem)]"
            getKey={(f) => f.id}
            columns={[
              {
                key: 'name',
                label: t('form.name'),
                render: (f) => (
                  <button type="button" onClick={() => open(f)} className="truncate text-left font-medium text-neutral-900 hover:text-blue-600">
                    {f.name}
                  </button>
                ),
              },
              { key: 'id', label: t('form.formId'), render: (f) => <code className="text-xs">{f.id}</code> },
              {
                key: 'fields',
                label: t('form.fields'),
                render: (f) => String(f.definition?.fields?.length ?? 0),
              },
              {
                key: 'actions',
                label: t('common.actions'),
                className: 'text-right',
                render: (f) => (
                  <Inline gap={2} justify="end">
                    <button
                      type="button"
                      onClick={() => open(f)}
                      className="rounded border border-neutral-300 px-2 py-1 text-xs text-neutral-600 hover:border-blue-300 hover:text-blue-600"
                    >
                      {t('form.open')}
                    </button>
                    <ConfirmButton label={t('form.delete.confirm')} onConfirm={() => remove(f)} />
                  </Inline>
                ),
              },
            ]}
            rows={forms}
          />
        ))}
    </Stack>
  );
}