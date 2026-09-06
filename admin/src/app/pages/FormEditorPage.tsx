import { useEffect, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { Inline, Stack } from '@liapoldus/ui-kit';
import { runOperation, type AdminForm, type Submission } from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { Field } from '../components/Field';
import { ConfirmButton } from '../components/ConfirmButton';
import { EntityTable } from '../components/EntityTable';
import { validateDefinition } from '../forms/form-utils';

const INPUT_CLASS =
  'rounded border border-neutral-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none';

const TAB_CLASS = (active: boolean) =>
  `rounded-t-md border border-b-0 px-3 py-1 text-xs ${
    active
      ? 'border-blue-400 bg-blue-50 text-blue-700'
      : 'border-neutral-300 text-neutral-500 hover:text-neutral-800'
  }`;

function SubmissionsList({ siteId, formId }: { siteId: string; formId: string }) {
  const { t } = useAdmin();
  const list = useOperation<Submission[]>(
    'listSubmissions',
    { siteId, formId },
    (d) => (Array.isArray(d) ? d : []),
  );

  if (list.state.status === 'loading') return <p className="text-sm text-neutral-400">{t('common.loading')}</p>;
  if (list.state.status === 'error') {
    return (
      <Stack gap={2}>
        <p className="text-sm text-red-600">
          {t('common.error')}: {list.state.detail}
        </p>
        <button type="button" onClick={list.reload} className="w-fit rounded border border-neutral-300 px-3 py-1 text-sm">
          {t('common.reload')}
        </button>
      </Stack>
    );
  }

  const submissions = list.state.data;
  if (submissions.length === 0) return <p className="text-sm text-neutral-400">{t('form.submissions.none')}</p>;

  return (
    <EntityTable<Submission>
      gridClass="grid-cols-[minmax(0,8rem)_minmax(0,12rem)_minmax(0,1fr)]"
      getKey={(s) => s.id}
      columns={[
        { key: 'id', label: t('form.submissions.id'), render: (s) => <code className="text-xs">{s.id}</code> },
        { key: 'date', label: t('form.submissions.date'), render: (s) => s.createdAt },
        {
          key: 'payload',
          label: t('form.submissions.payload'),
          render: (s) => (
            <details>
              <summary className="cursor-pointer text-xs text-neutral-500">JSON</summary>
              <pre className="mt-1 max-h-40 overflow-auto rounded bg-neutral-50 p-2 text-xs">
                {JSON.stringify(s.payload, null, 2)}
              </pre>
            </details>
          ),
        },
      ]}
      rows={submissions}
    />
  );
}

export function FormEditorPage() {
  const { siteId = '', formId = '' } = useParams();
  const navigate = useNavigate();
  const { api, t } = useAdmin();
  const form = useOperation<AdminForm | null>(
    'getForm',
    { siteId, formId },
    (d) => (typeof d === 'object' && d !== null && 'id' in d ? (d as AdminForm) : null),
  );

  const [name, setName] = useState('');
  const [definitionText, setDefinitionText] = useState('');
  const [tab, setTab] = useState<'definition' | 'submissions'>('definition');
  const [saved, setSaved] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (form.state.status !== 'success' || !form.state.data) return;
    const f = form.state.data;
    setName(f.name ?? '');
    setDefinitionText(JSON.stringify(f.definition ?? {}, null, 2));
    setSaved('');
  }, [form.state]);

  const save = async () => {
    setError('');
    const check = validateDefinition(definitionText);
    if (!check.ok) {
      setError(`${t('form.invalid')}: ${check.error}`);
      return;
    }
    setBusy(true);
    const res = await runOperation(
      api,
      'updateForm',
      { siteId, formId, patch: { name: name.trim(), definition: check.value } },
      t,
    );
    setBusy(false);
    if (res.ok) setSaved(t('form.saved'));
    else setError(res.detail);
  };

  const remove = async () => {
    const res = await runOperation(api, 'deleteForm', { siteId, formId }, t);
    if (res.ok) navigate(`/sites/${siteId}/forms`);
  };

  if (form.state.status === 'loading') {
    return <p className="p-8 text-sm text-neutral-400">{t('common.loading')}</p>;
  }

  if (form.state.status === 'error') {
    return (
      <Stack pad={8} gap={2}>
        <p className="text-sm text-red-600">
          {t('common.error')}: {form.state.detail}
        </p>
        <button type="button" onClick={form.reload} className="w-fit rounded border border-neutral-300 px-3 py-1 text-sm">
          {t('common.reload')}
        </button>
      </Stack>
    );
  }

  const item = form.state.data!;

  return (
    <Stack pad={8} gap={4}>
      <Inline justify="between" align="center">
        <div>
          <h1 className="text-xl font-medium">{t('form.title')}</h1>
          <p className="text-sm text-neutral-500">
            <Link to={`/sites/${siteId}/forms`} className="text-blue-600 hover:underline">
              {t('form.back')}
            </Link>
            <span> · </span>
            <code className="font-mono text-xs">{item.id}</code>
          </p>
        </div>
        <div className="flex items-center gap-3 text-sm">
          {saved && <span className="text-emerald-600">{saved}</span>}
          {error && <span className="text-red-600">{error}</span>}
        </div>
      </Inline>

      <Inline gap={1}>
        <button
          type="button"
          onClick={() => setTab('definition')}
          className={TAB_CLASS(tab === 'definition')}
        >
          {t('form.definition')}
        </button>
        <button
          type="button"
          onClick={() => setTab('submissions')}
          className={TAB_CLASS(tab === 'submissions')}
        >
          {t('form.submissions')}
        </button>
      </Inline>

      {tab === 'definition' && (
        <Stack gap={3}>
          <Field label={t('form.name')} required>
            <input className={INPUT_CLASS} value={name} onChange={(e) => setName(e.target.value)} />
          </Field>
          <Field label={t('form.definition')}>
            <textarea
              className="min-h-64 w-full rounded border border-neutral-300 bg-neutral-50 p-2 font-mono text-xs focus:border-blue-500 focus:outline-none"
              value={definitionText}
              spellCheck={false}
              onChange={(e) => setDefinitionText(e.target.value)}
            />
          </Field>
          <Inline gap={2} align="center">
            <button
              type="button"
              onClick={save}
              disabled={busy}
              className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700 disabled:opacity-50"
            >
              {t('form.save')}
            </button>
            <ConfirmButton label={t('form.delete.confirm')} onConfirm={remove} />
          </Inline>
        </Stack>
      )}

      {tab === 'submissions' && <SubmissionsList siteId={siteId} formId={formId} />}
    </Stack>
  );
}