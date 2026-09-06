import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Inline, Stack } from '@liapoldus/ui-kit';
import { type Dashboard, type DashboardSite } from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { EntityTable } from '../components/EntityTable';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';

const INPUT_CLASS =
  'rounded border border-neutral-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none';

function shortSha(sha: string | undefined): string {
  if (!sha) return '—';
  return sha.length > 8 ? sha.slice(0, 8) : sha;
}

function runtimeBadge(status: string, t: (key: string) => string) {
  const label =
    status === 'ok'
      ? t('dashboard.runtime.ok')
      : status === 'degraded'
        ? t('dashboard.runtime.degraded')
        : t('dashboard.runtime.noBuilds');
  const cls =
    status === 'ok'
      ? 'bg-green-100 text-green-700 border-green-200'
      : status === 'degraded'
        ? 'bg-amber-100 text-amber-700 border-amber-200'
        : 'bg-neutral-100 text-neutral-600 border-neutral-200';
  return (
    <Inline gap={2} align="center">
      <span className="text-xs text-neutral-500">{t('dashboard.runtime')}:</span>
      <Badge variant="outline" className={cls}>
        {label}
      </Badge>
    </Inline>
  );
}

function gitChips(site: DashboardSite, t: (key: string) => string) {
  const git = site.git;
  if (!git || (!git.dev.existing && !git.main.existing)) {
    return <span className="text-xs text-neutral-400">{t('dashboard.git.none')}</span>;
  }
  return (
    <Inline gap={2}>
      {git.dev.existing && (
        <Badge variant="outline" className="bg-blue-100 text-blue-700 border-blue-200">
          {t('dashboard.git.dev')} {shortSha(git.dev.sha)}
        </Badge>
      )}
      {git.main.existing && (
        <Badge variant="outline" className="bg-purple-100 text-purple-700 border-purple-200">
          {t('dashboard.git.main')} {shortSha(git.main.sha)}
        </Badge>
      )}
      {git.dev.existing && git.dirty && (
        <Badge variant="outline" className="bg-amber-100 text-amber-700 border-amber-200">
          {t('dashboard.git.dirty')}
        </Badge>
      )}
    </Inline>
  );
}

function StatCard({
  label,
  value,
  to,
}: {
  label: string;
  value: number;
  to?: string;
}) {
  const navigate = useNavigate();
  return (
    <button
      type="button"
      onClick={() => to && navigate(to)}
      className={`rounded-lg border border-neutral-200 bg-white p-4 text-left shadow-sm ${to ? 'cursor-pointer transition hover:border-blue-400 hover:shadow' : 'cursor-default'}`}
      disabled={!to}
    >
      <div className="text-2xl font-semibold text-neutral-900">{value}</div>
      <div className="text-xs text-neutral-500">{label}</div>
    </button>
  );
}

