import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { SyncEngine } from '../../src/core/sync';
import { RuntimeRegistry } from '../../src/core/registry';
import { TransportFactory } from '../../src/core/transport/factory';
import { DescriptorValidationError } from '../../src/errors';
import {
  providerDescriptor1,
  providerDescriptor2,
  providerDescriptor3,
  operationDescriptor2,
  operationDescriptor6,
  operationDescriptor7,
  operationDescriptor11,
} from './fixtures';
import { FakeWebSocket, FakeEventSource, makeFakeFetch, resetFakes } from './helpers';

const BASE = Date.parse('2026-09-01T00:00:05.000Z');

function setup() {
  const registry = new RuntimeRegistry();
  for (const d of [providerDescriptor1, providerDescriptor2, providerDescriptor3]) registry.register(d);
  for (const d of [operationDescriptor2, operationDescriptor6, operationDescriptor7, operationDescriptor11]) registry.register(d);
  return registry;
}

afterEach(() => {
  resetFakes();
  vi.useRealTimers();
});

function useFakeClock(): void {
  vi.useFakeTimers({ now: BASE });
}

describe('SyncEngine', () => {
  beforeEach(() => resetFakes());

  it('poll по poll.schedule из дескриптора: каждый тик — отдельный HTTP-запрос', async () => {
    useFakeClock();
    const registry = setup();
    const { fetch, calls } = makeFakeFetch(() => ({ items: ['a', 'b'] }));
    const transport = new TransportFactory({ fetch }).create(registry.getProvider('cms')!);
    const sync = new SyncEngine(registry, transport);

    const data: unknown[] = [];
    const unsub = sync.poll<unknown>('content.poll', (d) => data.push(d), { immediate: false });

    const first = 60_000 - (BASE % 60_000); // :05 → следующая минутная граница
    expect(first).toBe(55_000);
    await vi.advanceTimersByTimeAsync(first);
    expect(calls.length).toBe(1);
    expect(calls[0].url.endsWith('/api/poll')).toBe(true);

    await vi.advanceTimersByTimeAsync(60_000);
    expect(calls.length).toBe(2);
    expect(data).toEqual([{ items: ['a', 'b'] }, { items: ['a', 'b'] }]);

    unsub();
    await vi.advanceTimersByTimeAsync(120_000);
    expect(calls.length).toBe(2);
  });

  it('poll без poll.schedule, но с opts.schedule → работает по opts.schedule', async () => {
    useFakeClock();
    const registry = setup();
    const { fetch, calls } = makeFakeFetch(() => []);
    const transport = new TransportFactory({ fetch }).create(registry.getProvider('cms')!);
    const sync = new SyncEngine(registry, transport);

    sync.poll('content.list', () => undefined, { schedule: '*/2 * * * * *', immediate: false });
    await vi.advanceTimersByTimeAsync(999); // до секунды :06 ещё не дошло
    expect(calls.length).toBe(0);
    await vi.advanceTimersByTimeAsync(1); // :06 → первый тик
    expect(calls.length).toBe(1);
    await vi.advanceTimersByTimeAsync(2000); // :08 → второй тик
    expect(calls.length).toBe(2);
  });

  it('poll без расписания вовсе → DescriptorValidationError', () => {
    const registry = setup();
    const { fetch } = makeFakeFetch(() => []);
    const transport = new TransportFactory({ fetch }).create(registry.getProvider('cms')!);
    const sync = new SyncEngine(registry, transport);

    expect(() => sync.poll('content.list', () => undefined)).toThrow(DescriptorValidationError);
  });

  it('immediate (default true): первый тик выполняется сразу', async () => {
    useFakeClock();
    const registry = setup();
    const { fetch, calls } = makeFakeFetch(() => []);
    const transport = new TransportFactory({ fetch }).create(registry.getProvider('cms')!);
    const sync = new SyncEngine(registry, transport);

    sync.poll('content.poll', () => undefined);
    await Promise.resolve();
    expect(calls.length).toBe(1);
    await vi.advanceTimersByTimeAsync(55_000); // :01:00 → второй тик
    expect(calls.length).toBe(2);
  });

  it('тик с TransportError: ошибка в errorHandler, подписка продолжает', async () => {
    useFakeClock();
    const registry = setup();
    let n = 0;
    const { fetch } = makeFakeFetch(() => {
      n++;
      if (n === 1) throw new Error('boom');
      return [];
    });
    const transport = new TransportFactory({ fetch }).create(registry.getProvider('cms')!);
    const sync = new SyncEngine(registry, transport);

    const errors: string[] = [];
    sync.poll('content.poll', () => undefined, {
      immediate: false,
      errorHandler: (e) => errors.push(e.message),
    });

    await vi.advanceTimersByTimeAsync(55_000);
    expect(errors[0]).toContain('boom');
    expect(n).toBe(1);

    await vi.advanceTimersByTimeAsync(60_000);
    expect(n).toBe(2);
  });

  it('subscribe ws-провайдера с push:true → канал ws (поток сообщений)', async () => {
    const registry = setup();
    const transport = new TransportFactory({ WebSocket: FakeWebSocket, EventSource: FakeEventSource }).create(registry.getProvider('ws')!);
    const sync = new SyncEngine(registry, transport);

    const seen: unknown[] = [];
    const unsub = sync.subscribe('ws.subscribe', (d) => seen.push(d));
    await new Promise((r) => setTimeout(r, 0));

    FakeWebSocket.instances[0].emitMessage(JSON.stringify({ id: 1 }));
    FakeWebSocket.instances[0].emitMessage(JSON.stringify({ id: 2 }));
    await Promise.resolve();
    expect(seen).toEqual([{ id: 1 }, { id: 2 }]);

    unsub(); // отписка закрывает push-канал
    unsub();
    expect(FakeWebSocket.instances[0].closed).toBe(true);
  });

  it('subscribe http-операции без push → канал poll (поллинг по cron)', async () => {
    useFakeClock();
    const registry = setup();
    const { fetch, calls } = makeFakeFetch(() => []);
    const transport = new TransportFactory({ fetch }).create(registry.getProvider('cms')!);
    const sync = new SyncEngine(registry, transport);

    sync.subscribe('content.poll', () => undefined, { immediate: false });
    await vi.advanceTimersByTimeAsync(55_000);
    expect(calls.length).toBe(1);
  });

  it('отписка канселит cron-job', async () => {
    useFakeClock();
    const registry = setup();
    const { fetch, calls } = makeFakeFetch(() => []);
    const transport = new TransportFactory({ fetch }).create(registry.getProvider('cms')!);
    const sync = new SyncEngine(registry, transport);

    const unsub = sync.poll('content.poll', () => undefined, { immediate: false });
    unsub();
    expect(sync.activeSize).toBe(0);
    await vi.advanceTimersByTimeAsync(60_000 * 5);
    expect(calls.length).toBe(0);
  });
});