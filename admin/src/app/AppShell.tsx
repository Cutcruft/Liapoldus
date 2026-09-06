import { NavLink, Outlet, useNavigate } from 'react-router-dom';
import { Sidebar, Stack, Divider, Spacer } from '@liapoldus/ui-kit';
import { APP_VERSION } from '../runtime';
import { useAdmin, ADMIN_TOKEN_KEY } from './admin-context';
import type { AdminStringKey } from '../runtime';

const NAV: Array<{ to: string; end?: boolean; key: AdminStringKey }> = [
  { to: '/', end: true, key: 'nav.dashboard' },
  { to: '/sites', key: 'nav.sites' },
  { to: '/settings', key: 'nav.settings' },
];

function NavLinkItem({ to, end, label }: { to: string; end?: boolean; label: string }) {
  return (
    <NavLink
      to={to}
      end={end}
      className={({ isActive }) =>
        `block rounded px-2 py-1.5 text-sm ${
          isActive ? 'bg-neutral-100 font-medium text-neutral-900' : 'text-neutral-600 hover:bg-neutral-50'
        }`
      }
    >
      {label}
    </NavLink>
  );
}

/** Каркас админки: левая навигация + main-область (роутер). */
export function AppShell() {
  const { t, tokenStore } = useAdmin();
  const navigate = useNavigate();

  const logout = () => {
    tokenStore.setState({ token: null });
    globalThis.localStorage?.removeItem(ADMIN_TOKEN_KEY);
    navigate('/login', { replace: true });
  };

  return (
    <div className="flex h-full min-h-0 bg-white text-neutral-900">
      <Sidebar side="left" width="sm" contentClassName="bg-neutral-50" className="border-r border-neutral-200">
        <Stack gap={1} pad={3} className="h-full">
          <div className="px-2 py-1.5 text-sm font-semibold tracking-wide uppercase text-neutral-500">
            Liapoldus
          </div>
          <div className="px-2 text-xs text-neutral-400">конструктор сайтов</div>
          <Divider space={2} />
          <nav aria-label="Главное меню">
            <Stack gap={1}>
              {NAV.map((item) => (
                <NavLinkItem key={item.to} to={item.to} end={item.end} label={t(item.key)} />
              ))}
            </Stack>
          </nav>
          <Spacer />
          <div className="px-2 py-1 text-xs text-neutral-400">{APP_VERSION}</div>
          <button
            type="button"
            onClick={logout}
            className="block w-full rounded px-2 py-1.5 text-left text-sm text-neutral-500 hover:bg-neutral-100 hover:text-neutral-800"
          >
            {t('auth.logout')}
          </button>
        </Stack>
        <main className="h-full min-w-0" aria-label="Контент">
          <Outlet />
        </main>
      </Sidebar>
    </div>
  );
}