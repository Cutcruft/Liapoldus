import { createSliceStore, type SliceStore } from '@liapoldus/ui-runtime';

export type TabKind = 'site' | 'settings';

export interface TabItem {
  key: string;
  kind: TabKind;
  label: string;
  siteId?: string;
}

export interface TabsState {
  tabs: TabItem[];
}

export type TabsStore = SliceStore<TabsState>;

const SITE_PREFIX = 'site:';
export const SETTINGS_TAB = 'settings';

export const siteTabKey = (siteId: string) => `${SITE_PREFIX}${siteId}`;
export const siteIdOfTab = (key: string) =>
  key.startsWith(SITE_PREFIX) ? key.slice(SITE_PREFIX.length) : undefined;

let store: TabsStore | null = null;

/** Одиночный сторадж открытых вкладок (переживает маршрутизацию). */
export function getTabsStore(): TabsStore {
  if (!store) store = createSliceStore<TabsState>({ tabs: [] });
  return store;
}

/** Открыть сайт: добавляет вкладку (если нет) и фокусирует её. */
export function openOrFocusSite(siteId: string, label: string, navigate: (to: string) => void): void {
  const key = siteTabKey(siteId);
  const s = getTabsStore();
  const tabs = s.getState().tabs;
  if (!tabs.some((tb) => tb.key === key)) {
    s.setState({ tabs: [...tabs, { key, kind: 'site', label, siteId }] });
  } else {
    // актуализируем label (переименование сайта)
    s.setState({ tabs: tabs.map((tb) => (tb.key === key ? { ...tb, label } : tb)) });
  }
  navigate(`/sites/${siteId}`);
}

/** Открыть системные настройки: добавляет вкладку (если нет) и фокусирует. */
export function openOrFocusSettings(navigate: (to: string) => void): void {
  const s = getTabsStore();
  const tabs = s.getState().tabs;
  if (!tabs.some((tb) => tb.key === SETTINGS_TAB)) {
    s.setState({ tabs: [...tabs, { key: SETTINGS_TAB, kind: 'settings', label: '' }] });
  }
  navigate('/settings');
}

/** Обновить label вкладки (например, после загрузки имени сайта). */
export function updateTabLabel(key: string, label: string): void {
  const s = getTabsStore();
  const tabs = s.getState().tabs;
  if (!tabs.some((tb) => tb.key === key && tb.label === label)) {
    s.setState({ tabs: tabs.map((tb) => (tb.key === key ? { ...tb, label } : tb)) });
  }
}

/** Сброс открытых вкладок (для тестов). */
export function resetTabs(): void {
  getTabsStore().setState({ tabs: [] });
}

/** Закрыть вкладку; если закрывали активную — перейти на соседнюю или главную. */
export function closeTab(key: string, currentPath: string, navigate: (to: string) => void): void {
  const s = getTabsStore();
  const tabs = s.getState().tabs;
  const idx = tabs.findIndex((tb) => tb.key === key);
  if (idx < 0) return;
  const next = tabs.filter((tb) => tb.key !== key);
  s.setState({ tabs: next });
  const siteId = siteIdOfTab(key);
  const wasActive =
    (key === SETTINGS_TAB && currentPath.startsWith('/settings')) ||
    (siteId !== undefined && currentPath.startsWith(`/sites/${siteId}`));
  if (wasActive) {
    const fallback = next[idx - 1] ?? next[idx] ?? next[next.length - 1];
    navigate(fallback ? (fallback.kind === 'settings' ? '/settings' : `/sites/${fallback.siteId ?? ''}`) : '/');
  }
}

/** Ключ активной вкладки по pathname (site заглушка или null вне вкладок). */
export function activeTabKey(pathname: string): string | null {
  if (pathname === '/settings' || pathname.startsWith('/settings/')) return SETTINGS_TAB;
  const m = pathname.match(/^\/sites\/([^/]+)/);
  return m?.[1] ? siteTabKey(decodeURIComponent(m[1])) : null;
}