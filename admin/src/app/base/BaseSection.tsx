import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { runOperation, type SiteSettings } from '../../runtime';
import { useAdmin } from '../admin-context';
import { AssetPicker } from '../assets/AssetPicker';
import { useOperation } from '../use-operation';
import { loadCatalog, type BuiltinComponent } from '../editor/schemas';

const INPUT = 'w-full rounded border border-neutral-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none';

function coerce(raw: unknown): SiteSettings | null {
  if (!raw || typeof raw !== 'object') return null;
  const value = raw as Partial<SiteSettings>;
  if (typeof value.siteId !== 'string' || typeof value.defaultLocale !== 'string') return null;
  return {
    siteId: value.siteId,
    defaultLocale: value.defaultLocale,
    defaultLayoutSectionId: value.defaultLayoutSectionId,
    head: {
      titleTemplate: value.head?.titleTemplate ?? '',
      description: value.head?.description ?? '',
      faviconAssetId: value.head?.faviconAssetId ?? '',
      meta: value.head?.meta ?? {},
    },
  };
}

/** Site-wide presentation defaults. Page-level head/layout overrides arrive with page R10 wiring. */
export function BaseSection() {
  const { siteId = '' } = useParams();
  const { api, t } = useAdmin();
  const settings = useOperation<SiteSettings | null>('getSiteSettings', { siteId }, coerce);
  const [draft, setDraft] = useState<SiteSettings | null>(null);
  const [metaText, setMetaText] = useState('{}');
  const [pickerOpen, setPickerOpen] = useState(false);
  const [layoutOptions, setLayoutOptions] = useState<BuiltinComponent[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [saved, setSaved] = useState(false);
  const loadedSettings = settings.state.status === 'success' ? settings.state.data : null;

  useEffect(() => {
    const loaded = loadedSettings;
    if (!loaded) return;
    setDraft(loaded);
    setMetaText(JSON.stringify(loaded.head.meta, null, 2));
  }, [loadedSettings]);

  useEffect(() => {
    if (!siteId) return;
    let alive = true;
    void loadCatalog(api, siteId).then((list) => {
      if (alive) setLayoutOptions(list.filter((c) => c.acceptsPageContent));
    });
    return () => {
      alive = false;
    };
  }, [api, siteId]);

  if (settings.state.status === 'loading') return <p className="text-sm text-neutral-400">{t('common.loading')}</p>;
  if (settings.state.status === 'error') return <p className="text-sm text-red-600">{settings.state.detail}</p>;
  if (!draft) return <p className="text-sm text-neutral-400">{t('common.loading')}</p>;

  const updateHead = (key: keyof SiteSettings['head'], value: string) =>
    setDraft((current) => current ? { ...current, head: { ...current.head, [key]: value } } : current);

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    let meta: Record<string, string>;
    try {
      const parsed: unknown = JSON.parse(metaText || '{}');
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed) || Object.values(parsed).some((v) => typeof v !== 'string')) {
        throw new Error('meta must be an object of string values');
      }
      meta = parsed as Record<string, string>;
    } catch {
      setError('Meta должны быть JSON-объектом со строковыми значениями.');
      return;
    }
    setBusy(true);
    setError('');
    setSaved(false);
    const next = { ...draft, head: { ...draft.head, meta } };
    const result = await runOperation(api, 'updateSiteSettings', { siteId, settings: next }, t);
    setBusy(false);
    if (!result.ok) {
      setError(result.detail);
      return;
    }
    const response = coerce(result.data);
    setDraft(response ?? next);
    setMetaText(JSON.stringify((response ?? next).head.meta, null, 2));
    setSaved(true);
  };

  return (
    <form onSubmit={save} className="mx-auto max-w-2xl space-y-5">
      <div>
        <h2 className="text-xl font-medium text-neutral-900">База сайта</h2>
        <p className="mt-1 text-sm text-neutral-500">Общие язык, метаданные и favicon. Настройки страницы смогут их переопределить.</p>
      </div>
      <label className="block text-sm font-medium text-neutral-700">
        Язык по умолчанию
        <input className={`${INPUT} mt-1`} value={draft.defaultLocale} onChange={(e) => setDraft({ ...draft, defaultLocale: e.target.value })} />
      </label>
      <label className="block text-sm font-medium text-neutral-700">
        Каркас по умолчанию
        <select
          className={`${INPUT} mt-1`}
          value={draft.defaultLayoutSectionId ?? ''}
          onChange={(e) => setDraft({ ...draft, defaultLayoutSectionId: e.target.value || undefined })}
        >
          <option value="">Нет</option>
          {layoutOptions.map((c) => (
            <option key={c.type} value={c.type}>{c.label}</option>
          ))}
        </select>
        <span className="mt-1 block text-xs text-neutral-400">Секция, принимающая контент страницы; страницы без своего каркаса возьмут его.</span>
      </label>
      <label className="block text-sm font-medium text-neutral-700">
        Шаблон title
        <input className={`${INPUT} mt-1`} value={draft.head.titleTemplate ?? ''} onChange={(e) => updateHead('titleTemplate', e.target.value)} placeholder="%s · Название сайта" />
      </label>
      <label className="block text-sm font-medium text-neutral-700">
        Description
        <textarea className={`${INPUT} mt-1 min-h-20`} value={draft.head.description ?? ''} onChange={(e) => updateHead('description', e.target.value)} />
      </label>
      <div className="text-sm font-medium text-neutral-700">
        Favicon
        <div className="mt-1 flex gap-2">
          <input className={`${INPUT} flex-1`} readOnly value={draft.head.faviconAssetId ?? ''} placeholder="Не выбран" />
          <button type="button" onClick={() => setPickerOpen(true)} className="rounded border border-neutral-300 px-3 text-sm hover:bg-neutral-50">Выбрать</button>
          {draft.head.faviconAssetId && <button type="button" onClick={() => updateHead('faviconAssetId', '')} className="rounded border border-neutral-300 px-3 text-sm hover:bg-neutral-50">Сбросить</button>}
        </div>
      </div>
      <label className="block text-sm font-medium text-neutral-700">
        Дополнительные meta (JSON)
        <textarea className={`${INPUT} mt-1 min-h-32 font-mono text-xs`} value={metaText} onChange={(e) => setMetaText(e.target.value)} />
      </label>
      {error && <p className="text-sm text-red-600">{error}</p>}
      {saved && <p className="text-sm text-green-700">Сохранено.</p>}
      <button type="submit" disabled={busy} className="rounded bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50">{busy ? 'Сохраняю…' : 'Сохранить'}</button>
      <AssetPicker open={pickerOpen} onClose={() => setPickerOpen(false)} siteId={siteId} selectedId={draft.head.faviconAssetId} onSelect={(assetId) => updateHead('faviconAssetId', assetId)} />
    </form>
  );
}
