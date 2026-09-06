import { useEffect, useMemo, useRef, useState } from 'react';
import { useParams } from 'react-router-dom';
import { Inline, Stack } from '@liapoldus/ui-kit';
import { runOperation, type ColorToken, type TokenSet, TOKEN_GROUPS } from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ColorPicker } from '../components/ColorPicker';
import { TokenPreview } from '../components/TokenPreview';

const AUTOSAVE_MS = 500;

type SaveState = 'idle' | 'saving' | 'saved' | 'failed';

interface ColorDraft {
  name: string;
  light: string;
  dark: string;
}

/** Собирает TokenSet из свободных черновых значений (пустые строки отбрасываются). */
function draftToTokens(colors: ColorDraft[], groups: Record<string, Record<string, string>>): TokenSet {
  const next: TokenSet = { colors: [] };
  for (const c of colors) {
    const name = c.name.trim();
    if (!name) continue;
    const value: Partial<Record<'light' | 'dark', string>> = {};
    if (c.light.trim()) value.light = c.light.trim();
    if (c.dark.trim()) value.dark = c.dark.trim();
    if (!value.light && !value.dark) continue;
    next.colors.push({ name, value: { light: value.light ?? '', dark: value.dark ?? '' } });
  }
  for (const group of TOKEN_GROUPS) {
    const entries = Object.entries(groups[group] ?? {}).filter(
      ([k, v]) => k.trim() && v.trim(),
    );
    if (entries.length > 0) {
      (next as Record<string, unknown>)[group] = Object.fromEntries(entries);
    }
  }
  return next;
}

/** Разворачивает сохранённый TokenSet обратно в черновики, заполняя недостающие поля гуфами. */
function tokensToDraft(tokens: TokenSet): { colors: ColorDraft[]; groups: Record<string, Record<string, string>> } {
  const set: TokenSet = tokens ?? { colors: [] };
  const colors: ColorDraft[] = (set.colors ?? []).map((c) => ({
    name: c.name,
    light: c.value?.light ?? '',
    dark: c.value?.dark ?? '',
  }));
  const groups: Record<string, Record<string, string>> = {};
  for (const group of TOKEN_GROUPS) {
    groups[group] = { ...(set[group] ?? {}) };
  }
  return { colors, groups };
}

function sameTokenSet(a: TokenSet, b: TokenSet): boolean {
  const ca = (a.colors ?? []).map((c) => `${c.name}|${c.value?.light ?? ''}|${c.value?.dark ?? ''}`).join('¤');
  const cb = (b.colors ?? []).map((c) => `${c.name}|${c.value?.light ?? ''}|${c.value?.dark ?? ''}`).join('¤');
  if (ca !== cb) return false;
  for (const group of TOKEN_GROUPS) {
    const ga = a[group] ?? {};
    const gb = b[group] ?? {};
    const ka = Object.keys(ga).filter((k) => !k.trim());
    const kb = Object.keys(gb).filter((k) => !k.trim());
    if (ka.length !== kb.length) return false;
    for (const k of ka) if (ga[k] !== gb[k]) return false;
    for (const k of kb) if (gb[k] !== ga[k]) return false;
  }
  return true;
}

