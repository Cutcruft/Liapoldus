import type { RouteObject } from 'react-router-dom';
import { AppShell } from './AppShell';
import { Placeholder } from './Placeholder';
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
      { path: 'sites/:siteId/assets', element: <SiteAssetsPage /> },
      { path: 'sites/:siteId/routes', element: <SiteRoutesPage /> },
      { path: 'sites/:siteId/routes/:routeId', element: <RouteEditorPage /> },
      { path: 'sites/:siteId/forms', element: <SiteFormsPage /> },
      { path: 'sites/:siteId/forms/:formId', element: <FormEditorPage /> },
      { path: 'sites/:siteId/builds', element: <SiteBuildsPage /> },
      { path: 'settings', element: <Placeholder titleKey="nav.settings" /> },
    ],
  },
];