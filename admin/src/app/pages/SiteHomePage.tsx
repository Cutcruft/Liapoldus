import { NavLink, useNavigate, useParams } from 'react-router-dom';
import { Inline, Stack } from '@liapoldus/ui-kit';
import { runOperation, type Site } from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { ConfirmButton } from '../components/ConfirmButton';

const SECTIONS: Array<{
  to: string;
  labelKey: 'nav.pages' | 'nav.contents' | 'nav.assets' | 'nav.routes' | 'nav.forms' | 'nav.builds' | 'nav.git';
}> = [
  { to: 'pages', labelKey: 'nav.pages' },
  { to: 'contents', labelKey: 'nav.contents' },
  { to: 'assets', labelKey: 'nav.assets' },
  { to: 'routes', labelKey: 'nav.routes' },
  { to: 'forms', labelKey: 'nav.forms' },
  { to: 'builds', labelKey: 'nav.builds' },
  { to: 'git', labelKey: 'nav.git' },
];

export function SiteHomePage() {
  const { siteId = '' } = useParams();
  const { api, t } = useAdmin();
  const navigate = useNavigate();
  const site = useOperation<Site | null>('getSite', { siteId }, (d: unknown) =>
    typeof d === 'object' && d !== null && 'id' in d ? (d as Site) : null,
  );

  const remove = async () => {
    const res = await runOperation(api, 'deleteSite', { siteId }, t);
    if (res.ok) navigate('/sites');
  };

  return (
    <Stack pad={8} gap={4}>
      {site.state.status === 'loading' && <p className="text-sm text-neutral-400">{t('common.loading')}</p>}

      {site.state.status === 'error' && (
        <Stack gap={2}>
          <p className="text-sm text-red-600">
            {t('common.error')}: {site.state.detail}
          </p>
          <button type="button" onClick={site.reload} className="w-fit rounded border border-neutral-300 px-3 py-1 text-sm">
            {t('common.reload')}
          </button>
        </Stack>
      )}

      {site.state.status === 'success' &&
        (site.state.data ? (
          <>
            <Inline justify="between" align="center">
              <div>
                <h1 className="text-xl font-medium">{site.state.data.name}</h1>
                <p className="text-sm text-neutral-500">
                  <code>{site.state.data.slug}</code> · локали: {site.state.data.defaultLocale} · хосты:{' '}
                  {site.state.data.hosts.join(', ') || '—'}
                </p>
              </div>
              <ConfirmButton label={t('site.delete.confirm')} onConfirm={remove} />
            </Inline>

            <nav aria-label="Разделы сайта">
              <Stack gap={2}>
                <h2 className="text-sm font-medium text-neutral-500">{t('site.sections')}</h2>
                <Inline gap={2}>
                  {SECTIONS.map((s) => (
                    <NavLink
                      key={s.to}
                      to={s.to}
                      className={({ isActive }) =>
                        `rounded border px-3 py-1.5 text-sm ${
                          isActive ? 'border-blue-300 bg-blue-50 text-blue-700' : 'border-neutral-300 text-neutral-600'
                        }`
                      }
                    >
                      {t(s.labelKey)}
                    </NavLink>
                  ))}
                </Inline>
              </Stack>
            </nav>
          </>
        ) : (
          <p className="text-sm text-neutral-400">{t('site.notFound')}</p>
        ))}
    </Stack>
  );
}