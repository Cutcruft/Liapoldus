import { afterEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { renderApp, type MockCall } from '../test-utils';
import { setBuildSocketFactory, type WsLike } from '../builds/build-ws';
import type { Build, BuildEvent, Snapshot } from '../../runtime';

const snapV1: Snapshot = { id: 'snap1', siteId: 's1', name: 'V1', createdAt: '2026-09-01T10:00:00Z' };
const snapV2: Snapshot = { id: 'snap2', siteId: 's1', name: 'V2', createdAt: '2026-09-02T10:00:00Z' };

const buildV1: Build = {
  id: 'b1',
  siteId: 's1',
  snapshotId: 'snap1',
  environment: 'production',
  status: 'ready',
  log: ['ok'],
  artifactDir: 'build/s1/production/snap1',
  createdAt: '2026-09-01T10:00:00Z',
  finishedAt: '2026-09-01T10:00:05Z',
};

function inertSocket(): WsLike {
  return { onmessage: null, onopen: null, onclose: null, onerror: null, close: () => {} };
}

afterEach(() => {
  setBuildSocketFactory(undefined);
});

function fixture({ snapshots, builds }: { snapshots: Snapshot[]; builds: Build[] }) {
  return async (url: string, init: RequestInit) => {
    const method = init.method ?? 'GET';
    if (url === '/api/sites/s1/snapshots') {
      if (method === 'POST') return jsonResponse(201, { ...snapshots[0], id: 'snap-new' });
      return jsonResponse(200, snapshots);
    }
    if (url === '/api/sites/s1/builds') {
      if (method === 'POST') return jsonResponse(201, builds[0] ?? buildV1);
      return jsonResponse(200, builds);
    }
    if (url.startsWith('/api/snapshots/') && method === 'DELETE') return jsonResponse(204, undefined);
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

describe('SiteBuildsPage', () => {
  it('рендерит снапшоты и сборки с fetch-запросами', async () => {
    const { calls } = await renderApp({
      path: '/sites/s1/builds',
      handler: fixture({ snapshots: [snapV1, snapV2], builds: [buildV1] }),
    });

    expect(screen.getByText('Публикация')).toBeTruthy();
    expect(await screen.findByText('V1')).toBeTruthy();
    expect(screen.getByText('V2')).toBeTruthy();
    expect(screen.getByText('Текущий на prod')).toBeTruthy();

    expect(requests(calls, '/api/sites/s1/snapshots', 'GET').length).toBeGreaterThan(0);
    expect(requests(calls, '/api/sites/s1/builds', 'GET').length).toBeGreaterThan(0);
  });

  it('пустые списки показывают подсказки', async () => {
    await renderApp({ path: '/sites/s1/builds', handler: fixture({ snapshots: [], builds: [] }) });

    expect(await screen.findByText('Снапшотов нет — создайте первый')).toBeTruthy();
    expect(screen.getByText('Сборок ещё нет')).toBeTruthy();
  });

  it('публикация на prod требует ввода PROD и шлёт createBuild', async () => {
    const handlers: Array<(url: string, init: RequestInit) => Promise<Response>> = [];
    const handler = fixture({ snapshots: [snapV1, snapV2], builds: [buildV1] });

    const { calls } = await renderApp({
      path: '/sites/s1/builds',
      handler: async (url, init) => {
        const result = handler(url, init);
        handlers.push(() => result);
        return result;
      },
    });

    const pending = await screen.findByText('Публикация');
    void pending;

    const publishButtons = await screen.findAllByRole('button', { name: 'Опубликовать на prod' });
    fireEvent.click(publishButtons[0]!);

    const dialog = await screen.findByRole('alertdialog');
    expect(within(dialog).getByText('Публикация на production')).toBeTruthy();

    const reveal = within(dialog).getByLabelText('Введите PROD для подтверждения');
    const confirm = within(dialog).getByRole('button', { name: 'Опубликовать на prod' });

    fireEvent.change(reveal, { target: { value: 'prod' } });
    expect((confirm as HTMLButtonElement).disabled).toBe(true);

    fireEvent.change(reveal, { target: { value: 'PROD' } });
    expect((confirm as HTMLButtonElement).disabled).toBe(false);

    fireEvent.click(confirm);
    await waitFor(() => {
      expect(requests(calls, '/api/sites/s1/builds', 'POST').length).toBeGreaterThan(0);
    });
  });

  it('откат ре-публикует старый снапшот через createBuild', async () => {
    const currentV2: Build = {
      ...buildV1,
      id: 'b2',
      snapshotId: 'snap2',
      createdAt: '2026-09-02T10:00:00Z',
      finishedAt: '2026-09-02T10:00:05Z',
    };
    const oldV1: Build = {
      ...buildV1,
      id: 'b1-old',
      createdAt: '2026-08-30T10:00:00Z',
      finishedAt: '2026-08-30T10:00:05Z',
    };

    const { calls } = await renderApp({
      path: '/sites/s1/builds',
      handler: fixture({ snapshots: [snapV1, snapV2], builds: [oldV1, currentV2] }),
    });

    const rollback = await screen.findByRole('button', { name: 'Откатить на prod' });
    fireEvent.click(rollback);

    const dialog = await screen.findByRole('alertdialog');
    const reveal = within(dialog).getByLabelText('Введите PROD для подтверждения');
    fireEvent.change(reveal, { target: { value: 'PROD' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Откатить на prod' }));

    await waitFor(() => {
      const posts = requests(calls, '/api/sites/s1/builds', 'POST');
      expect(posts.length).toBeGreaterThan(0);
      expect(JSON.parse(posts[0]!.init.body as string)).toEqual({
        snapshotId: 'snap1',
        environment: 'production',
      });
    });
  });

  it('live-событие WS перезагружает списки', async () => {
    const sockets: WsLike[] = [];
    setBuildSocketFactory(() => {
      const s = inertSocket();
      sockets.push(s);
      return s;
    });

    const { calls } = await renderApp({
      path: '/sites/s1/builds',
      handler: fixture({ snapshots: [snapV1], builds: [] }),
    });

    await screen.findByText('Сборок ещё нет');
    const before = requests(calls, '/api/sites/s1/builds', 'GET').length;

    const event: BuildEvent = {
      siteId: 's1',
      environment: 'production',
      snapshotId: 'snap1',
      status: 'ready',
      updatedAt: '2026-09-01T10:00:06Z',
    };
    sockets[0]!.onmessage?.({ data: JSON.stringify(event) });
    await waitFor(() => {
      expect(requests(calls, '/api/sites/s1/builds', 'GET').length).toBeGreaterThan(before);
    });
  });

  it('пересборка неудачной сборки шлёт createBuild в том же окружении', async () => {
    const failed: Build = {
      ...buildV1,
      id: 'b-fail',
      environment: 'development',
      status: 'failed',
      log: ['boom'],
    };

    const { calls } = await renderApp({
      path: '/sites/s1/builds',
      handler: fixture({ snapshots: [snapV1], builds: [failed] }),
    });

    const rebuild = await screen.findByRole('button', { name: 'Пересобрать' });
    fireEvent.click(rebuild);

    await waitFor(() => {
      const posts = requests(calls, '/api/sites/s1/builds', 'POST');
      expect(posts.length).toBeGreaterThan(0);
      expect(JSON.parse(posts[0]!.init.body as string)).toEqual({
        snapshotId: 'snap1',
        environment: 'development',
      });
    });
  });

  it('создание нового снапшота отправляет POST', async () => {
    const { calls } = await renderApp({
      path: '/sites/s1/builds',
      handler: fixture({ snapshots: [snapV1], builds: [buildV1] }),
    });

    const input = await screen.findByLabelText('Новый снапшот');
    fireEvent.change(input, { target: { value: 'V3' } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    await waitFor(() => {
      const posts = requests(calls, '/api/sites/s1/snapshots', 'POST');
      expect(posts.length).toBeGreaterThan(0);
      expect(JSON.parse(posts[0]!.init.body as string)).toEqual({ name: 'V3' });
    });
  });
});