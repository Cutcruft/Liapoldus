import { useEffect, useMemo, useRef, useState } from 'react';
import { useParams } from 'react-router-dom';
import { runOperation, type FontToken, type SiteSettings, type TokenSet, type Translate } from '../../runtime';
import { useAdmin } from '../admin-context';
import { AssetPicker } from '../assets/AssetPicker';
import { useOperation } from '../use-operation';
import { loadCatalog, type BuiltinComponent } from '../editor/schemas';
import { FontsEditor } from './FontsEditor';
import { TokenEditor } from './TokenEditor';
import { draftToTokens, tokenSetFingerprint, tokensToDraft, type TokensDraft } from './tokens-draft';

const AUTOSAVE_MS = 500;
const INPUT = 'w-full rounded border border-neutral-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none';

type SaveState = 'idle' | 'saving' | 'saved' | 'failed';
type TabKey = 'settings' | 'tokens' | 'fonts';
const TABS: TabKey[] = ['settings', 'tokens', 'fonts'];

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

/**
 * Site-wide presentation defaults (R10). Подтабы «Настройки | Токены | Шрифты»
 * в разделе «База»: настройки сохраняются вручную (updateSiteSettings), токены
 * и шрифты — один общий черновик TokenSet с автсохранением (updateTokens).
 * Черновик поднят сюда, чтобы табы «Токены» и «Шрифты» писали в один набор и
 * не клобберили друг друга двумя независимыми whole-set PUT.
 */
export function BaseSection() {
  const { siteId = '' } = useParams();
  const { api, t } = useAdmin();
  const [tab, setTab] = useState<TabKey>('settings');

  // --- Настройки сайта (ручное сохранение) ---
  const settings = useOperation<SiteSettings | null>('getSiteSettings', { siteId }, coerce);
  const [draft, setDraft] = useState<SiteSettings | null>(null);
  const [metaText, setMetaText] = useState('{}');
  const [pickerOpen, setPickerOpen] = useState(false);
  const [layoutOptions, setLayoutOptions] = useState<BuiltinComponent[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [saved, setSaved] = useState(false);
  const loadedSettings = settings.state.status === 'success' ? settings.state.data : null;

  // --- Токены (общий черновик «Токены» + «Шрифты», автсохранение) ---
  const tokens = useOperation<TokenSet>('getTokens', { siteId }, (raw) =>
    typeof raw === 'object' && raw !== null ? (raw as TokenSet) : { colors: [] },
  );
  const [tokenDraft, setTokenDraft] = useState<TokensDraft | null>(null);
  const [fonts, setFonts] = useState<FontToken[]>([]);
  const [tokensLoaded, setTokensLoaded] = useState(false);
  const [tokensSaveState, setTokensSaveState] = useState<SaveState>('idle');
  const [tokensSaveError, setTokensSaveError] = useState('');
  const [lastSaved, setLastSaved] = useState('');

  useEffect(() => {
    if (tokensLoaded || tokens.state.status !== 'success') return;
    const set = tokens.state.data;
    setTokenDraft(tokensToDraft(set));
    setFonts(set.fonts ?? []);
    setLastSaved(tokenSetFingerprint(set));
    setTokensLoaded(true);
  }, [tokensLoaded, tokens.state]);

  const currentTokens = useMemo<TokenSet>(() => {
    if (!tokenDraft) return { colors: [] };
    return draftToTokens(tokenDraft, fonts);
  }, [tokenDraft, fonts]);
  const dirty = tokensLoaded && tokenSetFingerprint(currentTokens) !== lastSaved;

  const saveTimer = useRef<number | undefined>(undefined);
  useEffect(() => {
    if (!tokensLoaded || !tokenDraft || !dirty) return;
    setTokensSaveState('saving');
    const timer = window.setTimeout(() => {
      void (async () => {
        const res = await runOperation(api, 'updateTokens', { siteId, tokens: currentTokens }, t);
        if (res.ok) {
          setTokensSaveState('saved');
          if (typeof res.data === 'object' && res.data !== null) {
            setLastSaved(tokenSetFingerprint(res.data as TokenSet));
          }
        } else {
          setTokensSaveState('failed');
          setTokensSaveError(res.detail);
        }
      })();
    }, AUTOSAVE_MS);
    saveTimer.current = timer;
    return () => window.clearTimeout(timer);
  }, [tokensLoaded, tokenDraft, dirty, currentTokens, api, siteId, t]);

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
    setDraft((current) => (current ? { ...current, head: { ...current.head, [key]: value } } : current));

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

  const reloadTokens = () => {
    setTokensLoaded(false);
    setTokenDraft(null);
    setTokensSaveState('idle');
    tokens.reload();
  };

  return (
    <div className="mx-auto max-w-3xl space-y-5">
      <div>
        <h2 className="text-xl font-medium text-neutral-900">База сайта</h2>
        <p className="mt-1 text-sm text-neutral-500">Язык, метаданные, дизайн-токены и шрифты. Настройки страницы смогут их переопределить.</p>
      </div>

      <div role="tablist" aria-label="Разделы базы" className="flex items-center gap-1 border-b border-neutral-200">
        {TABS.map((key) => (
          <button
            key={key}
            type="button"
            role="tab"
            aria-selected={tab === key}
            onClick={() => setTab(key)}
            className={`rounded-t-md border-b-2 px-3 py-2 text-sm ${
              tab === key
                ? 'border-blue-600 font-medium text-blue-800'
                : 'border-transparent text-neutral-600 hover:text-neutral-800'
            }`}
          >
            {tabLabel(key, t)}
          </button>
        ))}
      </div>

      {tab === 'settings' ? (
        <form onSubmit={save} className="space-y-5">
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
      ) : tab === 'tokens' ? (
        <TokensTab
          t={t}
          state={tokens.state}
          loaded={tokensLoaded}
          draft={tokenDraft}
          saveState={tokensSaveState}
          dirty={dirty}
          error={tokensSaveError}
          onDraft={setTokenDraft}
          onReload={reloadTokens}
        />
      ) : (
        <FontsTab
          t={t}
          state={tokens.state}
          loaded={tokensLoaded}
          fonts={fonts}
          siteId={siteId}
          saveState={tokensSaveState}
          dirty={dirty}
          error={tokensSaveError}
          onFonts={setFonts}
          onReload={reloadTokens}
        />
      )}
    </div>
  );
}

function tabLabel(key: TabKey, t: Translate): string {
  if (key === 'settings') return t('tokens.tab.settings');
  if (key === 'tokens') return t('tokens.tab.tokens');
  return t('tokens.tab.fonts');
}

function TokensTab({
  t,
  state,
  loaded,
  draft,
  saveState,
  dirty,
  error,
  onDraft,
  onReload,
}: {
  t: Translate;
  state: { status: string; detail?: string };
  loaded: boolean;
  draft: TokensDraft | null;
  saveState: SaveState;
  dirty: boolean;
  error: string;
  onDraft: (next: TokensDraft) => void;
  onReload: () => void;
}) {
  if (!loaded && state.status === 'loading') return <p className="text-sm text-neutral-400">{t('common.loading')}</p>;
  if (!loaded && state.status === 'error') {
    return (
      <div>
        <p className="text-sm text-red-600">
          {t('common.error')}: {String(state.detail ?? '')}
        </p>
        <button type="button" onClick={onReload} className="mt-2 rounded border border-neutral-300 px-3 py-1 text-sm">
          {t('common.reload')}
        </button>
      </div>
    );
  }
  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium text-neutral-700">{t('tokens.title')}</h3>
        <SaveIndicator saveState={saveState} dirty={dirty} t={t} />
      </div>
      {saveState === 'failed' && <p className="text-sm text-destructive">{t('tokens.failed', { detail: error })}</p>}
      {draft && <TokenEditor draft={draft} onDraftsChange={onDraft} t={t} />}
    </div>
  );
}

