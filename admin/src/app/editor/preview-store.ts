import { createSliceStore, type SliceStore } from '@liapoldus/ui-runtime';
import type { AdminApi, Build, BuildStatus, DevRebuildEvent, Translate } from '../../runtime';
import { runOperation } from '../../runtime';

export type PreviewStatus = 'idle' | 'building' | 'ready' | 'failed';

export type PreviewState = {
  status: PreviewStatus;
  url?: string;
  error?: string;
  /** Идёт сетевой цикл (snapshot → build → poll); запросы складываются в `queued`. */
  inFlight: boolean;
  queued: boolean;
  buildStatus?: BuildStatus;
};

export const ENV_DEV = 'development';

export type PreviewActions = {
  requestBuild: () => Promise<void>;
  applyEvent: (event: DevRebuildEvent) => void;
  clear: () => void;
};

const POLL_MS = 400;

function storageKey(siteId: string): string {
  return `liapoldus.preview.${siteId}`;
}

export function readPersistedSnapshot(siteId: string): string | undefined {
  if (typeof localStorage === 'undefined') return undefined;
  return localStorage.getItem(storageKey(siteId)) ?? undefined;
}

function persistSnapshot(siteId: string, snapshotId: string): void {
  if (typeof localStorage === 'undefined') return;
  localStorage.setItem(storageKey(siteId), snapshotId);
}

function clearPersistedSnapshot(siteId: string): void {
  if (typeof localStorage === 'undefined') return;
  localStorage.removeItem(storageKey(siteId));
}

export function buildHtmlUrl(siteId: string, environment: string, snapshotId: string): string {
  return `/build/${siteId}/${environment}/${snapshotId}/dist/index.html`;
}

/** Превью-стор для сайта: восстанавливает последний собранный снапшот из localStorage. */
export function createPreviewStore(siteId: string): SliceStore<PreviewState> {
  const persisted = readPersistedSnapshot(siteId);
  return createSliceStore<PreviewState>({
    status: persisted ? 'ready' : 'idle',
    url: persisted ? buildHtmlUrl(siteId, ENV_DEV, persisted) : undefined,
    inFlight: false,
    queued: false,
  });
}

async function sleep(ms: number): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve, ms));
}

/** Сборка превью для текущего состояния сайта (автосборка после save + ручная). */
export function previewActions(
  store: SliceStore<PreviewState>,
  deps: { api: AdminApi; t: Translate; siteId: string },
): PreviewActions {
  const { api, t, siteId } = deps;
  const set = (patch: Partial<PreviewState>) => store.setState(patch);

  const pollUntilDone = async (initial: Build): Promise<Build> => {
    let build = initial;
    while (build.status === 'queued' || build.status === 'building') {
      await sleep(POLL_MS);
      const res = await runOperation(api, 'getBuild', { buildId: build.id }, t);
      if (!res.ok) throw new Error(res.detail);
      build = res.data as Build;
    }
    return build;
  };

  /** best-effort: удаляем прошлый превью-снапшот сайта (ротация, история не мусорится). */
  const rotatePreview = async (previousId?: string): Promise<void> => {
    if (!previousId) return;
    await runOperation(api, 'deleteSnapshot', { snapshotId: previousId }, t);
  };

  const cycle = async (): Promise<void> => {
    const previousId = readPersistedSnapshot(siteId);
    set({ status: 'building', error: undefined, buildStatus: 'queued' });

    const snapRes = await runOperation(
      api,
      'createSnapshot',
      { siteId, name: `Dev preview ${new Date().toISOString().slice(0, 19).replace('T', ' ')}` },
      t,
    );
    if (!snapRes.ok) throw new Error(snapRes.detail);
    const snapshot = snapRes.data as { id: string };
    persistSnapshot(siteId, snapshot.id);

    const buildRes = await runOperation(
      api,
      'createBuild',
      { siteId, snapshotId: snapshot.id, environment: ENV_DEV },
      t,
    );
    if (!buildRes.ok) throw new Error(buildRes.detail);
    const done = await pollUntilDone(buildRes.data as Build);
    if (done.status === 'failed') {
      throw new Error(done.log.length > 0 ? done.log[done.log.length - 1]! : 'build failed');
    }

    set({ status: 'ready', url: buildHtmlUrl(siteId, ENV_DEV, snapshot.id), buildStatus: 'ready' });
    await rotatePreview(previousId);
  };

  const requestBuild = (): Promise<void> => {
    if (store.getState().inFlight) {
      set({ queued: true });
      return Promise.resolve();
    }
    set({ inFlight: true, queued: false });
    const runner = async (): Promise<void> => {
      try {
        await cycle();
      } catch (err) {
        set({ status: 'failed', error: err instanceof Error ? err.message : String(err) });
      } finally {
        set({ inFlight: false });
        if (store.getState().queued) {
          set({ queued: false });
          await runner();
        }
      }
    };
    return runner();
  };

  const applyEvent = (event: DevRebuildEvent): void => {
    if (event.status !== 'ready') return;
    persistSnapshot(siteId, event.snapshotId);
    set({
      status: 'ready',
      url: buildHtmlUrl(siteId, event.environment, event.snapshotId),
      error: undefined,
      buildStatus: 'ready',
    });
  };

  const clear = (): void => {
    clearPersistedSnapshot(siteId);
    set({ status: 'idle', url: undefined, error: undefined, buildStatus: undefined });
  };

  return { requestBuild, applyEvent, clear };
}