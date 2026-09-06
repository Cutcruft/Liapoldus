import { useState, type FormEvent, type ReactNode } from 'react';
import { useParams } from 'react-router-dom';
import { Inline, Stack } from '@liapoldus/ui-kit';
import { runOperation, type GitCommitInfo, type GitOverview, type GitStatus, type Snapshot } from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { EntityTable } from '../components/EntityTable';
import { Field } from '../components/Field';
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

const INPUT_CLASS =
  'rounded border border-neutral-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none';

function shortSha(sha: string | undefined): string {
  if (!sha) return '—';
  return sha.length > 8 ? sha.slice(0, 8) : sha;
}

function branchChip(name: string, status: GitStatus['dev'], mainSha: string | undefined, dirty: boolean): ReactNode {
  const heading =
    status.head?.message ||
    (status.existing ? `HEAD ${shortSha(status.sha)}` : undefined) ||
    undefined;
  return (
    <Stack gap={1} className="rounded border border-neutral-200 bg-neutral-50 p-3">
      <Inline gap={2} align="center">
        <Badge variant="outline" className={name === 'main' ? 'bg-purple-100 text-purple-700 border-purple-200' : 'bg-blue-100 text-blue-700 border-blue-200'}>
          {name}
        </Badge>
        {status.existing ? (
          <Badge variant="outline" className="bg-neutral-100 text-neutral-600 border-neutral-200">
            {shortSha(status.sha)}
          </Badge>
        ) : (
          <span className="text-xs text-neutral-400">нет HEAD</span>
        )}
      </Inline>
      {name === 'dev' && (
        <Badge variant="outline" className={dirty ? 'bg-amber-100 text-amber-700 border-amber-200' : 'bg-green-100 text-green-700 border-green-200'}>
          {dirty ? 'есть несохранённые изменения' : 'чисто'}
        </Badge>
      )}
      {name === 'main' && mainSha && status.sha !== mainSha && (
        <span className="text-xs text-neutral-400">отличается от prod-состояния</span>
      )}
      {heading && <p className="text-xs text-neutral-600 italic">{heading}</p>}
    </Stack>
  );
}

