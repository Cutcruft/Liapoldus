import { useEffect } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { Stack } from '@liapoldus/ui-kit';
import type { Site } from '../runtime';
import { useAdmin } from './admin-context';
import { useOperation } from './use-operation';
import { siteTabKey, updateTabLabel } from './tabs-store';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';

/**
 * Экран вкладки сайта (слайс R1): пока заглушка. Режимы
 * «Обслуживание» / «Редактор» и рабочая область появляются на R2.
 * Обновляет label вкладки после загрузки имени сайта.
 */
export function SiteTabPlaceholder() {
  const { siteId = '' } = useParams();
  const { t } = useAdmin();
  const navigate = useNavigate();

  const site = useOperation<Site | null>('getSite', { siteId }, (d) =>
    typeof d === 'object' && d !== null ? (d as Site) : null,
  );

  useEffect(() => {
    if (site.state.status === 'success' && site.state.data) {
      updateTabLabel(siteTabKey(siteId), site.state.data.name);
    }
  }, [site.state.status, siteId]);

  return (
    <Stack pad={8} gap={4}>
      {site.state.status === 'loading' ? (
        <Skeleton className="h-8 w-56" />
      ) : site.state.status === 'error' ? (
        <Stack gap={2}>
          <h1 className="text-xl font-medium">{t('site.notFound')}</h1>
          <p className="text-sm text-red-600">
            {t('common.error')}: {site.state.detail}
          </p>
          <Button type="button" variant="outline" onClick={() => navigate('/')} className="w-fit">
            {t('common.reload')}
          </Button>
        </Stack>
      ) : site.state.data ? (
        <>
          <h1 className="text-xl font-medium">{site.state.data.name}</h1>
          <div className="rounded-lg border border-dashed border-neutral-300 bg-neutral-50 p-10 text-center">
            <h2 className="text-base font-medium text-neutral-800">{t('tab.placeholder.title')}</h2>
            <p className="mx-auto mt-1 max-w-lg text-sm text-neutral-500">{t('tab.placeholder.text')}</p>
            <Button type="button" variant="outline" onClick={() => navigate('/')} className="mt-4">
              {t('home.sites')}
            </Button>
          </div>
        </>
      ) : null}
    </Stack>
  );
}