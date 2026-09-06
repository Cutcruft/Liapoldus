import { useEffect, useState, type FormEvent, type ReactNode } from 'react';
import { useParams } from 'react-router-dom';
import { Inline, Stack } from '@liapoldus/ui-kit';
import { runOperation, type Build, type Snapshot } from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { ConfirmButton } from '../components/ConfirmButton';
import { EntityTable } from '../components/EntityTable';
import { Field } from '../components/Field';
import { connectBuildWs } from '../builds/build-ws';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog';

const PROD = 'production';
const REVEAL = 'PROD';

const INPUT_CLASS =
  'rounded border border-neutral-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none';

/** Последняя готовая сборка окружения (список приходит от старых к новым). */
function latestReady(builds: Build[], environment: string): Build | undefined {
  let found: Build | undefined;
  for (const b of builds) {
    if (b.environment === environment && b.status === 'ready') found = b;
  }
  return found;
}

function statusBadge(status: Build['status']): ReactNode {
  const color =
    status === 'ready'
      ? 'bg-green-100 text-green-700 border-green-200'
      : status === 'failed'
        ? 'bg-red-100 text-red-700 border-red-200'
        : status === 'building'
          ? 'bg-blue-100 text-blue-700 border-blue-200'
          : 'bg-neutral-100 text-neutral-600 border-neutral-200';
  return <Badge variant="outline" className={color}>{status}</Badge>;
}

function envBadge(environment: string): ReactNode {
  const isProd = environment === PROD;
  return (
    <Badge variant="outline" className={isProd ? 'bg-purple-100 text-purple-700 border-purple-200' : 'bg-neutral-100 text-neutral-600 border-neutral-200'}>
      {isProd ? 'prod' : 'dev'}
    </Badge>
  );
}

