import { useEffect, useState, type FormEvent } from 'react';
import { useParams } from 'react-router-dom';
import { Inline, Stack } from '@liapoldus/ui-kit';
import {
  runOperation,
  type Deployment,
  type GitOverview,
  type Snapshot,
} from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { ConfirmButton } from '../components/ConfirmButton';
import { EntityTable } from '../components/EntityTable';
import { Field } from '../components/Field';
import { connectBuildWs } from '../ws-client';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

const PROD = 'production';
const DEV = 'development';

const INPUT_CLASS =
  'rounded border border-neutral-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none';

function activeFor(deployments: Deployment[], environment: string): Deployment | undefined {
  for (const d of deployments) {
    if (d.environment === environment) return d;
  }
  return undefined;
}

function snapshotById(snapshots: Snapshot[], id: string): Snapshot | undefined {
  for (const s of snapshots) {
    if (s.id === id) return s;
  }
  return undefined;
}

function envBadge(environment: string): string {
  return environment === PROD ? 'prod' : 'dev';
}

/**
 * Р9 — раздел «Состояния»: снапшоты и выпуск. Две текущие среды (dev/prod),
 * индикатор незакоммиченных правок (git dirty), список снапшотов с позицией в
 * каждой среде, создание снапшота, выпуск/откат (пин активного снапшота).
 */
