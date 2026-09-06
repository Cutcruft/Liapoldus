import { beforeEach, describe, expect, it } from 'vitest';
import { createAdminApi, type AdminApi, type HttpMethod } from '../../runtime/api';
import { makeTranslate } from '../../runtime/i18n';
import { STRINGS } from '../../runtime/strings';
import { buildHtmlUrl, createPreviewStore, previewActions } from './preview-store';

const t = makeTranslate(STRINGS);

type Call = { method: HttpMethod; path: string; body?: unknown };

function fixtureApi(handler: (method: HttpMethod, path: string, body: unknown) => Response): AdminApi {
  return createAdminApi({
    baseUrl: '',
    getToken: () => null,
    fetchFn: async (input: string | URL | RequestInfo, init?: RequestInit) => {
      const method = (init?.method ?? 'GET') as HttpMethod;
      let body: unknown;
      if (init?.body && typeof init.body === 'string') {
        try {
          body = JSON.parse(init.body);
        } catch {
          body = init.body;
        }
      }
      return handler(method, String(input), body);
    },
  });
}

function json(status: number, body?: unknown): Response {
  return new Response(body === undefined ? null : JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

const READY_BUILD = (id: string) => ({
  id,
  siteId: 's1',
  snapshotId: 'snap1',
  environment: 'development',
  status: 'ready',
  log: [],
  artifactDir: 'x',
  createdAt: new Date().toISOString(),
});

describe('preview-store', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('createPreviewStore: timeout пусто → idle без url; c персистом → ready + url', () => {
    const idle = createPreviewStore('s1');
    expect(idle.getState()).toMatchObject({ status: 'idle', url: undefined, inFlight: false });

    localStorage.setItem('liapoldus.preview.s1', 'snap-x');
    const ready = createPreviewStore('s1');
    expect(ready.getState().status).toBe('ready');
    expect(ready.getState().url).toBe('/build/s1/development/snap-x/dist/index.html');
  });

  it('buildHtmlUrl формирует путь артефакта', () => {
    expect(buildHtmlUrl('s1', 'development', 'snap1')).toBe('/build/s1/development/snap1/dist/index.html');
  });

  it('requestBuild: snapshot → build → ready url + персист; ротация удаляет прошлый снапшот', async () => {
    localStorage.setItem('liapoldus.preview.s1', 'snap0');
    const calls: Call[] = [];
    const api = fixtureApi((method, path, body) => {
      calls.push({ method, path, body });
      if (method === 'POST' && path.endsWith('/snapshots')) return json(201, { id: 'snap1' });
      if (method === 'POST' && path.endsWith('/builds')) return json(201, READY_BUILD('b1'));
      if (method === 'DELETE' && path === '/api/snapshots/snap0') return json(204);
      return json(404, { error: 'not found' });
    });

    const store = createPreviewStore('s1');
    const actions = previewActions(store, { api, t, siteId: 's1' });
    await actions.requestBuild();

    expect(store.getState()).toMatchObject({ status: 'ready', error: undefined, inFlight: false });
    expect(store.getState().url).toBe('/build/s1/development/snap1/dist/index.html');
    expect(localStorage.getItem('liapoldus.preview.s1')).toBe('snap1');
    expect(calls.some((c) => c.method === 'DELETE' && c.path === '/api/snapshots/snap0')).toBe(true);
    expect(calls.find((c) => c.method === 'POST' && c.path.endsWith('/builds'))?.body).toEqual({
      snapshotId: 'snap1',
      environment: 'development',
    });
  });

  it('вторая сборка ротирует snap1 → snap2 (удаляется только прошлый)', async () => {
    localStorage.setItem('liapoldus.preview.s1', 'snap1');
    const deletes: string[] = [];
    const api = fixtureApi((method, path) => {
      if (method === 'POST' && path.endsWith('/snapshots')) return json(201, { id: 'snap2' });
      if (method === 'POST' && path.endsWith('/builds')) return json(201, READY_BUILD('b2'));
      if (method === 'DELETE') {
        deletes.push(path);
        return json(204);
      }
      return json(404, { error: 'not found' });
    });

    const store = createPreviewStore('s1');
    const actions = previewActions(store, { api, t, siteId: 's1' });
    await actions.requestBuild();

    expect(store.getState().url).toBe('/build/s1/development/snap2/dist/index.html');
    expect(deletes).toEqual(['/api/snapshots/snap1']);
  });

  it('coalescing: повторный requestBuild во время сборки → queued, после завершения запускается', async () => {
    const snapshots: string[] = [];
    const api = fixtureApi((method, path) => {
      if (method === 'POST' && path.endsWith('/snapshots')) {
        snapshots.push(path);
        return json(201, { id: `snap${snapshots.length}` });
      }
      if (method === 'POST' && path.endsWith('/builds')) return json(201, READY_BUILD('b'));
      if (method === 'DELETE') return json(204);
      return json(404, { error: 'not found' });
    });

    const store = createPreviewStore('s1');
    const actions = previewActions(store, { api, t, siteId: 's1' });
    const first = actions.requestBuild();
    const second = actions.requestBuild();

    expect(store.getState().inFlight).toBe(true);
    expect(store.getState().queued).toBe(true);
    await Promise.all([first, second]);

    expect(snapshots.length).toBe(2);
    expect(store.getState().inFlight).toBe(false);
    expect(store.getState().queued).toBe(false);
    expect(store.getState().status).toBe('ready');
    expect(store.getState().url).toBe('/build/s1/development/snap2/dist/index.html');
  });

  it('cycle пакует poll по getBuild, пока build queued/building', async () => {
    const api = fixtureApi((method, path) => {
      if (method === 'POST' && path.endsWith('/snapshots')) return json(201, { id: 'snap1' });
      if (method === 'POST' && path.endsWith('/builds')) {
        return json(201, { ...READY_BUILD('b1'), status: 'building' });
      }
      if (method === 'GET' && path === '/api/builds/b1') {
        return json(200, { ...READY_BUILD('b1'), status: 'ready' });
      }
      if (method === 'DELETE') return json(204);
      return json(404, { error: 'not found' });
    });

    const store = createPreviewStore('s1');
    const actions = previewActions(store, { api, t, siteId: 's1' });
    await actions.requestBuild();
    expect(store.getState().status).toBe('ready');
  });

  it('failed build → status failed + error из лога', async () => {
    const api = fixtureApi((method, path) => {
      if (method === 'POST' && path.endsWith('/snapshots')) return json(201, { id: 'snap1' });
      if (method === 'POST' && path.endsWith('/builds')) {
        return json(201, { ...READY_BUILD('b1'), status: 'failed', log: ['esbuild: ошибка в texts'] });
      }
      if (method === 'DELETE') return json(204);
      return json(404, { error: 'not found' });
    });

    const store = createPreviewStore('s1');
    const actions = previewActions(store, { api, t, siteId: 's1' });
    await actions.requestBuild();
    expect(store.getState().status).toBe('failed');
    expect(store.getState().error).toContain('esbuild');
  });

  it('applyEvent: не-ready игнор, ready → url + персист нового снапшота', () => {
    const store = createPreviewStore('s1');
    const actions = previewActions(
      store,
      { api: fixtureApi(() => json(404, {})), t, siteId: 's1' },
    );

    actions.applyEvent({ siteId: 's1', environment: 'development', snapshotId: 'live1', status: 'building', updatedAt: '' });
    expect(store.getState().url).toBeUndefined();

    actions.applyEvent({ siteId: 's1', environment: 'development', snapshotId: 'live1', status: 'ready', updatedAt: '' });
    expect(store.getState().status).toBe('ready');
    expect(store.getState().url).toBe('/build/s1/development/live1/dist/index.html');
    expect(localStorage.getItem('liapoldus.preview.s1')).toBe('live1');
  });
});