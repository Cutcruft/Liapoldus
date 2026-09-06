import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { Inline, Stack } from '@liapoldus/ui-kit';
import { runOperation, type ContentDetail, type Site } from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { Field } from '../components/Field';
import { ConfirmButton } from '../components/ConfirmButton';
import { JsonFieldsEditor } from '../content/JsonFieldsEditor';

const INPUT_CLASS =
  'rounded border border-neutral-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none';

export function ContentEditorPage() {
  const { siteId = '', contentId = '' } = useParams();
  const { api, t } = useAdmin();
  const site = useOperation<Site | null>('getSite', { siteId }, (d) =>
    typeof d === 'object' && d !== null && 'id' in d ? (d as Site) : null,
  );
  const content = useOperation<ContentDetail | null>(
    'getContent',
    { siteId, contentId },
    (d) => (typeof d === 'object' && d !== null && 'id' in d ? (d as ContentDetail) : null),
  );

  const [baseFields, setBaseFields] = useState<Record<string, unknown>>({});
  const [tlDrafts, setTlDrafts] = useState<Record<string, Record<string, unknown>>>({});
  const [activeLocale, setActiveLocale] = useState<string | null>(null);
  const [newLocale, setNewLocale] = useState('');
  const [saved, setSaved] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (content.state.status !== 'success' || !content.state.data) return;
    const d = content.state.data;
    setBaseFields(d.fields ?? {});
    const translations: Record<string, Record<string, unknown>> = {};
    for (const [locale, entry] of Object.entries(d.translations ?? {})) {
      translations[locale] = entry.fields ?? {};
    }
    setTlDrafts(translations);
    setActiveLocale(Object.keys(translations)[0] ?? null);
    setSaved('');
  }, [content.state]);

  const locales = Object.keys(tlDrafts).sort();

  const saveBase = async () => {
    setBusy(true);
    setError('');
    const res = await runOperation(api, 'updateContent', { siteId, contentId, fields: baseFields }, t);
    setBusy(false);
    if (res.ok) setSaved(t('content.saved'));
    else setError(res.detail);
  };

  const saveLocale = async (locale: string) => {
    setBusy(true);
    setError('');
    const res = await runOperation(api, 'putTranslation', { siteId, contentId, locale, fields: tlDrafts[locale] ?? {} }, t);
    setBusy(false);
    if (res.ok) setSaved(t('content.saved'));
    else setError(res.detail);
  };

  const removeLocale = async (locale: string) => {
    setBusy(true);
    setError('');
    const res = await runOperation(api, 'deleteTranslation', { siteId, contentId, locale }, t);
    setBusy(false);
    if (!res.ok) {
      setError(res.detail);
      return;
    }
    const next: Record<string, Record<string, unknown>> = {};
    for (const [l, f] of Object.entries(tlDrafts)) if (l !== locale) next[l] = f;
    setTlDrafts(next);
    setActiveLocale(Object.keys(next)[0] ?? null);
  };

  const addLocale = (e: React.FormEvent) => {
    e.preventDefault();
    const locale = newLocale.trim();
    if (!locale || tlDrafts[locale]) return;
    setTlDrafts((d) => ({ ...d, [locale]: {} }));
    setActiveLocale(locale);
    setNewLocale('');
  };

  const defaultLocale = site.state.status === 'success' && site.state.data ? site.state.data.defaultLocale : '—';

  if (content.state.status === 'loading') {
    return <p className="p-8 text-sm text-neutral-400">{t('common.loading')}</p>;
  }

  if (content.state.status === 'error') {
    return (
      <Stack pad={8} gap={2}>
        <p className="text-sm text-red-600">
          {t('common.error')}: {content.state.detail}
        </p>
        <button type="button" onClick={content.reload} className="w-fit rounded border border-neutral-300 px-3 py-1 text-sm">
          {t('common.reload')}
        </button>
      </Stack>
    );
  }

  const item = content.state.data!;

  return (
    <Stack pad={8} gap={4}>
      <Inline justify="between" align="center">
        <div>
          <h1 className="text-xl font-medium">{t('content.title')}</h1>
          <p className="text-sm text-neutral-500">
            <code>{item.collectionId}</code> · <code className="font-mono text-xs">{item.id}</code>
          </p>
        </div>
        <div className="flex items-center gap-3 text-sm">
          {saved && <span className="text-emerald-600">{saved}</span>}
          {error && <span className="text-red-600">{error}</span>}
        </div>
      </Inline>

      <Stack gap={2}>
        <h2 className="text-sm font-medium text-neutral-500">{t('content.base')}</h2>
        <p className="text-xs text-neutral-400">{t('content.baseHint', { locale: defaultLocale })}</p>
        <JsonFieldsEditor value={baseFields} onChange={setBaseFields} t={t} />
        <div>
          <button
            type="button"
            onClick={saveBase}
            disabled={busy}
            className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700 disabled:opacity-50"
          >
            {t('content.save')}
          </button>
        </div>
      </Stack>

      <Stack gap={2}>
        <h2 className="text-sm font-medium text-neutral-500">{t('content.translations')}</h2>
        {locales.length === 0 ? (
          <p className="text-sm text-neutral-400">{t('content.noTranslations')}</p>
        ) : (
          <>
            <Inline gap={1}>
              {locales.map((locale) => (
                <button
                  key={locale}
                  type="button"
                  onClick={() => setActiveLocale(locale)}
                  className={`rounded-t-md border border-b-0 px-3 py-1 text-xs ${
                    activeLocale === locale
                      ? 'border-blue-400 bg-blue-50 text-blue-700'
                      : 'border-neutral-300 text-neutral-500 hover:text-neutral-800'
                  }`}
                >
                  {locale}
                </button>
              ))}
            </Inline>
            {activeLocale && (
              <Stack gap={2}>
                <JsonFieldsEditor
                  key={activeLocale}
                  value={tlDrafts[activeLocale] ?? {}}
                  onChange={(next) => setTlDrafts((d) => ({ ...d, [activeLocale]: next }))}
                  t={t}
                />
                <Inline gap={2}>
                  <button
                    type="button"
                    onClick={() => saveLocale(activeLocale)}
                    disabled={busy}
                    className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700 disabled:opacity-50"
                  >
                    {t('content.save')}
                  </button>
                  <ConfirmButton label={t('content.deleteTranslation.confirm')} onConfirm={() => removeLocale(activeLocale)} />
                </Inline>
              </Stack>
            )}
          </>
        )}

        <form onSubmit={addLocale} className="flex items-end gap-2">
          <Field label={t('content.locale')}>
            <input
              className={INPUT_CLASS}
              value={newLocale}
              placeholder="en"
              onChange={(e) => setNewLocale(e.target.value)}
            />
          </Field>
          <button
            type="submit"
            disabled={!newLocale.trim() || Boolean(tlDrafts[newLocale.trim()])}
            className="rounded border border-neutral-300 px-3 py-1.5 text-sm text-neutral-600 hover:border-blue-300 hover:text-blue-600 disabled:opacity-40"
          >
            {t('content.addLocale')}
          </button>
        </form>
      </Stack>
    </Stack>
  );
}