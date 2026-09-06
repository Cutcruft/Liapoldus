import type { RouteObject } from 'react-router-dom';
import { AppShell } from './AppShell';
import { RequireAuth } from './RequireAuth';
import { SettingsPage } from './pages/SettingsPage';
import { LoginPage } from './pages/LoginPage';
import { HomePage } from './pages/HomePage';
import { SiteTabPlaceholder } from './SiteTabPlaceholder';
// Старые site-страницы (перенесём на R3–R8): в проде маршруты удалены сразу,
// в dev остаются для регрессионных тестов и ручной проверки (R0).
import { SitesPage } from './pages/SitesPage';
import { SiteHomePage } from './pages/SiteHomePage';
import { SitePagesPage } from './pages/SitePagesPage';
import { SiteRoutesPage } from './pages/SiteRoutesPage';
import { RouteEditorPage } from './pages/RouteEditorPage';
import { SiteContentsPage } from './pages/SiteContentsPage';
import { ContentEditorPage } from './pages/ContentEditorPage';
import { SiteAssetsPage } from './pages/SiteAssetsPage';
import { SiteFormsPage } from './pages/SiteFormsPage';
import { FormEditorPage } from './pages/FormEditorPage';
import { SiteBuildsPage } from './pages/SiteBuildsPage';
import { SiteCommitsPage } from './pages/SiteCommitsPage';
import { SiteTokensPage } from './pages/SiteTokensPage';
import { SiteDepsPage } from './pages/SiteDepsPage';
import { SiteAllowlistPage } from './pages/SiteAllowlistPage';
import { EditorPage } from './editor/EditorPage';

/** Новые маршруты каркаса (слайс R1). Старые site-роуты — только в dev. */
const CORE_ROUTES: RouteObject[] = [
  { index: true, element: <HomePage /> },
  { path: 'sites/:siteId', element: <SiteTabPlaceholder /> },
  { path: 'settings', element: <SettingsPage /> },
];

const LEGACY_ROUTES: RouteObject[] = [
  { path: 'sites', element: <SitesPage /> },
  { path: 'sites/:siteId', element: <SiteHomePage /> },
  { path: 'sites/:siteId/pages', element: <SitePagesPage /> },
  { path: 'sites/:siteId/pages/:pageId', element: <EditorPage /> },
  { path: 'sites/:siteId/contents', element: <SiteContentsPage /> },
  { path: 'sites/:siteId/contents/:contentId', element: <ContentEditorPage /> },
  { path: 'sites/:siteId/assets', element: <SiteAssetsPage /> },
  { path: 'sites/:siteId/routes', element: <SiteRoutesPage /> },
  { path: 'sites/:siteId/routes/:routeId', element: <RouteEditorPage /> },
  { path: 'sites/:siteId/forms', element: <SiteFormsPage /> },
  { path: 'sites/:siteId/forms/:formId', element: <FormEditorPage /> },
  { path: 'sites/:siteId/builds', element: <SiteBuildsPage /> },
  { path: 'sites/:siteId/git', element: <SiteCommitsPage /> },
  { path: 'sites/:siteId/tokens', element: <SiteTokensPage /> },
  { path: 'sites/:siteId/deps', element: <SiteDepsPage /> },
  { path: 'sites/:siteId/allowlist', element: <SiteAllowlistPage /> },
];

export const appRoutes: RouteObject[] = [
  { path: 'login', element: <LoginPage /> },
  {
    path: '/',
    element: (
      <RequireAuth>
        <AppShell />
      </RequireAuth>
    ),
    children: import.meta.env.DEV ? [...CORE_ROUTES, ...LEGACY_ROUTES] : CORE_ROUTES,
  },
];