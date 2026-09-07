import { useEffect, useState } from 'react';
import { createSliceStore, type SliceStore } from '@liapoldus/ui-runtime';

/**
 * Draft-хранилище редактора контента (R3): несохранённые правки полей по
 * (siteId, contentId, locale). Смена языка/перехода не теряет введённое:
 * редактор читает draft поверх серверных значений и очищает его после успешного
 * сохранения. Модульный синглтон (как остальные slice-сторы админки).
 */

export interface ContentDraft {
  fields: Record<string, unknown>;
}

interface ContentDraftsState {
  drafts: Record<string, ContentDraft>;
}

function draftKey(siteId: string, contentId: string, locale: string): string {
  return `${siteId}::${contentId}::${locale}`;
}

type ContentDraftsStore = SliceStore<ContentDraftsState>;

let store: ContentDraftsStore | null = null;

export function getContentDraftsStore(): ContentDraftsStore {
  if (!store) store = createSliceStore<ContentDraftsState>({ drafts: {} });
  return store;
}

export function getContentDraft(siteId: string, contentId: string, locale: string): ContentDraft | null {
  return getContentDraftsStore().getState().drafts[draftKey(siteId, contentId, locale)] ?? null;
}

export function hasContentDraft(siteId: string, contentId: string, locale: string): boolean {
  return getContentDraft(siteId, contentId, locale) !== null;
}

export function setContentDraft(siteId: string, contentId: string, locale: string, fields: Record<string, unknown>): void {
  const s = getContentDraftsStore();
  const drafts = s.getState().drafts;
  const key = draftKey(siteId, contentId, locale);
  const next = { ...drafts, [key]: { fields } };
  // Не спамим хранилище обновлениями при эквивалентных кликах (пустая правка).
  if (drafts[key] === next[key]) return;
  s.setState({ drafts: next });
}

export function clearContentDraft(siteId: string, contentId: string, locale: string): void {
  const s = getContentDraftsStore();
  const key = draftKey(siteId, contentId, locale);
  if (!(key in s.getState().drafts)) return;
  const drafts = { ...s.getState().drafts };
  delete drafts[key];
  s.setState({ drafts });
}

/** Очистить все драфты контента (например после удаления). */
export function clearContentDrafts(siteId: string, contentId: string): void {
  const s = getContentDraftsStore();
  const prefix = `${siteId}::${contentId}::`;
  const drafts = s.getState().drafts;
  const next: Record<string, ContentDraft> = {};
  let changed = false;
  for (const [k, v] of Object.entries(drafts)) {
    if (k.startsWith(prefix)) {
      changed = true;
      continue;
    }
    next[k] = v;
  }
  if (changed) s.setState({ drafts: next });
}

/** Сброс всех драфтов (для тестов). */
export function resetContentDrafts(): void {
  getContentDraftsStore().setState({ drafts: {} });
}

const EMPTY_DRAFT_SLICE: ContentDraft | undefined = undefined;

/** Реактивный признак наличия несохранённого драфта (для индикатора «не сохранено»). */
export function useContentDraft(siteId: string, contentId: string, locale: string): boolean {
  const key = draftKey(siteId, contentId, locale);
  const [has, setHas] = useState(() => getContentDraftsStore().getState().drafts[key] !== undefined);
  useEffect(() => {
    const store = getContentDraftsStore();
    setHas(store.getState().drafts[key] !== undefined);
    return store.subscribeSlice(
      (s) => s.drafts[key] ?? EMPTY_DRAFT_SLICE,
      (next) => setHas(next !== undefined),
    );
  }, [key]);
  return has;
}