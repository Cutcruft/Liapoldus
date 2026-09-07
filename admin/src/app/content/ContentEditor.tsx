import { useEffect, useMemo, useState } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import { Inline, Stack } from '@liapoldus/ui-kit';
import { runOperation, type ContentDetail, type Site } from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { JsonFieldsEditor } from './JsonFieldsEditor';
import { ConfirmButton } from '../components/ConfirmButton';
import { contentTitle } from './ContentList';
import {
  clearContentDraft,
  getContentDraft,
  setContentDraft,
  useContentDraft,
} from './content-drafts';

const INPUT_CLASS = 'rounded border border-neutral-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none';

function translationKeys(item: ContentDetail, baseLocale: string): string[] {
  return Object.keys(item.translations ?? {})
    .filter((l) => l !== baseLocale)
    .sort();
}

/**
 * Редактор контента (R3): селектор языка (базовый + переводы), поля через
 * JsonFieldsEditor (rich-text — tiptap), блок «Переводы» со статусами.
 * Несохранённые правки живут в slice-сторе (content-drafts) — смена языка их не теряет.
 */
export function ContentEditor({ contentId }: { contentId: string }) {
  const { siteId = '' } = useParams();
  const [, setSearchParams] = useSearchParams();
  const { api, t } = useAdmin();

  const site = useOperation<Site | null>('getSite', { siteId }, (d) =>
    typeof d === 'object' && d !== null ? (d as Site) : null,
  );
  const content = useOperation<ContentDetail | null>(
    'getContent',
    { siteId, contentId },
    (d) => (typeof d === 'object' && d !== null && 'id' in d ? (d as ContentDetail) : null),
  );

  const baseLocale = site.state.status === 'success' ? site.state.data?.defaultLocale ?? '' : '';
  const item = content.state.status === 'success' ? content.state.data : null;

  const translationKeysMemo = useMemo(
    () => (item ? translationKeys(item, baseLocale) : []),
    [item, baseLocale],
  );
  const [activeLocale, setActiveLocale] = useState<string>('base');
  const [newLocale, setNewLocale] = useState('');
  const [saved, setSaved] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  // После загрузки/перезагрузки убеждаемся, что активный язык ещё существует.
  useEffect(() => {
    if (activeLocale !== 'base' && !translationKeysMemo.includes(activeLocale)) {
      setActiveLocale('base');
    }
  }, [translationKeysMemo, activeLocale]);

  const serverFields = (locale: string): Record<string, unknown> => {
    if (!item) return {};
    if (locale === 'base') return item.fields ?? {};
    return item.translations?.[locale]?.fields ?? {};
  };

  const currentFields = (locale: string): Record<string, unknown> =>
    getContentDraft(siteId, contentId, locale)?.fields ?? serverFields(locale);

  const dirty = useContentDraft(siteId, contentId, activeLocale);

  const changeFields = (locale: string, next: Record<string, unknown>): void => {
    if (saved) setSaved('');
    if (error) setError('');
    setContentDraft(siteId, contentId, locale, next);
  };

  const saveActive = async () => {
    setBusy(true);
    setError('');
    const args =
      activeLocale === 'base'
        ? { siteId, contentId, fields: currentFields('base') }
        : { siteId, contentId, locale: activeLocale, fields: currentFields(activeLocale) };
    const res = await runOperation(api, activeLocale === 'base' ? 'updateContent' : 'putTranslation', args, t);
    setBusy(false);
    if (!res.ok) {
      setError(res.detail);
      return;
    }
    clearContentDraft(siteId, contentId, activeLocale);
    content.reload();
    setSaved(t('content.saved'));
  };

  const removeTranslation = async (locale: string) => {
    setBusy(true);
    setError('');
    const res = await runOperation(api, 'deleteTranslation', { siteId, contentId, locale }, t);
    setBusy(false);
    if (!res.ok) {
      setError(res.detail);
      return;
    }
    clearContentDraft(siteId, contentId, locale);
    if (activeLocale === locale) setActiveLocale('base');
    content.reload();
  };

  const addLocale = (e: React.FormEvent) => {
    e.preventDefault();
    const locale = newLocale.trim();
    if (!locale || locale === baseLocale || translationKeysMemo.includes(locale)) return;
    setActiveLocale(locale);
    setNewLocale('');
  };

  if (content.state.status === 'loading') {
    return <p className="text-sm text-neutral-400">{t('common.loading')}</p>;
  }

  if (content.state.status === 'error') {
    return (
      <Stack gap={2}>
        <p className="text-sm text-red-600">
          {t('common.error')}: {content.state.detail}
        </p>
        <button type="button" onClick={content.reload} className="w-fit rounded border border-neutral-300 px-3 py-1 text-sm">
          {t('common.reload')}
        </button>
      </Stack>
    );
  }

  const itemData = item!;
  const editingTranslation = activeLocale !== 'base';
  const translationEmpty = !itemData.translations?.[activeLocale]?.fields;

  return (
    <Stack gap={4}>
      <Inline justify="between" align="center">
        <div>
          <button
            type="button"
            onClick={() => setSearchParams({ mode: 'content' })}
            className="mb-1 inline-flex items-center gap-1 rounded border border-neutral-300 px-2 py-1 text-xs text-neutral-600 hover:border-blue-300 hover:text-blue-600"
          >
            ← {t('content.back')}
          </button>
          <h1 className="text-xl font-medium">{contentTitle(itemData.fields, itemData.id)}</h1>
          <p className="text-sm text-neutral-500">
            <code>{itemData.collectionId}</code> · <code className="font-mono text-xs">{itemData.id}</code>
          </p>
        </div>
        <div className="flex items-center gap-3 text-sm">
          {dirty && <span className="text-amber-600">{t('content.draftUnsaved')}</span>}
          {saved && <span className="text-emerald-600">{saved}</span>}
          {error && <span className="text-red-600">{error}</span>}
        </div>
      </Inline>

      <div role="tablist" aria-label={t('content.language')} className="flex w-fit items-center gap-1 rounded-lg border border-neutral-200 bg-neutral-100 p-0.5">
        <button
          type="button"
          role="tab"
          aria-selected={activeLocale === 'base'}
          onClick={() => setActiveLocale('base')}
          className={`rounded-md px-3 py-1 text-sm ${activeLocale === 'base' ? 'bg-white font-medium text-neutral-900 shadow-sm' : 'text-neutral-600 hover:text-neutral-800'}`}
        >
          {baseLocale || 'base'}
        </button>
        {translationKeysMemo.map((locale) => (
          <button
            key={locale}
            type="button"
            role="tab"
            aria-selected={activeLocale === locale}
            aria-label={locale}
            onClick={() => setActiveLocale(locale)}
            className={`rounded-md px-3 py-1 text-sm ${activeLocale === locale ? 'bg-white font-medium text-neutral-900 shadow-sm' : 'text-neutral-600 hover:text-neutral-800'}`}
          >
            {locale}
          </button>
        ))}
      </div>

      <Stack gap={2}>
        <h2 className="text-sm font-medium text-neutral-500">
          {editingTranslation ? activeLocale : t('content.base')}
        </h2>
        {editingTranslation ? (
          <p className="text-xs text-neutral-400">
            {translationEmpty ? t('content.translation.noFields') : t('content.baseHint', { locale: baseLocale })}
          </p>
        ) : (
          <p className="text-xs text-neutral-400">{t('content.baseHint', { locale: baseLocale })}</p>
        )}
        <JsonFieldsEditor
          key={activeLocale}
          value={currentFields(activeLocale)}
          onChange={(next) => changeFields(activeLocale, next)}
          t={t}
          siteId={siteId}
        />
        <Inline gap={2}>
          <button
            type="button"
            onClick={saveActive}
            disabled={busy}
            className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700 disabled:opacity-50"
          >
            {t('content.save')}
          </button>
          {editingTranslation && (
            <ConfirmButton label={t('content.deleteTranslation.confirm')} onConfirm={() => removeTranslation(activeLocale)} />
          )}
        </Inline>
      </Stack>

      <Stack gap={2}>
        <h2 className="text-sm font-medium text-neutral-500">{t('content.translations')}</h2>
        {translationKeysMemo.length === 0 ? (
          <p className="text-sm text-neutral-400">{t('content.noTranslations')}</p>
        ) : (
          <div className="overflow-hidden rounded border border-neutral-200">
            <div className="grid grid-cols-[minmax(0,1fr)_8rem_auto] items-center gap-3 bg-neutral-50 px-4 py-2 text-xs font-medium text-neutral-500">
              <span>{t('content.locale')}</span>
              <span>{t('content.translations')}</span>
              <span />
            </div>
            {translationKeysMemo.map((locale) => {
              const fields = itemData.translations?.[locale]?.fields;
              const translated = fields !== undefined && Object.keys(fields).length > 0;
              return (
                <div
                  key={locale}
                  className="grid grid-cols-[minmax(0,1fr)_8rem_auto] items-center gap-3 border-t border-neutral-100 px-4 py-2 text-sm"
                >
                  <span className={activeLocale === locale ? 'font-medium text-blue-700' : 'text-neutral-900'}>{locale}</span>
                  <span className={`text-xs ${translated ? 'text-emerald-700' : 'text-neutral-400'}`}>
                    {translated ? t('content.status.translated') : t('content.status.missing')}
                  </span>
                  <Inline gap={2} justify="end">
                    <button
                      type="button"
                      onClick={() => setActiveLocale(locale)}
                      className="rounded border border-neutral-300 px-2 py-1 text-xs text-neutral-600 hover:border-blue-300 hover:text-blue-600"
                    >
                      {t('content.open')}
                    </button>
                    <ConfirmButton label={t('content.deleteTranslation.confirm')} onConfirm={() => removeTranslation(locale)} />
                  </Inline>
                </div>
              );
            })}
          </div>
        )}

        <form onSubmit={addLocale} className="flex items-end gap-2">
          <label className="flex flex-col gap-1 text-sm text-neutral-500">
            {t('content.locale.new')}
            <input
              className={`${INPUT_CLASS} w-32`}
              value={newLocale}
              placeholder="en"
              onChange={(e) => setNewLocale(e.target.value)}
            />
          </label>
          <button
            type="submit"
            disabled={!newLocale.trim() || newLocale.trim() === baseLocale || translationKeysMemo.includes(newLocale.trim())}
            className="rounded border border-neutral-300 px-3 py-1.5 text-sm text-neutral-600 hover:border-blue-300 hover:text-blue-600 disabled:opacity-40"
          >
            {t('content.addLocale')}
          </button>
        </form>
      </Stack>
    </Stack>
  );
}