function PublishDialog({
  trigger,
  title,
  description,
  confirmLabel,
  revealLabel,
  onConfirm,
}: {
  trigger: ReactNode;
  title: string;
  description: string;
  confirmLabel: string;
  revealLabel: string;
  onConfirm: () => Promise<void>;
}) {
  const { t } = useAdmin();
  const [typed, setTyped] = useState('');
  const ok = typed === REVEAL;
  return (
    <AlertDialog>
      <AlertDialogTrigger asChild>{trigger}</AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          <AlertDialogDescription>{description}</AlertDialogDescription>
        </AlertDialogHeader>
        <Stack gap={2}>
          <Field label={revealLabel} required>
            <Input value={typed} onChange={(e) => setTyped(e.target.value)} placeholder={REVEAL} aria-label={revealLabel} />
          </Field>
          {typed !== '' && !ok && <p className="text-sm text-red-600">{t('builds.publish.invalid')}</p>}
        </Stack>
        <AlertDialogFooter>
          <AlertDialogCancel asChild>
            <Button type="button" variant="outline" size="sm">
              {t('common.cancel')}
            </Button>
          </AlertDialogCancel>
          <AlertDialogAction asChild>
            <Button type="button" variant="destructive" size="sm" disabled={!ok} onClick={() => void onConfirm()}>
              {confirmLabel}
            </Button>
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

export function SiteBuildsPage() {
  const { siteId = '' } = useParams();
  const { api, t, tokenStore } = useAdmin();

  const snapshots = useOperation<Snapshot[]>('listSnapshots', { siteId }, (d) =>
    Array.isArray(d) ? (d as Snapshot[]) : [],
  );
  const builds = useOperation<Build[]>('listBuilds', { siteId }, (d) =>
    Array.isArray(d) ? (d as Build[]) : [],
  );

  const [name, setName] = useState('');
  const [creating, setCreating] = useState(false);
  const [publishError, setPublishError] = useState('');

  useEffect(() => {
    const token = tokenStore.getState().token ?? '';
    return connectBuildWs(siteId, token, () => {
      snapshots.reload();
      builds.reload();
    });
  }, [siteId]);

  const currentProd = latestReady(builds.state.status === 'success' ? builds.state.data : [], PROD);

  const createSnapshot = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!name.trim()) return;
    setCreating(true);
    const res = await runOperation(api, 'createSnapshot', { siteId, name: name.trim() }, t);
    setCreating(false);
    if (res.ok) {
      setName('');
      snapshots.reload();
    }
  };

  const publish = async (snapshot: Snapshot) => {
    setPublishError('');
    const res = await runOperation(api, 'createBuild', { siteId, snapshotId: snapshot.id, environment: PROD }, t);
    if (!res.ok) setPublishError(t('builds.publishError', { detail: res.detail }));
    builds.reload();
    snapshots.reload();
  };

  const rebuild = async (build: Build) => {
    const res = await runOperation(api, 'createBuild', { siteId, snapshotId: build.snapshotId, environment: build.environment }, t);
    if (!res.ok) setPublishError(t('builds.publishError', { detail: res.detail }));
    builds.reload();
  };

  const removeSnapshot = async (snapshot: Snapshot) => {
    const res = await runOperation(api, 'deleteSnapshot', { snapshotId: snapshot.id }, t);
    if (res.ok) {
      snapshots.reload();
      builds.reload();
    }
  };

  const list = builds.state.status === 'success' ? builds.state.data : [];
  const hasProdBuild = (snapshotId: string) => list.some((b) => b.environment === PROD && b.snapshotId === snapshotId && b.status === 'ready');

  return (
    <Stack pad={8} gap={4}>
      <h1 className="text-xl font-medium">{t('builds.title')}</h1>

      {publishError && <p className="text-sm text-red-600">{publishError}</p>}

      <section>
        <Inline justify="between" align="center">
          <h2 className="text-base font-medium text-neutral-700">{t('builds.snapshots')}</h2>
          <button
            type="button"
            onClick={() => snapshots.reload()}
            className={INPUT_CLASS}
          >
            {t('common.reload')}
          </button>
        </Inline>

        <form onSubmit={createSnapshot} className="mt-3 flex items-end gap-3 rounded border border-neutral-200 bg-neutral-50 p-3">
          <Field label={t('builds.snapshot.new')} required>
            <input className={INPUT_CLASS} value={name} onChange={(e) => setName(e.target.value)} aria-label={t('builds.snapshot.new')} />
          </Field>
          <button
            type="submit"
            disabled={creating || !name.trim()}
            className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700 disabled:opacity-50"
          >
            {t('common.create')}
          </button>
        </form>

        {snapshots.state.status === 'loading' && <p className="mt-3 text-sm text-neutral-400">{t('common.loading')}</p>}
        {snapshots.state.status === 'error' && (
          <p className="mt-3 text-sm text-red-600">
            {t('common.error')}: {snapshots.state.detail}
          </p>
        )}
        {snapshots.state.status === 'success' &&
          (snapshots.state.data.length === 0 ? (
            <p className="mt-3 text-sm text-neutral-400">{t('builds.snapshot.none')}</p>
          ) : (
            <EntityTable<Snapshot>
              gridClass="grid-cols-[minmax(0,2fr)_minmax(0,1fr)_minmax(0,7rem)_minmax(0,18rem)]"
              getKey={(s) => s.id}
              columns={[
                {
                  key: 'name',
                  label: t('builds.snapshot.name'),
                  render: (s) => <span className="font-medium">{s.name}</span>,
                },
                {
                  key: 'createdAt',
                  label: t('builds.createdAt'),
                  render: (s) => <span className="text-xs text-neutral-500">{s.createdAt ? new Date(s.createdAt).toLocaleString('ru-RU') : '—'}</span>,
                },
                {
                  key: 'published',
                  label: t('builds.published'),
                  render: (s) =>
                    currentProd?.snapshotId === s.id ? (
                      <Badge className="bg-green-100 text-green-700 border-green-200">{t('builds.current.prod')}</Badge>
                    ) : hasProdBuild(s.id) ? (
                      <Badge variant="outline" className="bg-neutral-100 text-neutral-600 border-neutral-200">
                        {envBadge(PROD)}
                      </Badge>
                    ) : (
                      <span className="text-xs text-neutral-400">—</span>
                    ),
                },
                {
                  key: 'actions',
                  label: t('common.actions'),
                  className: 'text-right',
                  render: (s) => (
                    <Inline gap={2} justify="end">
                      <PublishDialog
                        trigger={
                          <Button type="button" variant="outline" size="sm">
                            {t('builds.publish')}
                          </Button>
                        }
                        title={t('builds.publish.title')}
                        description={t('builds.publish.description', { name: s.name })}
                        confirmLabel={t('builds.publish')}
                        revealLabel={t('builds.publish.reveal')}
                        onConfirm={() => publish(s)}
                      />
                      {hasProdBuild(s.id) && currentProd?.snapshotId !== s.id && (
                        <PublishDialog
                          trigger={
                            <Button type="button" variant="outline" size="sm">
                              {t('builds.rollback')}
                            </Button>
                          }
                          title={t('builds.rollback.title')}
                          description={t('builds.rollback.description', { name: s.name })}
                          confirmLabel={t('builds.rollback')}
                          revealLabel={t('builds.rollback.reveal')}
                          onConfirm={() => publish(s)}
                        />
                      )}
                      <ConfirmButton label={t('builds.delete.confirm')} onConfirm={() => removeSnapshot(s)} />
                    </Inline>
                  ),
                },
              ]}
              rows={[...snapshots.state.data].reverse()}
            />
          ))}
      </section>

      <section>
        <Inline justify="between" align="center">
          <h2 className="text-base font-medium text-neutral-700">{t('builds.builds')}</h2>
          <button type="button" onClick={() => builds.reload()} className={INPUT_CLASS}>
            {t('common.reload')}
          </button>
        </Inline>

        {builds.state.status === 'loading' && <p className="mt-3 text-sm text-neutral-400">{t('common.loading')}</p>}
        {builds.state.status === 'error' && (
          <p className="mt-3 text-sm text-red-600">
            {t('common.error')}: {builds.state.detail}
          </p>
        )}
        {builds.state.status === 'success' &&
          (builds.state.data.length === 0 ? (
            <p className="mt-3 text-sm text-neutral-400">{t('builds.builds.none')}</p>
          ) : (
            <EntityTable<Build>
              gridClass="grid-cols-[minmax(0,3.5rem)_minmax(0,1fr)_minmax(0,6rem)_minmax(0,9rem)_minmax(0,1fr)_minmax(0,16rem)]"
              getKey={(b) => b.id}
              columns={[
                { key: 'env', label: t('builds.env'), render: (b) => envBadge(b.environment) },
                { key: 'snapshotId', label: t('builds.snapshot.name'), render: (b) => <code className="text-xs">{b.snapshotId}</code> },
                { key: 'status', label: t('builds.statusCol'), render: (b) => statusBadge(b.status) },
                {
                  key: 'finishedAt',
                  label: t('builds.createdAt'),
                  render: (b) => (
                    <span className="text-xs text-neutral-500">
                      {b.finishedAt ? new Date(b.finishedAt).toLocaleString('ru-RU') : b.createdAt ? new Date(b.createdAt).toLocaleString('ru-RU') : '—'}
                    </span>
                  ),
                },
                {
                  key: 'artifact',
                  label: 'Artifact',
                  render: (b) => (b.artifactDir ? <code className="text-xs">{b.artifactDir}</code> : <span className="text-xs text-neutral-400">—</span>),
                },
                {
                  key: 'actions',
                  label: t('common.actions'),
                  className: 'text-right',
                  render: (b) => (
                    <Inline gap={2} justify="end">
                      {b.status === 'failed' && (
                        <button type="button" onClick={() => rebuild(b)} className="rounded border border-neutral-300 px-2 py-1 text-xs text-neutral-600 hover:border-blue-300 hover:text-blue-600">
                          {t('builds.rebuild')}
                        </button>
                      )}
                      {b.status === 'ready' && b.artifactDir && (
                        <a
                          href={`/${b.artifactDir}/dist/index.html`}
                          target="_blank"
                          rel="noreferrer"
                          className="rounded border border-neutral-300 px-2 py-1 text-xs text-neutral-600 hover:border-blue-300 hover:text-blue-600"
                        >
                          {t('builds.open')}
                        </a>
                      )}
                      {b.log.length > 0 && (
                        <details className="shrink-0">
                          <summary className="cursor-pointer rounded border border-neutral-300 px-2 py-1 text-xs text-neutral-600">
                            {t('builds.log')}
                          </summary>
                          <pre className="max-w-md overflow-auto rounded border border-neutral-200 bg-neutral-50 p-2 text-xs text-neutral-700">
                            {b.log.join('\n')}
                          </pre>
                        </details>
                      )}
                    </Inline>
                  ),
                },
              ]}
              rows={[...builds.state.data].reverse()}
            />
          ))}
      </section>
    </Stack>
  );
}