import { useEffect } from 'react';
import { useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { Stack } from '@liapoldus/ui-kit';
import type { Site } from '../runtime';
import { useAdmin } from './admin-context';
import { useOperation } from './use-operation';
import { siteTabKey, updateTabLabel } from './tabs-store';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import {
  ClipboardList,
  Component as ComponentIcon,
  History,
  Image as ImageIcon,
  Inbox,
  LayoutTemplate,
  Palette,
  Server,
  type LucideIcon,
} from 'lucide-react';
import {
  currentMaintenanceSection,
  DEFAULT_EDITOR_SECTION,
  isEditorSection,
  isMaintenanceSection,
  rememberedEditorSection,
  rememberEditorSection,
  selectMaintenanceSection,
  type EditorSectionKey,
  type MaintenanceSectionKey,
} from './site-tab-store';
import { ContentSection } from './content/ContentSection';
import { FormsSection } from './forms/FormsSection';
import { MediaSection } from './media/MediaSection';
import { ComponentsSection } from './components/ComponentsSection';
import { InfraSection } from './infra/InfraSection';
import { PagesSection } from './pages/PagesSection';
import { SnapshotSection } from './states/SnapshotSection';

/** Режим вкладки сайта (из URL `?view=`). */
export type SiteTabMode = 'maintenance' | 'editor';

const EDITOR_SECTIONS: ReadonlyArray<{ key: EditorSectionKey; icon: LucideIcon }> = [
  { key: 'components', icon: ComponentIcon },
  { key: 'infra', icon: Server },
  { key: 'pages', icon: LayoutTemplate },
  { key: 'states', icon: History },
  { key: 'base', icon: Palette },
];

const MAINTENANCE_SECTIONS: ReadonlyArray<{ key: MaintenanceSectionKey; icon: LucideIcon }> = [
  { key: 'content', icon: ClipboardList },
  { key: 'forms', icon: Inbox },
  { key: 'media', icon: ImageIcon },
];

function sectionNavLabel(key: string): string {
  return `tab.section.${key}`;
}
function maintenanceNavLabel(key: string): string {
  return `tab.maintenance.${key}`;
}

function SectionPlaceholder({ title, description }: { title: string; description: string }) {
  const { t } = useAdmin();
  const navigate = useNavigate();
  return (
    <div className="flex h-full items-center justify-center">
      <div className="rounded-lg border border-dashed border-neutral-300 bg-neutral-50 p-8 text-center">
        <h2 className="text-base font-medium text-neutral-800">{title}</h2>
        <p className="mx-auto mt-1 max-w-md text-sm text-neutral-500">{description}</p>
        <Button type="button" variant="outline" onClick={() => navigate('/')} className="mt-4">
          {t('home.sites')}
        </Button>
      </div>
    </div>
  );
}

/**
 * Вкладка сайта (слайсы R2–R3). Шапка с сегмент-тумблером
 * [Обслуживание | Редактор] (`?view=editor`), слева вертикальная навигация
 * подразделов, справа рабочая область. Подразделы обслуживания кодируются в
 * URL `?mode=content|forms|media` (R3; fallback — SPA-память), редактор
 * контента — `?mode=content&contentId=...`. Возврат в «Редактор» открывает
 * последний выбранный раздел (SPA-память).
 */
export function SiteTab() {
  const { siteId = '' } = useParams();
  const { t } = useAdmin();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();

  const view: SiteTabMode = searchParams.get('view') === 'editor' ? 'editor' : 'maintenance';
  const sectionParam = searchParams.get('section');
  const modeParam = searchParams.get('mode');
  const activeEditorSection = isEditorSection(sectionParam) ? sectionParam : rememberedEditorSection(siteId);
  const maintenanceSection = isMaintenanceSection(modeParam)
    ? modeParam
    : currentMaintenanceSection(siteId);

  const site = useOperation<Site | null>('getSite', { siteId }, (d) =>
    typeof d === 'object' && d !== null ? (d as Site) : null,
  );

  useEffect(() => {
    if (site.state.status === 'success' && site.state.data) {
      updateTabLabel(siteTabKey(siteId), site.state.data.name);
    }
  }, [site.state.status, siteId]);

  const switchView = (next: SiteTabMode) => {
    const nextParams = new URLSearchParams(searchParams);
    if (next === 'editor') {
      nextParams.set('view', 'editor');
      nextParams.set('section', activeEditorSection);
      nextParams.delete('contentId');
    } else {
      if (isEditorSection(sectionParam)) {
        rememberEditorSection(siteId, sectionParam);
      }
      nextParams.delete('view');
      nextParams.delete('section');
      nextParams.set('mode', maintenanceSection);
    }
    setSearchParams(nextParams);
  };

  const pickEditorSection = (key: EditorSectionKey) => {
    const nextParams = new URLSearchParams(searchParams);
    nextParams.set('view', 'editor');
    nextParams.set('section', key);
    setSearchParams(nextParams);
  };

  const pickMaintenanceSection = (key: MaintenanceSectionKey) => {
    if (isEditorSection(sectionParam)) {
      rememberEditorSection(siteId, sectionParam);
    }
    selectMaintenanceSection(siteId, key);
    const nextParams = new URLSearchParams(searchParams);
    nextParams.delete('view');
    nextParams.delete('section');
    nextParams.set('mode', key);
    nextParams.delete('contentId');
    nextParams.delete('formId');
    setSearchParams(nextParams);
  };

  return (
    <Stack pad={4} gap={4} className="h-full min-h-0">
      {site.state.status === 'loading' ? (
        <Skeleton className="h-6 w-52" />
      ) : site.state.status === 'error' ? (
        <Stack gap={2}>
          <h1 className="text-lg font-medium text-red-600">
            {t('common.error')}: {site.state.detail}
          </h1>
          <Button type="button" variant="outline" onClick={() => navigate('/')} className="w-fit">
            {t('home.sites')}
          </Button>
        </Stack>
      ) : site.state.data ? (
        <>
          <div className="flex items-center justify-between gap-3">
            <h1 className="truncate text-lg font-semibold text-neutral-900">{site.state.data.name}</h1>
            <div
              role="tablist"
              aria-label={t('view.mode')}
              className="flex shrink-0 items-center rounded-lg border border-neutral-200 bg-neutral-100 p-0.5"
            >
              <button
                type="button"
                role="tab"
                aria-selected={view === 'maintenance'}
                onClick={() => switchView('maintenance')}
                className={`rounded-md px-3 py-1 text-sm ${
                  view === 'maintenance'
                    ? 'bg-white font-medium text-neutral-900 shadow-sm'
                    : 'text-neutral-600 hover:text-neutral-800'
                }`}
              >
                {t('view.maintenance')}
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={view === 'editor'}
                onClick={() => switchView('editor')}
                className={`rounded-md px-3 py-1 text-sm ${
                  view === 'editor'
                    ? 'bg-white font-medium text-neutral-900 shadow-sm'
                    : 'text-neutral-600 hover:text-neutral-800'
                }`}
              >
                {t('view.editor')}
              </button>
            </div>
          </div>

          {view === 'maintenance' ? (
            <div className="flex h-full min-h-0">
              <nav
                aria-label={t('view.maintenance')}
                className="flex w-44 shrink-0 flex-col gap-1 border-r border-neutral-200 pr-3"
              >
                {MAINTENANCE_SECTIONS.map((s) => {
                  const Icon = s.icon;
                  return (
                    <button
                      key={s.key}
                      type="button"
                      aria-pressed={maintenanceSection === s.key}
                      onClick={() => pickMaintenanceSection(s.key)}
                      className={`flex items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm ${
                        maintenanceSection === s.key
                          ? 'bg-blue-50 font-medium text-blue-800'
                          : 'text-neutral-600 hover:bg-neutral-100'
                      }`}
                    >
                      <Icon aria-hidden="true" className="h-4 w-4 shrink-0 text-neutral-400" />
                      <span className="truncate">{t(maintenanceNavLabel(s.key))}</span>
                    </button>
                  );
                })}
              </nav>
              <section className="min-w-0 flex-1 pl-4" aria-label={t(maintenanceNavLabel(maintenanceSection))}>
                {maintenanceSection === 'content' ? (
                  <ContentSection />
                ) : maintenanceSection === 'forms' ? (
                  <FormsSection />
                ) : maintenanceSection === 'media' ? (
                  <MediaSection />
                ) : (
                  <SectionPlaceholder
                    title={t(maintenanceNavLabel(maintenanceSection))}
                    description={t(maintenanceNavLabel(maintenanceSection) + '.description')}
                  />
                )}
              </section>
            </div>
          ) : (
            <div className="flex h-full min-h-0">
              <nav
                aria-label={t('view.editor')}
                className="flex w-44 shrink-0 flex-col gap-1 border-r border-neutral-200 pr-3"
              >
                {EDITOR_SECTIONS.map((s) => {
                  const Icon = s.icon;
                  return (
                    <button
                      key={s.key}
                      type="button"
                      aria-pressed={activeEditorSection === s.key}
                      onClick={() => pickEditorSection(s.key)}
                      className={`flex items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm ${
                        activeEditorSection === s.key
                          ? 'bg-blue-50 font-medium text-blue-800'
                          : 'text-neutral-600 hover:bg-neutral-100'
                      }`}
                    >
                      <Icon aria-hidden="true" className="h-4 w-4 shrink-0 text-neutral-400" />
                      <span className="truncate">{t(sectionNavLabel(s.key))}</span>
                    </button>
                  );
                })}
              </nav>
              <section className="min-w-0 flex-1 pl-4" aria-label={t(sectionNavLabel(activeEditorSection))}>
                {activeEditorSection === 'components' ? (
                  <ComponentsSection />
                ) : activeEditorSection === 'infra' ? (
                  <InfraSection />
                ) : activeEditorSection === 'pages' ? (
                  <PagesSection />
                ) : activeEditorSection === 'states' ? (
                  <SnapshotSection />
                ) : (
                  <SectionPlaceholder
                    title={t(sectionNavLabel(activeEditorSection))}
                    description={t(sectionNavLabel(activeEditorSection) + '.description')}
                  />
                )}
              </section>
            </div>
          )}
        </>
      ) : null}
    </Stack>
  );
}