function FontsTab({
  t,
  state,
  loaded,
  fonts,
  siteId,
  saveState,
  dirty,
  error,
  onFonts,
  onReload,
}: {
  t: Translate;
  state: { status: string; detail?: string };
  loaded: boolean;
  fonts: FontToken[];
  siteId: string;
  saveState: SaveState;
  dirty: boolean;
  error: string;
  onFonts: (next: FontToken[]) => void;
  onReload: () => void;
}) {
  if (!loaded && state.status === 'loading') return <p className="text-sm text-neutral-400">{t('common.loading')}</p>;
  if (!loaded && state.status === 'error') {
    return (
      <div>
        <p className="text-sm text-red-600">
          {t('common.error')}: {String(state.detail ?? '')}
        </p>
        <button type="button" onClick={onReload} className="mt-2 rounded border border-neutral-300 px-3 py-1 text-sm">
          {t('common.reload')}
        </button>
      </div>
    );
  }
  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium text-neutral-700">{t('fonts.title')}</h3>
        <SaveIndicator saveState={saveState} dirty={dirty} t={t} />
      </div>
      {saveState === 'failed' && <p className="text-sm text-destructive">{t('tokens.failed', { detail: error })}</p>}
      <FontsEditor fonts={fonts} onChange={onFonts} siteId={siteId} t={t} />
    </div>
  );
}

interface SaveIndicatorProps {
  saveState: SaveState;
  dirty: boolean;
  t: Translate;
}

function SaveIndicator({ saveState, dirty, t }: SaveIndicatorProps) {
  if (saveState === 'saving') return <span className="text-xs text-neutral-400">{t('tokens.saving')}</span>;
  if (dirty) return <span className="text-xs text-amber-600">{t('tokens.dirty')}</span>;
  return <span className="text-xs text-green-600">{t('tokens.saved')}</span>;
}