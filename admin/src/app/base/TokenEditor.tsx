import { Inline, Stack } from '@liapoldus/ui-kit';
import type { Translate } from '../../runtime';
import { TOKEN_GROUPS } from '../../runtime';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ColorPicker } from '../components/ColorPicker';
import { TokenPreview } from '../components/TokenPreview';
import { draftToTokens, type TokensDraft } from './tokens-draft';

/**
 * Управляемый редактор цветов и скалярных групп токенов. Черновик живёт выше
 * (BaseSection): компонент чисто презентационный — все изменения выходят через
 * onDraftsChange, автсохранение и сборка TokenSet происходят на уровне раздела.
 * Это позволяет табу «Шрифты» редактировать тот же TokenSet без клобберинга.
 */
export function TokenEditor({
  draft,
  onDraftsChange,
  t,
}: {
  draft: TokensDraft;
  onDraftsChange: (next: TokensDraft) => void;
  t: Translate;
}) {
  const { colors, groups } = draft;
  const setColor = (i: number, patch: Partial<TokensDraft['colors'][number]>) =>
    onDraftsChange({ colors: colors.map((c, j) => (j === i ? { ...c, ...patch } : c)), groups });
  const addColor = () => onDraftsChange({ colors: [...colors, { name: '', light: '', dark: '' }], groups });
  const removeColor = (i: number) => onDraftsChange({ colors: colors.filter((_, j) => j !== i), groups });
  const setGroups = (next: TokensDraft['groups']) => onDraftsChange({ colors, groups: next });

  const setGroupValue = (group: string, key: string, value: string) =>
    setGroups({ ...groups, [group]: { ...groups[group], [key]: value } });
  const addGroupValue = (group: string) => setGroupValue(group, '', '');
  const removeGroupValue = (group: string, key: string) => {
    const next = { ...groups[group] };
    delete next[key];
    setGroups({ ...groups, [group]: next });
  };

  const groupName = (g: string) => t(T_LABEL[g as keyof typeof T_LABEL] ?? 'tokens.group.custom');

  return (
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
                        const prev = groups[g] ?? {};
                        const next = { ...prev };
                        delete next[k];
                        if (nk) next[nk] = v;
                        setGroups({ ...groups, [g]: next });
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
        <TokenPreview tokens={draftToTokens(draft)} />
      </section>
    </div>
  );
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