function RevealDialog({
  trigger,
  title,
  description,
  confirmLabel,
  revealLabel,
  invalidText,
  expected,
  onConfirm,
}: {
  trigger: ReactNode;
  title: string;
  description: string;
  confirmLabel: string;
  revealLabel: string;
  invalidText: string;
  expected: string;
  onConfirm: () => Promise<void>;
}) {
  const { t } = useAdmin();
  const [typed, setTyped] = useState('');
  const ok = typed === expected;
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
            <Input value={typed} onChange={(e) => setTyped(e.target.value)} placeholder={expected} aria-label={revealLabel} />
          </Field>
          {typed !== '' && !ok && <p className="text-sm text-red-600">{invalidText}</p>}
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

export function SiteCommitsPage() {
  const { siteId = '' } = useParams();
  const { api, t } = useAdmin();

  const overview = useOperation<GitOverview>('getGitOverview', { siteId }, (d) =>
    typeof d === 'object' && d !== null && 'status' in d ? (d as GitOverview) : { status: empty(), commits: [] },
  );
  const snapshots = useOperation<Snapshot[]>('listSnapshots', { siteId }, (d) =>
    Array.isArray(d) ? (d as Snapshot[]) : [],
  );

  const [message, setMessage] = useState('');
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState('');

  const status = overview.state.status === 'success' ? overview.state.data.status : empty();
  const commits = overview.state.status === 'success' ? overview.state.data.commits : [];

  const reload = () => {
    overview.reload();
    snapshots.reload();
  };

  const commit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    setActionError('');
    const res = await runOperation(api, 'commitGit', { siteId, message: message.trim() }, t);
    setBusy(false);
    if (res.ok) {
      setMessage('');
      reload();
    } else {
      setActionError(res.detail);
    }
  };

  const publish = async (publishMessage: string) => {
    setActionError('');
    const res = await runOperation(api, 'publishGit', { siteId, message: publishMessage.trim() }, t);
    if (!res.ok) setActionError(res.detail);
    reload();
  };

  const restore = async (sha: string) => {
    setActionError('');
    const res = await runOperation(api, 'restoreGit', { siteId, sha, message: '' }, t);
    if (!res.ok) setActionError(res.detail);
    reload();
  };

  const rollback = async (snapshot: Snapshot) => {
    if (!snapshot.gitSha) return;
    setActionError('');
    const res = await runOperation(api, 'rollbackGit', { siteId, sha: snapshot.gitSha, message: '' }, t);
    if (!res.ok) setActionError(res.detail);
    reload();
  };

  const notInitialized = overview.state.status === 'success' && !status.dev.existing && !status.main.existing;

  return (
    <Stack pad={8} gap={4}>
      <Inline justify="between" align="center">
        <h1 className="text-xl font-medium">{t('git.title')}</h1>
        <button type="button" onClick={reload} className={INPUT_CLASS}>
          {t('common.reload')}
        </button>
      </Inline>

      {actionError && <p className="text-sm text-red-600">{t('git.error', { detail: actionError })}</p>}

      {overview.state.status === 'loading' && <p className="text-sm text-neutral-400">{t('common.loading')}</p>}
      {overview.state.status === 'error' && (
        <p className="text-sm text-red-600">
          {t('common.error')}: {overview.state.detail}
        </p>
      )}

      {overview.state.status === 'success' && (
        <>
          {notInitialized ? (
            <p className="text-sm text-neutral-400">{t('git.notInit')}</p>
          ) : (
            <section aria-label={t('git.branches')}>
              <h2 className="mb-2 text-base font-medium text-neutral-700">{t('git.branches')}</h2>
              <Inline gap={3} className="max-w-2xl">
                {branchChip('dev', status.dev, status.main.sha, status.dirty)}
                {branchChip('main', status.main, status.main.sha, status.dirty)}
              </Inline>
            </section>
          )}

          <form onSubmit={(e) => void commit(e)} className="flex items-end gap-3 rounded border border-neutral-200 bg-neutral-50 p-3">
            <Field label={t('git.commit.new')}>
              <input
                className={INPUT_CLASS}
                value={message}
                onChange={(e) => setMessage(e.target.value)}
                placeholder={t('git.commit.placeholder')}
                aria-label={t('git.commit.new')}
              />
            </Field>
            <Button type="submit" variant="outline" size="sm" disabled={busy}>
              {t('git.commit.btn')}
            </Button>
            <RevealDialog
              trigger={
                <Button type="button" size="sm">
                  {t('git.publish')}
                </Button>
              }
              title={t('git.publish.title')}
              description={t('git.publish.description')}
              confirmLabel={t('git.publish')}
              revealLabel={t('git.publish.reveal')}
              invalidText={t('git.publish.invalid')}
              expected="PROD"
              onConfirm={() => publish('')}
            />
          </form>

          <section>
            <h2 className="mb-2 text-base font-medium text-neutral-700">{t('git.commits')}</h2>
            {commits.length === 0 ? (
              <p className="text-sm text-neutral-400">{t('git.commits.none')}</p>
            ) : (
              <EntityTable<GitCommitInfo>
                gridClass="grid-cols-[minmax(0,7rem)_minmax(0,2fr)_minmax(0,10rem)_minmax(0,11rem)_minmax(0,14rem)]"
                getKey={(c) => c.sha}
                columns={[
                  {
                    key: 'sha',
                    label: t('git.sha'),
                    render: (c) => <code className="text-xs">{shortSha(c.sha)}</code>,
                  },
                  {
                    key: 'message',
                    label: t('git.message'),
                    render: (c) => <span className="font-medium">{c.message}</span>,
                  },
                  {
                    key: 'author',
                    label: t('git.author'),
                    render: (c) => <span className="text-xs text-neutral-500">{c.author}</span>,
                  },
                  {
                    key: 'time',
                    label: t('git.date'),
                    render: (c) => (
                      <span className="text-xs text-neutral-500">
                        {c.time ? new Date(c.time).toLocaleString('ru-RU') : '—'}
                      </span>
                    ),
                  },
                  {
                    key: 'actions',
                    label: t('common.actions'),
                    className: 'text-right',
                    render: (c) => (
                      <Inline gap={2} justify="end">
                        <RevealDialog
                          trigger={
                            <Button type="button" variant="outline" size="sm">
                              {t('git.restore')}
                            </Button>
                          }
                          title={t('git.restore.title')}
                          description={t('git.restore.description', { short: shortSha(c.sha) })}
                          confirmLabel={t('git.restore')}
                          revealLabel={t('git.restore.reveal')}
                          invalidText={t('git.restore.invalid')}
                          expected="RESTORE"
                          onConfirm={() => restore(c.sha)}
                        />
                      </Inline>
                    ),
                  },
                ]}
                rows={commits}
              />
            )}
          </section>

          <section>
            <h2 className="mb-2 text-base font-medium text-neutral-700">{t('git.snapshots')}</h2>
            {snapshots.state.status === 'loading' && <p className="text-sm text-neutral-400">{t('common.loading')}</p>}
            {snapshots.state.status === 'error' && (
              <p className="text-sm text-red-600">
                {t('common.error')}: {snapshots.state.detail}
              </p>
            )}
            {snapshots.state.status === 'success' &&
              (snapshots.state.data.length === 0 ? (
                <p className="text-sm text-neutral-400">{t('git.snapshots.none')}</p>
              ) : (
                <EntityTable<Snapshot>
                  gridClass="grid-cols-[minmax(0,2fr)_minmax(0,8rem)_minmax(0,11rem)_minmax(0,14rem)]"
                  getKey={(s) => s.id}
                  columns={[
                    {
                      key: 'name',
                      label: t('git.message'),
                      render: (s) => (
                        <Inline gap={2} align="center">
                          <span className="font-medium">{s.name}</span>
                          {status.main.sha && s.gitSha === status.main.sha && (
                            <Badge className="bg-green-100 text-green-700 border-green-200">{t('git.current')}</Badge>
                          )}
                        </Inline>
                      ),
                    },
                    {
                      key: 'gitSha',
                      label: t('git.sha'),
                      render: (s) => <code className="text-xs">{s.gitSha ? shortSha(s.gitSha) : '—'}</code>,
                    },
                    {
                      key: 'createdAt',
                      label: t('git.date'),
                      render: (s) => (
                        <span className="text-xs text-neutral-500">
                          {s.createdAt ? new Date(s.createdAt).toLocaleString('ru-RU') : '—'}
                        </span>
                      ),
                    },
                    {
                      key: 'actions',
                      label: t('common.actions'),
                      className: 'text-right',
                      render: (s) =>
                        s.gitSha ? (
                          <Inline gap={2} justify="end">
                            <RevealDialog
                              trigger={
                                <Button type="button" variant="destructive" size="sm">
                                  {t('git.rollback')}
                                </Button>
                              }
                              title={t('git.rollback.title')}
                              description={t('git.rollback.description', { name: s.name, short: shortSha(s.gitSha) })}
                              confirmLabel={t('git.rollback')}
                              revealLabel={t('git.rollback.reveal')}
                              invalidText={t('git.publish.invalid')}
                              expected="PROD"
                              onConfirm={() => rollback(s)}
                            />
                          </Inline>
                        ) : (
                          <span className="text-xs text-neutral-400">—</span>
                        ),
                    },
                  ]}
                  rows={[...snapshots.state.data].reverse()}
                />
              ))}
          </section>
        </>
      )}
    </Stack>
  );
}

function empty(): GitStatus {
  return { siteId: '', branches: [], dev: { existing: false }, main: { existing: false }, dirty: false };
}