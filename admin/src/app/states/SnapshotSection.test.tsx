import { describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import type { Deployment, GitOverview, Snapshot } from '../../runtime';
import { jsonResponse, renderApp, type MockCall } from '../test-utils';

const snapV1: Snapshot = { id: 'snap1', siteId: 's1', name: 'V1', createdAt: '2026-09-01T10:00:00Z' };
const snapV2: Snapshot = { id: 'snap2', siteId: 's1', name: 'V2', createdAt: '2026-09-02T10:00:00Z' };

const devActive: Deployment = { id: 'd1', siteId: 's1', snapshotId: 'snap1', environment: 'development', createdAt: '2026-09-01T11:00:00Z' };
const prodActive: Deployment = { id: 'd2', siteId: 's1', snapshotId: 'snap2', environment: 'production', createdAt: '2026-09-02T11:00:00Z' };

const cleanGit: GitOverview = {
  status: { siteId: 's1', branches: ['dev', 'main'], dev: { existing: true }, main: { existing: true }, dirty: false },
  commits: [],
};
const dirtyGit: GitOverview = { ...cleanGit, status: { ...cleanGit.status, dirty: true } };

const requests = (calls: MockCall[], url: string, method: string) =>
  calls.filter((c) => c.url === url && c.method === method);

function fixture(opts: { snapshots: Snapshot[]; deployments: Deployment[]; git?: GitOverview }) {
  const git = opts.git ?? cleanGit;
  return (url: string, init: RequestInit) => {
    const method = init.method ?? 'GET';
    if (url === '/api/sites/s1') return jsonResponse(200, { id: 's1', name: 'Site', slug: 'site', defaultLocale: 'ru', hosts: [] });
    if (url === '/api/sites/s1/snapshots') {
      if (method === 'POST') return jsonResponse(201, { id: 'snap3', siteId: 's1', name: 'V3', createdAt: '2026-09-03T10:00:00Z' });
      return jsonResponse(200, opts.snapshots);
    }
    if (url === '/api/sites/s1/deployments') {
      if (method === 'POST') return jsonResponse(201, { id: 'd3', siteId: 's1', snapshotId: 'snapX', environment: 'production', createdAt: '2026-09-03T11:00:00Z' });
      return jsonResponse(200, opts.deployments);
    }
    if (url === '/api/sites/s1/git') return jsonResponse(200, git);
    if (url.startsWith('/api/snapshots/') && method === 'DELETE') return jsonResponse(204, undefined);
    return jsonResponse(404, { error: 'not found' });
  };
}

const STATES_PATH = '/sites/s1?view=editor&section=states';

describe('SnapshotSection (R9 — Состояния)', () => {
  it('показывает текущие среды и статусы снапшотов', async () => {
    renderApp({ path: STATES_PATH, handler: fixture({ snapshots: [snapV1, snapV2], deployments: [devActive, prodActive] }) });

    expect(await screen.findByRole('heading', { name: 'Состояния' })).toBeTruthy();
    expect(screen.getByText('Разработка')).toBeTruthy();
    expect(screen.getByText('Production')).toBeTruthy();
    expect((await screen.findAllByText('V1')).length).toBeGreaterThan(0);
    expect((await screen.findAllByText('V2')).length).toBeGreaterThan(0);
    expect(screen.getAllByText('Активный')).toHaveLength(2);
  });

  it('показывает индикатор незакоммиченных правок при dirty git', async () => {
    const { calls } = await renderApp({ path: STATES_PATH, handler: fixture({ snapshots: [], deployments: [], git: dirtyGit }) });
    expect(await screen.findByText('Есть незакоммиченные правки — создайте снапшот для фиксации состояния')).toBeTruthy();
    void calls;

    await renderApp({ path: STATES_PATH, handler: fixture({ snapshots: [], deployments: [], git: cleanGit }) });
    await waitFor(() =>
      expect(screen.queryByText('Есть незакоммиченные правки — создайте снапшот для фиксации состояния')).toBeNull(),
    );
  });

  it('создаёт снапшот: POST /snapshots {name}', async () => {
    const { calls } = await renderApp({ path: STATES_PATH, handler: fixture({ snapshots: [], deployments: [] }) });

    fireEvent.change(await screen.findByLabelText('Новый снапшот'), { target: { value: 'V3' } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    await waitFor(() => expect(requests(calls, '/api/sites/s1/snapshots', 'POST').length).toBeGreaterThan(0));
    const post = requests(calls, '/api/sites/s1/snapshots', 'POST')[0];
    expect(JSON.parse(String(post?.init.body))).toEqual({ name: 'V3' });
  });

  it('выпуск в dev без подтверждения шлёт releaseDeployment', async () => {
    const { calls } = await renderApp({ path: STATES_PATH, handler: fixture({ snapshots: [snapV1, snapV2], deployments: [devActive, prodActive] }) });

    fireEvent.click(await screen.findByRole('button', { name: 'Выпустить dev' }));

    await waitFor(() => expect(requests(calls, '/api/sites/s1/deployments', 'POST').length).toBeGreaterThan(0));
    const post = requests(calls, '/api/sites/s1/deployments', 'POST')[0];
    expect(JSON.parse(String(post?.init.body))).toEqual({ snapshotId: 'snap2', environment: 'development' });
  });

  it('выпуск в prod требует подтверждения и шлёт releaseDeployment', async () => {
    const { calls } = await renderApp({ path: STATES_PATH, handler: fixture({ snapshots: [snapV1, snapV2], deployments: [] }) });

    const releaseButtons = await screen.findAllByRole('button', { name: 'Выпустить prod' });
    fireEvent.click(releaseButtons[0]!);

    const dialog = await screen.findByRole('alertdialog');
    expect(within(dialog).getByText('Снапшот «V2» будет выпущен в prod. Продолжить?')).toBeTruthy();
    fireEvent.click(within(dialog).getByRole('button', { name: 'Подтвердить' }));

    await waitFor(() => expect(requests(calls, '/api/sites/s1/deployments', 'POST').length).toBeGreaterThan(0));
    const post = requests(calls, '/api/sites/s1/deployments', 'POST')[0];
    expect(JSON.parse(String(post?.init.body))).toEqual({ snapshotId: 'snap2', environment: 'production' });
  });

  it('откат более раннего снапшота шлёт rollbackDeployment', async () => {
    const { calls } = await renderApp({ path: STATES_PATH, handler: fixture({ snapshots: [snapV1, snapV2], deployments: [prodActive] }) });

    const rollbackButtons = await screen.findAllByRole('button', { name: 'Откатить prod' });
    fireEvent.click(rollbackButtons[0]!);

    const dialog = await screen.findByRole('alertdialog');
    expect(within(dialog).getByText('Снапшот «V1» станет активным в prod. Продолжить?')).toBeTruthy();
    fireEvent.click(within(dialog).getByRole('button', { name: 'Подтвердить' }));

    await waitFor(() => expect(requests(calls, '/api/sites/s1/deployments/rollback', 'POST').length).toBeGreaterThan(0));
    const post = requests(calls, '/api/sites/s1/deployments/rollback', 'POST')[0];
    const url = post?.url ?? '';
    expect(url.includes('/api/sites/s1/deployments/rollback')).toBe(true);
    expect(JSON.parse(String(post?.init.body))).toEqual({ snapshotId: 'snap1', environment: 'production' });
  });
});