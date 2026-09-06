import type { RouteObject } from 'react-router-dom';
import { AppShell } from './AppShell';
import { Placeholder } from './Placeholder';
import { SitesPage } from './pages/SitesPage';
import { SiteHomePage } from './pages/SiteHomePage';
import { SitePagesPage } from './pages/SitePagesPage';
import { SiteRoutesPage } from './pages/SiteRoutesPage';
import { SiteContentsPage } from './pages/SiteContentsPage';
import { ContentEditorPage } from './pages/ContentEditorPage';
import { EditorPage } from './editor/EditorPage';

export const appRoutes: RouteObject[] = [
  {
    path: '/',
    element: <AppShell />,
    children: [
      { index: true, element: <Placeholder titleKey="nav.dashboard" /> },
      { path: 'sites', element: <SitesPage /> },
      { path: 'sites/:siteId', element: <SiteHomePage /> },
      { path: 'sites/:siteId/pages', element: <SitePagesPage /> },
      { path: 'sites/:siteId/pages/:pageId', element: <EditorPage /> },
      { path: 'sites/:siteId/contents', element: <SiteContentsPage /> },
      { path: 'sites/:siteId/contents/:contentId', element: <ContentEditorPage /> },
      { path: 'sites/:siteId/assets', element: <Placeholder titleKey="nav.assets" scope="—" /> },
      { path: 'sites/:siteId/routes', element: <SiteRoutesPage /> },
      { path: 'sites/:siteId/forms', element: <Placeholder titleKey="nav.forms" scope="—" /> },
      { path: 'sites/:siteId/snapshots', element: <Placeholder titleKey="nav.snapshots" scope="—" /> },
      { path: 'settings', element: <Placeholder titleKey="nav.settings" /> },
    ],
  },
];