export function DashboardPage() {
  const { t } = useAdmin();
  const navigate = useNavigate();
  const [selectedSite, setSelectedSite] = useState('');

  const dashboard = useOperation<Dashboard>('getDashboard', {}, (d) =>
    typeof d === 'object' && d !== null
      ? (d as Dashboard)
      : { siteCount: 0, sites: [], recentBuilds: [], recentSnapshots: [], runtimeStatus: 'no-builds' },
  );

  const goSite = () => {
    if (selectedSite) navigate(`/sites/${selectedSite}`);
  };

  const data = dashboard.state.status === 'success' ? dashboard.state.data : undefined;

  return (
    <Stack pad={8} gap={4}>
      <Inline justify="between" align="center">
        <h1 className="text-xl font-medium">{t('nav.dashboard')}</h1>
        <button type="button" onClick={dashboard.reload} className={INPUT_CLASS}>
          {t('common.reload')}
        </button>
      </Inline>

      {dashboard.state.status === 'loading' && (
        <Stack gap={3}>
          <Skeleton className="h-12 w-48" />
          <Skeleton className="h-10 w-72" />
          <Skeleton className="h-32 w-full" />
        </Stack>
      )}

      {dashboard.state.status === 'error' && (
        <Stack gap={2}>
          <p className="text-sm text-red-600">
            {t('common.error')}: {dashboard.state.detail}
          </p>
          <Button type="button" variant="outline" onClick={dashboard.reload} className="w-fit">
            {t('common.reload')}
          </Button>
        </Stack>
      )}

      {data && (
        <Stack gap={4}>
          <Inline gap={3} align="center">
            {runtimeBadge(data.runtimeStatus, (k) => t(k as never))}
          </Inline>

          <Inline gap={3}>
            <StatCard label={t('dashboard.count.sites', { n: data.siteCount })} value={data.siteCount} to={data.siteCount > 0 ? '/sites' : undefined} />
            <StatCard label={t('dashboard.count.builds', { n: data.recentBuilds.length })} value={data.recentBuilds.length} />
            <StatCard label={t('dashboard.count.snapshots', { n: data.recentSnapshots.length })} value={data.recentSnapshots.length} />
          </Inline>

          {data.siteCount === 0 ? (
            <div className="rounded-lg border border-dashed border-neutral-300 bg-neutral-50 p-8 text-center">
              <h2 className="text-base font-medium text-neutral-800">{t('dashboard.empty.title')}</h2>
              <p className="mt-1 text-sm text-neutral-500">{t('dashboard.empty.text')}</p>
              <Button asChild className="mt-4">
                <a href="/sites">{t('dashboard.empty.cta')}</a>
              </Button>
            </div>
          ) : (
            <>
              <Inline gap={3} align="center">
                <span className="text-sm font-medium text-neutral-700">{t('dashboard.quick')}</span>
                <Button asChild variant="outline" size="sm">
                  <a href="/sites">{t('dashboard.empty.cta')}</a>
                </Button>
                <Inline gap={2}>
                  <select
                    className={INPUT_CLASS}
                    value={selectedSite}
                    onChange={(e) => setSelectedSite(e.target.value)}
                    aria-label={t('dashboard.selectSite')}
                  >
                    <option value="">{t('dashboard.selectSite')}</option>
                    {data.sites.map((s) => (
                      <option key={s.siteId} value={s.siteId}>
                        {s.name}
                      </option>
                    ))}
                  </select>
                  <Button type="button" variant="outline" size="sm" onClick={goSite} disabled={!selectedSite}>
                    Открыть
                  </Button>
                </Inline>
              </Inline>

              <section>
                <h2 className="mb-2 text-base font-medium text-neutral-700">{t('dashboard.sites')}</h2>
                {data.sites.length === 0 ? (
                  <p className="text-sm text-neutral-400">{t('dashboard.sites.none')}</p>
                ) : (
                  <EntityTable<DashboardSite>
                    gridClass="grid-cols-[minmax(0,3fr)_minmax(0,2fr)_minmax(0,1fr)_minmax(0,3fr)_minmax(0,10rem)]"
                    getKey={(s) => s.siteId}
                    columns={[
                      {
                        key: 'name',
                        label: t('site.name'),
                        render: (s) => (
                          <button
                            type="button"
                            className="cursor-pointer font-medium text-blue-700 hover:underline"
                            onClick={() => navigate(`/sites/${s.siteId}`)}
                          >
                            {s.name}
                          </button>
                        ),
                      },
                      { key: 'slug', label: t('site.slug'), render: (s) => <code className="text-xs">{s.slug}</code> },
                      {
                        key: 'locale',
                        label: t('site.locale'),
                        render: (s) => s.defaultLocale || '—',
                      },
                      {
                        key: 'hosts',
                        label: t('site.hosts'),
                        render: (s) => (s.hosts.length ? s.hosts.join(', ') : '—'),
                      },
                      { key: 'git', label: t('dashboard.git.dev'), render: (s) => gitChips(s, t) },
                    ]}
                    rows={data.sites}
                  />
                )}
              </section>

              <section>
                <h2 className="mb-2 text-base font-medium text-neutral-700">{t('dashboard.recentBuilds')}</h2>
                {data.recentBuilds.length === 0 ? (
                  <p className="text-sm text-neutral-400">{t('dashboard.recent.none')}</p>
                ) : (
                  <EntityTable
                    gridClass="grid-cols-[minmax(0,2fr)_minmax(0,3fr)_minmax(0,7rem)_minmax(0,6rem)_minmax(0,11rem)]"
                    getKey={(b) => b.id}
                    columns={[
                      { key: 'site', label: t('dashboard.sites'), render: (b) => b.siteName },
                      { key: 'id', label: 'ID', render: (b) => <code className="text-xs">{b.id}</code> },
                      {
                        key: 'env',
                        label: 'Env',
                        render: (b) => <code className="text-xs">{b.environment}</code>,
                      },
                      {
                        key: 'status',
                        label: t('builds.statusCol'),
                        render: (b) => (
                          <Badge variant="outline" className={b.status === 'ready' ? 'bg-green-100 text-green-700 border-green-200' : b.status === 'failed' ? 'bg-red-100 text-red-700 border-red-200' : 'bg-neutral-100 text-neutral-600 border-neutral-200'}>
                            {b.status}
                          </Badge>
                        ),
                      },
                      {
                        key: 'createdAt',
                        label: t('git.date'),
                        render: (b) => (
                          <span className="text-xs text-neutral-500">
                            {new Date(b.createdAt).toLocaleString('ru-RU')}
                          </span>
                        ),
                      },
                    ]}
                    rows={data.recentBuilds}
                  />
                )}
              </section>

              <section>
                <h2 className="mb-2 text-base font-medium text-neutral-700">{t('dashboard.recentSnapshots')}</h2>
                {data.recentSnapshots.length === 0 ? (
                  <p className="text-sm text-neutral-400">{t('dashboard.recent.none')}</p>
                ) : (
                  <EntityTable
                    gridClass="grid-cols-[minmax(0,3fr)_minmax(0,2fr)_minmax(0,9rem)_minmax(0,11rem)]"
                    getKey={(s) => s.id}
                    columns={[
                      {
                        key: 'site',
                        label: t('dashboard.sites'),
                        render: (s) => <span className="font-medium">{s.siteName}</span>,
                      },
                      { key: 'name', label: t('dashboard.recentSnapshots'), render: (s) => s.name },
                      {
                        key: 'gitSha',
                        label: t('git.sha'),
                        render: (s) => (s.gitSha ? <code className="text-xs">{shortSha(s.gitSha)}</code> : '—'),
                      },
                      {
                        key: 'createdAt',
                        label: t('git.date'),
                        render: (s) => (
                          <span className="text-xs text-neutral-500">
                            {new Date(s.createdAt).toLocaleString('ru-RU')}
                          </span>
                        ),
                      },
                    ]}
                    rows={data.recentSnapshots}
                  />
                )}
              </section>
            </>
          )}
        </Stack>
      )}
    </Stack>
  );
}