import { useEffect } from 'react';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';
import { useSelector } from '@liapoldus/ui-runtime';
import { APP_VERSION } from '../runtime';
import { useAdmin, ADMIN_TOKEN_KEY } from './admin-context';
import {
  activeTabKey,
  closeTab,
  getTabsStore,
  SETTINGS_TAB,
  siteIdOfTab,
  type TabItem,
} from './tabs-store';
import { Plus, X } from 'lucide-react';

function tabPath(tab: TabItem): string {
  return tab.kind === 'settings' ? '/settings' : `/sites/${tab.siteId ?? ''}`;
}

/**
 * Каркас админки (слайс R1): шапка-бренд, трей URL-синхронных вкладок
 * (сайты + системные настройки) и контентная область. Старая левая
 * навигация (Обзор/Сайты/Настройки) удалена — главная рендерит карточки.
 */
export function AppShell() {
  const { t, tokenStore } = useAdmin();
  const navigate = useNavigate();
  const location = useLocation();
  const pathname = location.pathname;
  const tabs = useSelector(getTabsStore(), (s) => s.tabs);
  const activeKey = activeTabKey(pathname);

  // Восстановление вкладки при прямом переходе по URL (deep-link/рефреш).
  useEffect(() => {
    if (!activeKey) return;
    const store = getTabsStore();
    if (store.getState().tabs.some((tb) => tb.key === activeKey)) return;
    const siteId = siteIdOfTab(activeKey);
    store.setState({
      tabs: [
        ...store.getState().tabs,
        {
          key: activeKey,
          kind: siteId ? 'site' : 'settings',
          label: siteId ?? t('tab.settings'),
          siteId,
        },
      ],
    });
  }, [activeKey, t]);

  const logout = () => {
    tokenStore.setState({ token: null });
    globalThis.localStorage?.removeItem(ADMIN_TOKEN_KEY);
    navigate('/login', { replace: true });
  };

  return (
    <div className="flex h-full min-h-0 flex-col bg-neutral-50 text-neutral-900">
      <header className="flex shrink-0 items-center gap-2 border-b border-neutral-200 bg-white px-3 py-1.5">
        <button
          type="button"
          onClick={() => navigate('/')}
          className="shrink-0 rounded px-1.5 py-1 text-sm font-semibold tracking-tight text-neutral-900 hover:bg-neutral-100"
        >
          {t('app.name')}
        </button>
        <span className="shrink-0 text-xs text-neutral-400">{APP_VERSION}</span>
        <div className="mx-1 h-5 w-px shrink-0 bg-neutral-200" />
        <nav aria-label="Открытые вкладки" role="tablist" className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto">
          {tabs.map((tab) => {
            const active = tab.key === activeKey;
            return (
              <div
                key={tab.key}
                role="tab"
                aria-selected={active}
                className={`flex shrink-0 items-center gap-0.5 rounded-md text-sm ${
                  active ? 'bg-blue-50 text-blue-800' : 'text-neutral-600 hover:bg-neutral-100'
                }`}
              >
                <button
                  type="button"
                  onClick={() => navigate(tabPath(tab))}
                  className={`max-w-40 truncate rounded-l-md px-2 py-1 ${
                    active ? 'font-medium' : ''
                  }`}
                >
                  {tab.kind === 'settings' ? t('tab.settings') : tab.label}
                </button>
                <button
                  type="button"
                  aria-label={t('tab.close')}
                  onClick={() => closeTab(tab.key, pathname, navigate)}
                  className="rounded-r-md p-1 text-neutral-400 hover:text-neutral-800"
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              </div>
            );
          })}
          <button
            type="button"
            aria-label={t('tab.new')}
            onClick={() => navigate('/')}
            className="shrink-0 rounded-md p-1 text-neutral-400 hover:bg-neutral-100 hover:text-neutral-800"
          >
            <Plus className="h-4 w-4" />
          </button>
        </nav>
        <button
          type="button"
          onClick={logout}
          className="shrink-0 rounded px-2 py-1 text-sm text-neutral-500 hover:bg-neutral-100 hover:text-neutral-800"
        >
          {t('auth.logout')}
        </button>
      </header>
      <main className="min-h-0 flex-1 overflow-auto" aria-label="Контент">
        <Outlet />
      </main>
    </div>
  );
}