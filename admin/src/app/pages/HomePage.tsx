import { useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { Inline, Stack } from '@liapoldus/ui-kit';
import { runOperation, type Dashboard, type DashboardSite, type Site } from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { Field } from '../components/Field';
import { openOrFocusSite, openOrFocusSettings } from '../tabs-store';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Settings2, Plus } from 'lucide-react';

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
    <Inline gap={2} wrap>
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

function SiteCard({ site, onOpen }: { site: DashboardSite; onOpen: (site: DashboardSite) => void }) {
  const { t } = useAdmin();
  return (
    <button
      type="button"
      onClick={() => onOpen(site)}
      className="group flex min-h-36 flex-col gap-3 rounded-xl border border-neutral-200 bg-white p-5 text-left shadow-sm transition hover:border-blue-400 hover:shadow"
    >
      <Inline justify="between" gap={3} align="start" className="w-full">
        <span className="truncate text-base font-semibold text-neutral-900 group-hover:text-blue-700">
          {site.name}
        </span>
        <span className="shrink-0 rounded bg-neutral-100 px-1.5 py-0.5 text-xs text-neutral-500">
          {site.defaultLocale || '—'}
        </span>
      </Inline>
      <code className="text-xs text-neutral-400">{site.slug}</code>
      <span className="truncate text-xs text-neutral-500">
        {site.hosts.length ? site.hosts.join(', ') : t('site.hosts') + ': —'}
      </span>
      <div className="mt-auto">{gitChips(site, t)}</div>
    </button>
  );
}

function CreateSiteCard({
  onCreated,
}: {
  onCreated: (site: Site) => void;
}) {
  const { api, t } = useAdmin();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [locale, setLocale] = useState('ru');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!name.trim() || !slug.trim()) {
      setError(t('common.required'));
      return;
    }
    setSubmitting(true);
    setError('');
    const res = await runOperation(api, 'createSite', { name, slug, defaultLocale: locale }, t);
    setSubmitting(false);
    if (!res.ok) {
      setError(res.detail);
      return;
    }
    const site = res.data as Site;
    setName('');
    setSlug('');
    setOpen(false);
    onCreated(site);
  };

  if (!open) {
    return (
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="flex min-h-36 flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-neutral-300 bg-neutral-50 text-neutral-500 transition hover:border-blue-400 hover:text-blue-700"
      >
        <Plus className="h-6 w-6" />
        <span className="text-sm font-medium">{t('site.new')}</span>
      </button>
    );
  }

  return (
    <form onSubmit={submit} className="flex min-h-36 flex-col gap-3 rounded-xl border border-blue-300 bg-white p-5 shadow-sm">
      <span className="text-sm font-medium text-neutral-800">{t('site.new')}</span>
      <Stack gap={2}>
        <Field label={t('site.name')} required>
          <Input value={name} onChange={(e) => setName(e.target.value)} />
        </Field>
        <Field label={t('site.slug')} required>
          <Input value={slug} onChange={(e) => setSlug(e.target.value)} />
        </Field>
        <Field label={t('site.locale')}>
          <select
            className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
            value={locale}
            onChange={(e) => setLocale(e.target.value)}
          >
            <option value="ru">ru</option>
            <option value="en">en</option>
          </select>
        </Field>
      </Stack>
      <Inline gap={2}>
        <Button type="submit" disabled={submitting}>
          {t('common.create')}
        </Button>
        <Button type="button" variant="outline" onClick={() => setOpen(false)}>
          {t('common.cancel')}
        </Button>
      </Inline>
      {error && <p className="text-sm text-destructive">{error}</p>}
    </form>
  );
}

/**
 * Главная (слайс R1): карточки сайтов + «создать сайт» + ссылка на
 * системные настройки. Заменяет «Обзор» и «Сайты» старой навигации.
 */
export function HomePage() {
  const { t } = useAdmin();
  const navigate = useNavigate();

  const dashboard = useOperation<Dashboard>('getDashboard', {}, (d) =>
    typeof d === 'object' && d !== null
      ? (d as Dashboard)
      : { siteCount: 0, sites: [], recentBuilds: [], recentSnapshots: [], runtimeStatus: 'no-builds' },
  );

  const data = dashboard.state.status === 'success' ? dashboard.state.data : undefined;
  const sites: DashboardSite[] = data?.sites ?? [];

  const openSite = (site: DashboardSite) => openOrFocusSite(site.siteId, site.name, navigate);

  return (
    <Stack pad={8} gap={5} className="max-w-6xl">
      <Inline justify="between" align="center" wrap gap={3}>
        <div>
          <h1 className="text-xl font-medium">{t('home.sites')}</h1>
          <p className="mt-0.5 text-sm text-neutral-500">
            {t('dashboard.count.sites', { n: dashboard.state.status === 'success' ? dashboard.state.data.siteCount : 0 })}
          </p>
        </div>
        <Inline gap={2} align="center" wrap>
          {data && runtimeBadge(data.runtimeStatus, (k) => t(k as never))}
          <Button type="button" variant="outline" onClick={dashboard.reload} className="h-9">
            {t('common.reload')}
          </Button>
          <Button
            type="button"
            variant="outline"
            className="h-9"
            onClick={() => openOrFocusSettings(navigate)}
            aria-label={t('home.settings')}
          >
            <Settings2 className="h-4 w-4" />
            <span className="ml-1">{t('home.settings')}</span>
          </Button>
        </Inline>
      </Inline>

      {dashboard.state.status === 'loading' && (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {[0, 1, 2].map((i) => (
            <div key={i} className="h-36 animate-pulse rounded-xl border border-neutral-200 bg-neutral-100" />
          ))}
        </div>
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
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <CreateSiteCard
            onCreated={(site) => openOrFocusSite(site.id, site.name, navigate)}
          />
          {sites.map((site) => (
            <SiteCard key={site.siteId} site={site} onOpen={openSite} />
          ))}
          {sites.length === 0 && (
            <div className="col-span-full text-center text-sm text-neutral-400">
              {t('site.none')}
            </div>
          )}
        </div>
      )}
    </Stack>
  );
}