import { describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { renderApp, type MockCall } from '../test-utils';
import type { GitCommitInfo, GitOverview, Snapshot } from '../../runtime';

const now = '2026-09-05T10:00:00Z';
const yesterday = '2026-09-04T09:00:00Z';

const sha1 = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa';
const sha2 = 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb';

const overview: GitOverview = {
  status: {
    siteId: 's1',
    branches: ['dev', 'main'],
    dev: { existing: true, sha: sha2, head: { sha: sha2, message: 'second commit', author: 'admin', time: now } },
    main: { existing: true, sha: sha1, head: { sha: sha1, message: 'initial', author: 'admin', time: yesterday } },
    dirty: false,
  },
  commits: [
    { sha: sha2, message: 'second commit', author: 'admin', time: now },
    { sha: sha1, message: 'initial', author: 'admin', time: yesterday },
  ],
};

const overviewEmpty: GitOverview = {
  status: {
    siteId: 's1',
    branches: [],
    dev: { existing: false },
    main: { existing: false },
    dirty: false,
  },
  commits: [],
};

const snapshot1: Snapshot = { id: 'snap1', siteId: 's1', name: 'publish v1', gitSha: sha1, createdAt: yesterday };
const snapshot2: Snapshot = { id: 'snap2', siteId: 's1', name: 'publish v2', gitSha: sha2, createdAt: now };

function fixture({ data, snapshots }: { data: GitOverview; snapshots: Snapshot[] }) {
  return async (url: string, init: RequestInit) => {
    const method = init.method ?? 'GET';
    if (url === '/api/sites/s1/git') return jsonResponse(200, data);
    if (url === '/api/sites/s1/snapshots') {
      if (method === 'POST') return jsonResponse(201, snapshots[0]!);
      return jsonResponse(200, snapshots);
    }
    if (url.endsWith('/git/commit')) return jsonResponse(201, { sha: sha2 });
    if (url.endsWith('/git/publish')) return jsonResponse(201, { sha: sha2 });
    if (url.endsWith('/git/restore')) return jsonResponse(200, { sha: sha1 });
    if (url.endsWith('/git/rollback')) return jsonResponse(200, { sha: sha1 });
    return jsonResponse(404, { error: 'not found' });
  };
}

function jsonResponse(status: number, body: unknown): Response {
  return new Response(body === undefined ? null : JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

const requests = (calls: MockCall[], url: string, method: string) =>
  calls.filter((c) => c.url === url && c.method === method);

describe('SiteCommitsPage', () => {
  it('показывает «не инициализирован» при пустом репо', async () => {
    await renderApp({
      path: '/sites/s1/git',
      handler: fixture({ data: overviewEmpty, snapshots: [] }),
    });

    expect(await screen.findByText('Репозиторий не инициализирован — сделайте первый коммит')).toBeTruthy();
    expect(screen.getByText('Коммитов пока нет')).toBeTruthy();
  });

  it('рендерит ветки, историю и снапшоты', async () => {
    await renderApp({
      path: '/sites/s1/git',
      handler: fixture({ data: overview, snapshots: [snapshot1, snapshot2] }),
    });

    expect((await screen.findAllByText('second commit')).length).toBeGreaterThan(0);
    expect((await screen.findAllByText('initial')).length).toBeGreaterThan(0);
    expect((await screen.findAllByText('aaaaaaaa')).length).toBeGreaterThan(0);
    expect((await screen.findAllByText('bbbbbbbb')).length).toBeGreaterThan(0);

    expect(screen.getByText('чисто')).toBeTruthy();
    expect(screen.getByText('текущий prod')).toBeTruthy();
  });

  it('коммит отправляет POST на /git/commit', async () => {
    const { calls } = await renderApp({
      path: '/sites/s1/git',
      handler: fixture({ data: overview, snapshots: [] }),
    });

    const input = await screen.findByLabelText('Сообщение коммита');
    fireEvent.change(input, { target: { value: 'new feature' } });
    fireEvent.click(screen.getByRole('button', { name: 'Коммит' }));

    await waitFor(() => {
      const posts = requests(calls, '/api/sites/s1/git/commit', 'POST');
      expect(posts.length).toBeGreaterThan(0);
      expect(JSON.parse(posts[0]!.init.body as string)).toEqual({ message: 'new feature' });
    });
  });

  it('publish требует PROD и шлёт POST на /git/publish', async () => {
    const { calls } = await renderApp({
      path: '/sites/s1/git',
      handler: fixture({ data: overview, snapshots: [] }),
    });

    const publishBtn = await screen.findByRole('button', { name: 'Опубликовать' });
    fireEvent.click(publishBtn);

    const dialog = await screen.findByRole('alertdialog');
    const reveal = within(dialog).getByLabelText('Введите PROD для подтверждения');
    const confirm = within(dialog).getByRole('button', { name: 'Опубликовать' });

    fireEvent.change(reveal, { target: { value: 'PR' } });
    expect((confirm as HTMLButtonElement).disabled).toBe(true);

    fireEvent.change(reveal, { target: { value: 'PROD' } });
    expect((confirm as HTMLButtonElement).disabled).toBe(false);

    fireEvent.click(confirm);
    await waitFor(() => {
      expect(requests(calls, '/api/sites/s1/git/publish', 'POST').length).toBeGreaterThan(0);
    });
  });

  it('restore требует RESTORE и отправляет sha коммита', async () => {
    const { calls } = await renderApp({
      path: '/sites/s1/git',
      handler: fixture({ data: overview, snapshots: [] }),
    });

    await screen.findAllByText('second commit');
    const restoreBtns = await screen.findAllByRole('button', { name: 'Восстановить' });
    fireEvent.click(restoreBtns[1]!);

    const dialog = await screen.findByRole('alertdialog');
    expect(within(dialog).queryByText(/comitta/)).toBeFalsy();

    const reveal = within(dialog).getByLabelText('Введите RESTORE для подтверждения');
    fireEvent.change(reveal, { target: { value: 'RESTORE' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Восстановить' }));

    await waitFor(() => {
      const posts = requests(calls, '/api/sites/s1/git/restore', 'POST');
      expect(posts.length).toBeGreaterThan(0);
      const body = JSON.parse(posts[0]!.init.body as string);
      expect(body.sha).toBe(sha1);
    });
  });

  it('rollback отправляет sha снапшота на /git/rollback', async () => {
    const { calls } = await renderApp({
      path: '/sites/s1/git',
      handler: fixture({ data: overview, snapshots: [snapshot1, snapshot2] }),
    });

    const rollbackBtns = await screen.findAllByRole('button', { name: 'Откатить' });
    fireEvent.click(rollbackBtns[0]!);

    const dialog = await screen.findByRole('alertdialog');
    const reveal = within(dialog).getByLabelText('Введите PROD для подтверждения');
    fireEvent.change(reveal, { target: { value: 'PROD' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Откатить' }));

    await waitFor(() => {
      const posts = requests(calls, '/api/sites/s1/git/rollback', 'POST');
      expect(posts.length).toBeGreaterThan(0);
      const body = JSON.parse(posts[0]!.init.body as string);
      expect(body.sha).toBe(sha2);
    });
  });
});