export function SiteTokensPage() {
  const { siteId = '' } = useParams();
  const { api, t } = useAdmin();
  const tokens = useOperation<TokenSet>('getTokens', { siteId }, (d: unknown) =>
    typeof d === 'object' && d !== null ? (d as TokenSet) : { colors: [] },
  );

  const [colors, setColors] = useState<ColorDraft[]>([]);
  const [groups, setGroups] = useState<Record<string, Record<string, string>>>({});
  const [loaded, setLoaded] = useState(false);
  const [saveState, setSaveState] = useState<SaveState>('idle');
  const [saveError, setSaveError] = useState('');

  useEffect(() => {
    if (!loaded && tokens.state.status === 'success') {
      const d = tokensToDraft(tokens.state.data);
      setColors(d.colors);
      setGroups(d.groups);
      setLoaded(true);
    }
  }, [loaded, tokens.state]);

  const current = useMemo(() => draftToTokens(colors, groups), [colors, groups]);

  const saveTimer = useRef<number | undefined>(undefined);
  useEffect(() => {
    if (!loaded) return;
    setSaveState('saving');
    const timer = window.setTimeout(() => {
      void (async () => {
        const res = await runOperation(api, 'updateTokens', { siteId, tokens: current }, t);
        setSaveState(res.ok ? 'saved' : 'failed');
        if (!res.ok) setSaveError(res.detail);
      })();
    }, AUTOSAVE_MS);
    saveTimer.current = timer;
    return () => window.clearTimeout(timer);
  }, [loaded, current, api, siteId, t]);

  const setColor = (i: number, patch: Partial<ColorDraft>) =>
    setColors((cs) => cs.map((c, j) => (j === i ? { ...c, ...patch } : c)));

  const addColor = () => setColors((cs) => [...cs, { name: '', light: '', dark: '' }]);
  const removeColor = (i: number) => setColors((cs) => cs.filter((_, j) => j !== i));

  const setGroupValue = (group: string, key: string, value: string) =>
    setGroups((g) => ({ ...g, [group]: { ...g[group], [key]: value } }));
  const addGroupValue = (group: string) => setGroupValue(group, '', '');
  const removeGroupValue = (group: string, key: string) => {
    setGroups((g) => {
      const next = { ...g[group] };
      delete next[key];
      return { ...g, [group]: next };
    });
  };

  const savedSet: TokenSet = tokens.state.status === 'success' ? tokens.state.data : { colors: [] };
  const dirty = loaded && !sameTokenSet(current, savedSet);
  if (tokens.state.status === 'loading') {
    return <p className="p-8 text-sm text-neutral-400">{t('common.loading')}</p>;
  }
  if (tokens.state.status === 'error') {
    return (
      <div className="p-8">
        <p className="text-sm text-red-600">
          {t('common.error')}: {tokens.state.detail}
        </p>
        <button type="button" onClick={tokens.reload} className="mt-2 rounded border border-neutral-300 px-3 py-1 text-sm">
          {t('common.reload')}
        </button>
      </div>
    );
  }

  const groupName = (g: string) => T_LABEL[g as keyof typeof T_LABEL];

  return (
    <Stack gap={6} pad={4}>
      <Inline justify="between" align="center">
        <h1 className="text-lg font-medium">{t('tokens.title')}</h1>
        <SaveIndicator saveState={saveState} dirty={dirty} t={t} />
      </Inline>
      {saveState === 'failed' && <p className="text-sm text-destructive">{t('tokens.failed', { detail: saveError })}</p>}

      <div className="grid gap-6 lg:grid-cols-2">
        <Stack gap={4}>
          <section className="rounded-lg border border-neutral-200 p-4">
            <Stack gap={3}>
              <Inline justify="between" align="center">
                <h2 className="text-sm font-medium text-neutral-600">{t('tokens.colors')}</h2>
                <Button type="button" variant="outline" size="sm" onClick={addColor}>
                  {t('tokens.color.add')}
                </Button>
              </Inline>
              {colors.length === 0 && <p className="text-sm text-neutral-400">{t('tokens.colors.none')}</p>}
              {colors.map((c, i) => (
                <div key={i} className="rounded-md border border-neutral-100 p-3">
                  <Inline gap={3} align="start">
                    <Stack gap={2} className="min-w-[8rem] flex-1">
                      <Input
                        aria-label={t('tokens.color.name')}
                        placeholder={t('tokens.color.name')}
                        value={c.name}
                        onChange={(e) => setColor(i, { name: e.target.value })}
                      />
                      <div className="flex items-center gap-2">
                        <ColorPicker label={t('tokens.color.light')} value={c.light} onChange={(v) => setColor(i, { light: v })} />
                      </div>
                      <div className="flex items-center gap-2">
                        <ColorPicker label={t('tokens.color.dark')} value={c.dark} onChange={(v) => setColor(i, { dark: v })} />
                      </div>
                    </Stack>
                    <Button type="button" variant="outline" size="sm" onClick={() => removeColor(i)}>
                      {t('tokens.remove')}
                    </Button>
                  </Inline>
                </div>
              ))}
            </Stack>
          </section>

          <section className="rounded-lg border border-neutral-200 p-4">
            <Stack gap={3}>
              <Inline justify="between" align="center">
                <h2 className="text-sm font-medium text-neutral-600">{t('tokens.groups')}</h2>
                <select
                  className="h-9 rounded-md border border-input bg-transparent px-2 text-sm"
                  value=""
                  onChange={(e) => {
                    if (e.target.value) addGroupValue(e.target.value);
                  }}
                  aria-label={t('tokens.group.choose')}
                >
                  <option value="">{t('tokens.group.choose')}</option>
                  {TOKEN_GROUPS.map((g) => (
                    <option key={g} value={g}>
                      {groupName(g)}
                    </option>
                  ))}
                </select>
              </Inline>
              {TOKEN_GROUPS.map((g) => (
                <div key={g}>
                  <h3 className="mb-1 text-xs font-medium text-neutral-400">{groupName(g)}</h3>
                  {Object.entries(groups[g] ?? {}).map(([k, v]) => (
                    <Inline key={k} gap={2} className="mb-1">
                      <Input
                        aria-label={t('tokens.key')}
                        className="w-40"
                        value={k}
                        onChange={(e) => {
                          const nk = e.target.value;
                          setGroups((prev) => {
                            const next = { ...prev[g] };
                            delete next[k];
                            if (nk) next[nk] = v;
                            return { ...prev, [g]: next };
                          });
                        }}
                      />
                      <Input
                        aria-label={t('tokens.value')}
                        className="flex-1"
                        value={v}
                        onChange={(e) => setGroupValue(g, k, e.target.value)}
                      />
                      <Button type="button" variant="outline" size="sm" onClick={() => removeGroupValue(g, k)}>
                        ×
                      </Button>
                    </Inline>
                  ))}
                </div>
              ))}
            </Stack>
          </section>
        </Stack>

        <section className="rounded-lg border border-neutral-200 p-4">
          <h2 className="mb-2 text-sm font-medium text-neutral-600">{t('tokens.preview')}</h2>
          <TokenPreview tokens={current} />
        </section>
      </div>
    </Stack>
  );
}

interface SaveIndicatorProps {
  saveState: SaveState;
  dirty: boolean;
  t: import('../../runtime').Translate;
}

function SaveIndicator({ saveState, dirty, t }: SaveIndicatorProps) {
  if (saveState === 'saving') return <span className="text-xs text-neutral-400">{t('tokens.saving')}</span>;
  if (!dirty && saveState === 'saved') return <span className="text-xs text-green-600">{t('tokens.saved')}</span>;
  if (dirty) return <span className="text-xs text-amber-600">{t('tokens.dirty')}</span>;
  return <span className="text-xs text-green-600">{t('tokens.saved')}</span>;
}

const T_LABEL: Record<string, string> = {
  typography: 'tokens.group.typography',
  spacing: 'tokens.group.spacing',
  shadows: 'tokens.group.shadows',
  borders: 'tokens.group.borders',
  breakpoints: 'tokens.group.breakpoints',
  zIndex: 'tokens.group.zIndex',
  opacity: 'tokens.group.opacity',
  transitions: 'tokens.group.transitions',
  custom: 'tokens.group.custom',
};
