import { useParams, useSearchParams } from 'react-router-dom';
import { Inline, Stack } from '@liapoldus/ui-kit';
import { runOperation, type AdminForm, type Submission } from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { ConfirmButton } from '../components/ConfirmButton';
import { buildSubmissionsCsv, formatPayloadValue } from './form-utils';

/**
 * Ответы формы (R4): таблица сабмиссий с колонками динамически из definition
 * (значения полей), плюс дата, подробности (payload) и удаление. Экспорт CSV
 * с UTF-8 BOM. Список форм — по `?mode=forms`, назад возвращает туда же.
 */
export function FormDetail({ formId }: { formId: string }) {
  const { siteId = '' } = useParams();
  const [, setSearchParams] = useSearchParams();
  const { api, t } = useAdmin();

  const form = useOperation<AdminForm | null>(
    'getForm',
    { siteId, formId },
    (d) => (typeof d === 'object' && d !== null && 'id' in d ? (d as AdminForm) : null),
  );
  const list = useOperation<Submission[]>(
    'listSubmissions',
    { siteId, formId },
    (d) => (Array.isArray(d) ? (d as Submission[]) : []),
  );

  const item = form.state.status === 'success' ? form.state.data : null;
  const submissions = list.state.status === 'success' ? list.state.data : [];

  const fieldNames = item?.definition?.fields?.map((f) => f.name).filter(Boolean) ?? [];

  const back = () => setSearchParams({ mode: 'forms' });

  const remove = async (sub: Submission) => {
    const res = await runOperation(api, 'deleteSubmission', { siteId, formId, submissionId: sub.id }, t);
    if (res.ok) list.reload();
  };

  const exportCsv = () => {
    const csv = buildSubmissionsCsv(fieldNames, submissions, {
      date: t('form.submissions.date'),
      id: t('form.submissions.id'),
    });
    const blob = new Blob([`\uFEFF${csv}`], { type: 'text/csv;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${formId}-submissions.csv`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const gridColumns =
    fieldNames.length === 0
      ? 'minmax(0,10rem) minmax(0,1fr) minmax(0,8rem)'
      : [...fieldNames.map(() => 'minmax(8rem,1fr)'), 'minmax(0,10rem)', 'minmax(0,12rem)', 'minmax(0,8rem)'].join(' ');

  return (
    <Stack gap={4}>
      {form.state.status === 'loading' ? (
        <p className="text-sm text-neutral-400">{t('common.loading')}</p>
      ) : form.state.status === 'error' ? (
        <Stack gap={2}>
          <p className="text-sm text-red-600">
            {t('common.error')}: {form.state.detail}
          </p>
          <button type="button" onClick={form.reload} className="w-fit rounded border border-neutral-300 px-3 py-1 text-sm">
            {t('common.reload')}
          </button>
        </Stack>
      ) : item ? (
        <>
          <div className="flex items-center justify-between gap-3">
            <div className="min-w-0">
              <button
                type="button"
                onClick={back}
                className="text-sm text-neutral-500 hover:text-blue-600"
              >
                ← {t('form.back')}
              </button>
              <h2 className="truncate text-lg font-medium text-neutral-900">
                {item.name}
                <code className="ml-2 align-middle font-mono text-xs text-neutral-400">{item.id}</code>
              </h2>
            </div>
            <button
              type="button"
              onClick={exportCsv}
              disabled={submissions.length === 0}
              className="shrink-0 rounded border border-neutral-300 px-3 py-1.5 text-sm text-neutral-600 hover:border-blue-300 hover:text-blue-600 disabled:opacity-40"
            >
              {t('form.submissions.export')}
            </button>
          </div>

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
            (submissions.length === 0 ? (
              <p className="text-sm text-neutral-400">{t('form.submissions.none')}</p>
            ) : (
              <div className="overflow-x-auto rounded border border-neutral-200">
                <div
                  className="min-w-max items-center gap-3 border-b border-neutral-200 bg-neutral-50 px-4 py-2 text-xs font-medium text-neutral-500"
                  style={{ gridTemplateColumns: gridColumns }}
                >
                  {fieldNames.map((name) => (
                    <span key={name}>{name}</span>
                  ))}
                  <span>{t('form.submissions.date')}</span>
                  <span>{t('form.submissions.payload')}</span>
                  <span className="text-right">{t('common.actions')}</span>
                </div>
                {submissions.map((s) => (
                  <div
                    key={s.id}
                    className="grid min-w-max items-center gap-3 border-b border-neutral-100 px-4 py-2 text-sm"
                    style={{ gridTemplateColumns: gridColumns }}
                  >
                    {fieldNames.map((name) => (
                      <span key={name} className="truncate" title={formatPayloadValue(s.payload?.[name])}>
                        {formatPayloadValue(s.payload?.[name])}
                      </span>
                    ))}
                    <span className="text-xs text-neutral-500">{s.createdAt}</span>
                    <details>
                      <summary className="cursor-pointer text-xs text-neutral-500">JSON</summary>
                      <pre className="mt-1 max-h-40 overflow-auto rounded bg-neutral-50 p-2 text-xs">
                        {JSON.stringify(s.payload, null, 2)}
                      </pre>
                    </details>
                    <span className="flex justify-end">
                      <ConfirmButton
                        label={t('form.submissions.delete.confirm')}
                        onConfirm={() => remove(s)}
                      />
                    </span>
                  </div>
                ))}
              </div>
            ))}
        </>
      ) : null}
    </Stack>
  );
}