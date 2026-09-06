import { createSliceStore, type SliceStore } from '@liapoldus/ui-runtime';

export const EDITOR_SECTIONS = ['components', 'infra', 'pages', 'states', 'base'] as const;
export type EditorSectionKey = (typeof EDITOR_SECTIONS)[number];

export const MAINTENANCE_SECTIONS = ['content', 'forms', 'media'] as const;
export type MaintenanceSectionKey = (typeof MAINTENANCE_SECTIONS)[number];

export const DEFAULT_EDITOR_SECTION: EditorSectionKey = 'components';
export const DEFAULT_MAINTENANCE_SECTION: MaintenanceSectionKey = 'content';

export interface SiteTabState {
  /** Последний открытый раздел редактора по сайту (SPA-память, решение R2). */
  editorSection: Partial<Record<string, EditorSectionKey>>;
  /** Выбранный подраздел «Обслуживания» (SPA-память; в URL появится на R3/R4). */
  maintenanceSection: Partial<Record<string, MaintenanceSectionKey>>;
}

type SiteTabStore = SliceStore<SiteTabState>;

let store: SiteTabStore | null = null;

export function getSiteTabStore(): SiteTabStore {
  if (!store) store = createSliceStore<SiteTabState>({ editorSection: {}, maintenanceSection: {} });
  return store;
}

export function isEditorSection(v: string | null): v is EditorSectionKey {
  return !!v && (EDITOR_SECTIONS as readonly string[]).includes(v);
}

/** Запомнить раздел редактора сайта (возврат в режим открывает его). */
export function rememberEditorSection(siteId: string, section: EditorSectionKey): void {
  const s = getSiteTabStore();
  const map = s.getState().editorSection;
  if (map[siteId] === section) return;
  s.setState({ editorSection: { ...map, [siteId]: section } });
}

export function rememberedEditorSection(siteId: string): EditorSectionKey {
  return getSiteTabStore().getState().editorSection[siteId] ?? DEFAULT_EDITOR_SECTION;
}

/** Выбрать подраздел «Обслуживания» (пока только SPA-состояние). */
export function selectMaintenanceSection(siteId: string, section: MaintenanceSectionKey): void {
  const s = getSiteTabStore();
  const map = s.getState().maintenanceSection;
  if (map[siteId] === section) return;
  s.setState({ maintenanceSection: { ...map, [siteId]: section } });
}

export function currentMaintenanceSection(siteId: string): MaintenanceSectionKey {
  return getSiteTabStore().getState().maintenanceSection[siteId] ?? DEFAULT_MAINTENANCE_SECTION;
}

/** Сброс SPA-состояния вкладки сайта (для тестов). */
export function resetSiteTabStore(): void {
  getSiteTabStore().setState({ editorSection: {}, maintenanceSection: {} });
}