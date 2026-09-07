import { TOKEN_GROUPS } from '../../runtime';
import type { TokenSet } from '../../runtime';

/** Черновик цветной роли редактора: свободные строки, пустые отбрасываются при сборке. */
export interface ColorDraft {
  name: string;
  light: string;
  dark: string;
}

/**
 * Общее хранилище черновиков токенов (цвета + скалярные группы). Лежит выше
 * табов (BaseSection): и «Токены», и «Шрифты» редактируют один и тот же набор,
 * чтобы не было двух независимых whole-set PUT.
 */
export interface TokensDraft {
  colors: ColorDraft[];
  groups: Record<string, Record<string, string>>;
}

const EMPTY: { colors: TokenSet['colors'] } = { colors: [] };

/** Собирает TokenSet из черновых значений (пустые строки/ключи отбрасываются). */
export function draftToTokens(draft: TokensDraft, fonts: TokenSet['fonts'] = []): TokenSet {
  const next: TokenSet = { colors: [] };
  for (const c of draft.colors) {
    const name = c.name.trim();
    if (!name) continue;
    const value: Partial<Record<'light' | 'dark', string>> = {};
    if (c.light.trim()) value.light = c.light.trim();
    if (c.dark.trim()) value.dark = c.dark.trim();
    if (!value.light && !value.dark) continue;
    next.colors.push({ name, value: { light: value.light ?? '', dark: value.dark ?? '' } });
  }
  for (const group of TOKEN_GROUPS) {
    const entries = Object.entries(draft.groups[group] ?? {}).filter(([k, v]) => k.trim() && v.trim());
    if (entries.length > 0) {
      (next as Record<string, unknown>)[group] = Object.fromEntries(entries);
    }
  }
  next.fonts = fonts ?? [];
  return next;
}

/** Разворачивает сохранённый TokenSet обратно в черновики (groups дополняются пустыми map). */
export function tokensToDraft(tokens: TokenSet | null | undefined): TokensDraft {
  const set: TokenSet = tokens ?? (EMPTY as TokenSet);
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

/**
 * Канонический отпечаток TokenSet для проверки «есть ли несохранённое»: цвета,
 * шрифты и все скалярные группы в детерминированном порядке. Используется чтобы
 * сравнить текущий draft со значением последнего сохранения.
 */
export function tokenSetFingerprint(set: TokenSet): string {
  const colors = (set.colors ?? [])
    .map((c) => `${c.name}|${c.value?.light ?? ''}|${c.value?.dark ?? ''}`)
    .join('¤');
  const fonts = (set.fonts ?? [])
    .map((f) => `${f.family}|${f.assetId}|${f.weight ?? ''}|${f.style ?? ''}`)
    .join('¤');
  const scalars: string[] = [];
  for (const group of TOKEN_GROUPS) {
    for (const [k, v] of Object.entries(set[group] ?? {})) {
      scalars.push(`${group}.${k}=${v}`);
    }
  }
  return [colors, fonts, scalars.sort().join('&')].join('§');
}