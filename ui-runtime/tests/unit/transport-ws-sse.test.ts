import { describe, it, expect, beforeEach, vi } from 'vitest';
import { WebSocketTransport, SseTransport } from '../../src/core/transport/ws-sse';
import { RuntimeRegistry } from '../../src/core/registry';
import { TransportError, SubscriptionError } from '../../src/errors';
import { providerDescriptor2, providerDescriptor3, operationDescriptor6, operationDescriptor7 } from './fixtures';
import { FakeWebSocket, FakeEventSource, resetFakes } from './helpers';

describe('WebSocketTransport', () => {
  let registry: RuntimeRegistry;

  beforeEach(() => {
    resetFakes();
    registry = new RuntimeRegistry();
    registry.register(providerDescriptor3);
    registry.register(operationDescriptor7);
  });

  it('подключается по url из operation.subscribe', async () => {
    const transport = new WebSocketTransport(registry.getProvider('ws')!, FakeWebSocket);
    const provider = registry.getProvider('ws')!;
    const promise = transport.request({ provider, operation: registry.getOperation('ws.subscribe')! });
    await new Promise((r) => setTimeout(r, 0));
    const ws = FakeWebSocket.instances[0];
    expect(ws.url).toBe('wss://ws.example.com/events');
    ws.emitMessage(JSON.stringify({ deviceId: 'd1', online: true }));
    const res = await promise;
    expect(res.ok).toBe(true);
    expect(res.body).toEqual({ deviceId: 'd1', online: true });
  });

  it('закрывает открытый сокет через close()', async () => {
    const transport = new WebSocketTransport(registry.getProvider('ws')!, FakeWebSocket);
    const provider = registry.getProvider('ws')!;
    const promise = transport.request({ provider, operation: registry.getOperation('ws.subscribe')! });
    await new Promise((r) => setTimeout(r, 0));
    transport.close();
    expect(FakeWebSocket.instances[0].closed).toBe(true);
    await expect(promise).rejects.toBeInstanceOf(TransportError);
  });

  it('ошибка сокета → TransportError', async () => {
    const transport = new WebSocketTransport(registry.getProvider('ws')!, FakeWebSocket);
    const provider = registry.getProvider('ws')!;
    const promise = transport.request({ provider, operation: registry.getOperation('ws.subscribe')! });
    await new Promise((r) => setTimeout(r, 0));
    FakeWebSocket.instances[0].emitError('boom');
    await expect(promise).rejects.toBeInstanceOf(TransportError);
  });

  it('без url → TransportError', async () => {
    registry.register({ kind: 'provider', id: 'ws-nourl', protocol: 'ws' });
    const provider = registry.getProvider('ws-nourl')!;
    const transport = new WebSocketTransport(provider, FakeWebSocket);
    await expect(
      transport.request({
        provider,
        operation: {
          id: 'x',
          provider,
          typeOp: 'query',
          cache: 'disabled',
          scope: 'public',
          state: { registered: true },
        },
      }),
    ).rejects.toBeInstanceOf(TransportError);
  });
});

describe('SseTransport', () => {
  let registry: RuntimeRegistry;

  beforeEach(() => {
    resetFakes();
    registry = new RuntimeRegistry();
    registry.register(providerDescriptor2);
    registry.register(operationDescriptor6);
  });

  it('подписывается на событие из provider.subscribe и резолвит сообщение', async () => {
    const transport = new SseTransport(FakeEventSource as never);
    const provider = registry.getProvider('sse')!;
    const promise = transport.request({ provider, operation: registry.getOperation('sse.subscribe')! });
    await new Promise((r) => setTimeout(r, 0));
    const es = FakeEventSource.instances[0];
    expect(es.url).toBe('https://live.example.com/events');
    es.emitMessage(JSON.stringify({ id: 42, value: 'up' }));
    const res = await promise;
    expect(res.ok).toBe(true);
    expect(res.body).toEqual({ id: 42, value: 'up' });
  });

  it('ошибка SSE → SubscriptionError', async () => {
    const transport = new SseTransport(FakeEventSource as never);
    const provider = registry.getProvider('sse')!;
    const promise = transport.request({ provider, operation: registry.getOperation('sse.subscribe')! });
    await new Promise((r) => setTimeout(r, 0));
    FakeEventSource.instances[0].emitError();
    await expect(promise).rejects.toBeInstanceOf(SubscriptionError);
  });

  it('таймаут подписки → SubscriptionError', async () => {
    vi.useFakeTimers();
    try {
      const transport = new SseTransport(FakeEventSource as never);
      const provider = registry.getProvider('sse')!;
      const promise = transport.request({ provider, operation: registry.getOperation('sse.subscribe')!, timeoutMs: 40 });
      const assertion = expect(promise).rejects.toBeInstanceOf(SubscriptionError);
      await vi.advanceTimersByTimeAsync(50);
      await assertion;
    } finally {
      vi.useRealTimers();
    }
  });

  it('операция без url (транспорт-гард) → SubscriptionError', async () => {
    const provider = {
      id: 'sse-nourl',
      protocol: 'sse' as const,
      state: { registered: true },
    };
    const operation = {
      id: 'sse.nourl',
      provider,
      typeOp: 'query' as const,
      cache: 'disabled' as const,
      scope: 'public' as const,
      state: { registered: true },
    };
    const transport = new SseTransport(FakeEventSource as never);
    await expect(transport.request({ provider: provider as never, operation: operation as never })).rejects.toBeInstanceOf(SubscriptionError);
  });
});