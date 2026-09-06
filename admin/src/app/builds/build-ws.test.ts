import { afterEach, describe, expect, it, vi } from 'vitest';
import { connectBuildWs, setBuildSocketFactory, type WsLike } from './build-ws';
import type { BuildEvent } from '../../runtime';

function inertSocket(): WsLike {
  return { onmessage: null, onopen: null, onclose: null, onerror: null, close: () => {} };
}

afterEach(() => {
  setBuildSocketFactory(undefined);
  vi.useRealTimers();
});

describe('connectBuildWs', () => {
  it('URL включает siteId и token; JSON-события доходят до подписчика', () => {
    const sockets: WsLike[] = [];
    setBuildSocketFactory((url) => {
      const s = inertSocket();
      sockets.push(s);
      return s;
    });

    const onEvent = vi.fn();
    const off = connectBuildWs('s1', 'tok-123', onEvent);
    const ws = sockets[0]!;
    expect(ws).toBeDefined();
    ws.onmessage?.({
      data: JSON.stringify({ siteId: 's1', environment: 'production', snapshotId: 'snap1', status: 'ready', updatedAt: 'x' } satisfies BuildEvent),
    });
    ws.onmessage?.({ data: 'not json' });

    expect(onEvent).toHaveBeenCalledTimes(1);
    expect(onEvent.mock.calls[0]![0]).toMatchObject({ status: 'ready', snapshotId: 'snap1' });
    off();
  });

  it('URL идёт в /api/builds/ws с query siteId и token', () => {
    const created: string[] = [];
    setBuildSocketFactory((url) => {
      created.push(url);
      return inertSocket();
    });

    const off = connectBuildWs('s 1', 't', () => {});
    expect(created[0]).toContain('/api/builds/ws?siteId=s%201');
    expect(created[0]).toContain('token=t');
    off();
  });

  it('переподключение с экспоненциальным бэкоффом', () => {
    vi.useFakeTimers();
    const created: string[] = [];
    const sockets: WsLike[] = [];
    setBuildSocketFactory((url) => {
      created.push(url);
      const s = inertSocket();
      sockets.push(s);
      return s;
    });

    const off = connectBuildWs('s1', 't', () => {});
    sockets[0]!.onclose?.();
    vi.advanceTimersByTime(999);
    expect(created.length).toBe(1);
    vi.advanceTimersByTime(1);
    expect(created.length).toBe(2);

    sockets[1]!.onclose?.();
    vi.advanceTimersByTime(2000);
    expect(created.length).toBe(3);
    off();
  });

  it('cleanup закрывает сокет и не переподключается', () => {
    vi.useFakeTimers();
    const sockets: WsLike[] = [];
    setBuildSocketFactory(() => {
      const s = { ...inertSocket(), close: vi.fn(() => {}) };
      sockets.push(s);
      return s;
    });

    const off = connectBuildWs('s1', 't', () => {});
    const ws = sockets[0]!;
    off();
    ws.onclose?.();
    vi.advanceTimersByTime(60_000);
    expect(sockets[0]!.close).toHaveBeenCalled();
    expect(sockets.length).toBe(1);
  });

  it('пустой siteId → без сокета, cleanup безопасен', () => {
    const factory = vi.fn();
    setBuildSocketFactory(factory);
    const off = connectBuildWs('', 't', () => {});
    expect(factory).not.toHaveBeenCalled();
    off();
  });
});