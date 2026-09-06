import type { RouteObject } from 'react-router-dom';
import { useParams } from 'react-router-dom';
import { AppShell } from './AppShell';
import { Placeholder } from './Placeholder';
import { SitesPage } from './pages/SitesPage';
import { SiteHomePage } from './pages/SiteHomePage';
import { SitePagesPage } from './pages/SitePagesPage';
import { SiteRoutesPage } from './pages/SiteRoutesPage';
import { Stack } from '@liapoldus/ui-kit';
import { useAdmin } from './admin-context';

function EditorPlaceholder() {
  const { pageId = '' } = useParams();
  const { t } = useAdmin();
  return (
    <Stack pad={8} gap={4}>
      <h1 className="text-xl font-medium">
        {t('page.title')} · <code className="text-sm">{pageId}</code>
      </h1>
      <p className="text-sm text-neutral-400">{t('page.editor.placeholder')}</p>
    </Stack>
  );
}

export const appRoutes: RouteObject[] = [
  {
    path: '/',
    element: <AppShell />,
    children: [
      { index: true, element: <Placeholder titleKey="nav.dashboard" /> },
      { path: 'sites', element: <SitesPage /> },
      { path: 'sites/:siteId', element: <SiteHomePage /> },
      { path: 'sites/:siteId/pages', element: <SitePagesPage /> },
      { path: 'sites/:siteId/pages/:pageId', element: <EditorPlaceholder /> },
      { path: 'sites/:siteId/assets', element: <Placeholder titleKey="nav.assets" scope="—" /> },
      { path: 'sites/:siteId/routes', element: <SiteRoutesPage /> },
      { path: 'sites/:siteId/forms', element: <Placeholder titleKey="nav.forms" scope="—" /> },
      { path: 'sites/:siteId/snapshots', element: <Placeholder titleKey="nav.snapshots" scope="—" /> },
      { path: 'settings', element: <Placeholder titleKey="nav.settings" /> },
    ],
  },
];