export function SnapshotSection() {
  const { siteId = '' } = useParams();
  const { api, t, tokenStore } = useAdmin();

  const snapshots = useOperation<Snapshot[]>('listSnapshots', { siteId }, (d) =>
    Array.isArray(d) ? (d as Snapshot[]) : [],
  );
  const deployments = useOperation<Deployment[]>('listDeployments', { siteId }, (d) =>
    Array.isArray(d) ? (d as Deployment[]) : [],
  );
  const git = useOperation<GitOverview>('getGitOverview', { siteId }, (d) => {
    if (typeof d !== 'object' || d === null || !('status' in d)) {
      return {
        status: { siteId, branches: [], dev: { existing: false }, main: { existing: false }, dirty: false },
        commits: [],
      };
    }
    return d as GitOverview;
  });

  const [name, setName] = useState('');
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    const token = tokenStore.getState().token ?? '';
    return connectBuildWs(siteId, token, () => {
      snapshots.reload();
      deployments.reload();
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [siteId]);

  const snapshotList = snapshots.state.status === 'success' ? snapshots.state.data : [];
  const deploymentList = deployments.state.status === 'success' ? deployments.state.data : [];
  const dev = activeFor(deploymentList, DEV);
  const prod = activeFor(deploymentList, PROD);
  const dirty = git.state.status === 'success' && git.state.data ? git.state.data.status.dirty : false;

  const devName = dev ? (snapshotById(snapshotList, dev.snapshotId)?.name ?? dev.snapshotId) : undefined;
  const prodName = prod ? (snapshotById(snapshotList, prod.snapshotId)?.name ?? prod.snapshotId) : undefined;

  const isActiveIn = (snapshotId: string, env: Deployment | undefined) => env?.snapshotId === snapshotId;

  const createSnapshot = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!name.trim()) return;
    setCreating(true);
    setError('');
    const res = await runOperation(api, 'createSnapshot', { siteId, name: name.trim() }, t);
    setCreating(false);
    if (!res.ok) {
      setError(t('states.error', { detail: res.detail }));
      return;
    }
    setName('');
    snapshots.reload();
  };

  const release = async (snapshot: Snapshot, environment: string) => {
    setError('');
    const res = await runOperation(api, 'releaseDeployment', { siteId, snapshotId: snapshot.id, environment }, t);
    if (!res.ok) setError(t('states.error', { detail: res.detail }));
    deployments.reload();
    snapshots.reload();
  };

  const rollback = async (snapshot: Snapshot, environment: string) => {
    setError('');
    const res = await runOperation(api, 'rollbackDeployment', { siteId, snapshotId: snapshot.id, environment }, t);
    if (!res.ok) setError(t('states.error', { detail: res.detail }));
    deployments.reload();
    snapshots.reload();
  };

  const removeSnapshot = async (snapshot: Snapshot) => {
    setError('');
    const res = await runOperation(api, 'deleteSnapshot', { snapshotId: snapshot.id }, t);
    if (!res.ok) setError(t('states.error', { detail: res.detail }));
    snapshots.reload();
  };

  const earlierThan = (target: Snapshot, env: Deployment | undefined): boolean => {
    if (!env || env.snapshotId === target.id) return false;
    const current = snapshotById(snapshotList, env.snapshotId);
    if (!current?.createdAt || !target.createdAt) return false;
    return new Date(target.createdAt).getTime() < new Date(current.createdAt).getTime();
  };

  return (
    <Stack pad={4} gap={4} className="h-full overflow-auto">
      <div className="flex items-center justify-between gap-3">
        <h1 className="text-lg font-semibold text-neutral-900">{t('states.title')}</h1>
        <button
          type="button"
          onClick={() => {
            snapshots.reload();
            deployments.reload();
            git.reload();
          }}
          className={INPUT_CLASS}
        >
          {t('common.reload')}
        </button>
      </div>

      {error && <p className="text-sm text-red-600">{error}</p>}

      {dirty && (
        <div className="flex items-center gap-2 rounded border border-amber-300 bg-amber-50 px-3 py-2 text-sm text-amber-800">
          <span className="h-2 w-2 shrink-0 rounded-full bg-amber-500" aria-hidden="true" />
          {t('states.uncommitted')}
        </div>
      )}

      <section className="grid gap-3 sm:grid-cols-2">
        <div className="rounded-lg border border-neutral-200 p-3">
          <Inline justify="between" align="center" className="mb-1">
            <h2 className="text-sm font-medium text-neutral-800">{t('states.dev.title')}</h2>
            <Badge variant="outline" className="bg-neutral-100 text-neutral-600 border-neutral-200">
              {t('states.environment.development')}
            </Badge>
          </Inline>
          <p className="text-xs text-neutral-500">{t('states.dev.description')}</p>
          <p className="mt-2 text-sm text-neutral-800">
            {devName ? (
              <span className="font-medium">{devName}</span>
            ) : (
              <span className="text-neutral-400">{t('states.none')}</span>
            )}
          </p>
        </div>
        <div className="rounded-lg border border-neutral-200 p-3">
          <Inline justify="between" align="center" className="mb-1">
            <h2 className="text-sm font-medium text-neutral-800">{t('states.prod.title')}</h2>
            <Badge variant="outline" className="bg-purple-100 text-purple-700 border-purple-200">
              {t('states.environment.production')}
            </Badge>
          </Inline>
          <p className="text-xs text-neutral-500">{t('states.prod.description')}</p>
          <p className="mt-2 text-sm text-neutral-800">
            {prodName ? (
              <span className="font-medium">{prodName}</span>
            ) : (
              <span className="text-neutral-400">{t('states.none')}</span>
            )}
          </p>
        </div>
      </section>

      <section>
        <Inline justify="between" align="center" className="mb-2">
          <h2 className="text-base font-medium text-neutral-700">{t('states.snapshot')}</h2>
        </Inline>
        <form
          onSubmit={createSnapshot}
          className="mb-3 flex items-end gap-3 rounded border border-neutral-200 bg-neutral-50 p-3"
        >
          <Field label={t('states.snapshot.new')} required>
            <Input
              className={INPUT_CLASS}
              value={name}
              onChange={(e) => setName(e.target.value)}
              aria-label={t('states.snapshot.new')}
            />
          </Field>
          <Button
            type="submit"
            size="sm"
            disabled={creating || !name.trim()}
            className="bg-blue-600 text-white hover:bg-blue-700"
          >
            {t('common.create')}
          </Button>
        </form>

        {snapshots.state.status === 'loading' && (
          <p className="text-sm text-neutral-400">{t('common.loading')}</p>
        )}
        {snapshots.state.status === 'error' && (
          <p className="text-sm text-red-600">
            {t('common.error')}: {snapshots.state.detail}
          </p>
        )}
        {snapshots.state.status === 'success' &&
          (snapshotList.length === 0 ? (
            <p className="text-sm text-neutral-400">{t('states.snapshot.none')}</p>
          ) : (
            <EntityTable<Snapshot>
              gridClass="grid-cols-[minmax(0,2fr)_minmax(0,1fr)_minmax(0,7rem)_minmax(0,7rem)_minmax(0,20rem)]"
              getKey={(s) => s.id}
              columns={[
                {
                  key: 'name',
                  label: t('states.snapshot.name'),
                  render: (s) => <span className="font-medium">{s.name}</span>,
                },
                {
                  key: 'createdAt',
                  label: t('states.createdAt'),
                  render: (s) => (
                    <span className="text-xs text-neutral-500">
                      {s.createdAt ? new Date(s.createdAt).toLocaleString('ru-RU') : '—'}
                    </span>
                  ),
                },
                {
                  key: 'dev',
                  label: t('states.environment.development'),
                  render: (s) =>
                    isActiveIn(s.id, dev) ? (
                      <Badge className="bg-green-100 text-green-700 border-green-200">
                        {t('states.status.current')}
                      </Badge>
                    ) : (
                      <span className="text-xs text-neutral-400">—</span>
                    ),
                },
                {
                  key: 'prod',
                  label: t('states.environment.production'),
                  render: (s) =>
                    isActiveIn(s.id, prod) ? (
                      <Badge className="bg-green-100 text-green-700 border-green-200">
                        {t('states.status.current')}
                      </Badge>
                    ) : earlierThan(s, prod) ? (
                      <Badge variant="outline" className="bg-neutral-100 text-neutral-600 border-neutral-200">
                        {t('states.status.published')}
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
                      {!isActiveIn(s.id, dev) && (
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          onClick={() => release(s, DEV)}
                        >
                          {t('states.release')} {envBadge(DEV)}
                        </Button>
                      )}
                      {!isActiveIn(s.id, prod) &&
                        (earlierThan(s, prod) ? (
                          <ConfirmButton
                            label={`${t('states.rollback')} ${envBadge(PROD)}`}
                            description={t('states.rollback.description', {
                              name: s.name,
                              environment: t('states.environment.production'),
                            })}
                            onConfirm={() => rollback(s, PROD)}
                          />
                        ) : (
                          <ConfirmButton
                            label={`${t('states.release')} ${envBadge(PROD)}`}
                            description={t('states.release.description', {
                              name: s.name,
                              environment: t('states.environment.production'),
                            })}
                            onConfirm={() => release(s, PROD)}
                          />
                        ))}
                      <ConfirmButton label={t('common.delete')} onConfirm={() => removeSnapshot(s)} />
                    </Inline>
                  ),
                },
              ]}
              rows={[...snapshotList].reverse()}
            />
          ))}
      </section>
    </Stack>
  );
}