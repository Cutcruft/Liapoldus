import { describe, it, expect, beforeEach } from 'vitest';
import { TransportFactory } from '../../src/core/transport/factory';
import { HttpTransport } from '../../src/core/transport/http';
import { WebSocketTransport, SseTransport } from '../../src/core/transport/ws-sse';
import { GraphqlTransport } from '../../src/core/transport/graphql';
import { RuntimeRegistry } from '../../src/core/registry';
import { providerDescriptor1, providerDescriptor2, providerDescriptor3, providerDescriptor4 } from './fixtures';
import { FakeEventSource, FakeWebSocket, resetFakes, makeFakeFetch } from './helpers';

describe('TransportFactory', () => {
  let registry: RuntimeRegistry;

  beforeEach(() => {
    resetFakes();
    registry = new RuntimeRegistry();
  });

  it('распределяет провайдера по протоколу на транспорт', () => {
    registry.register(providerDescriptor1);
    registry.register(providerDescriptor2);
    registry.register(providerDescriptor3);
    registry.register(providerDescriptor4);
    const factory = new TransportFactory({ WebSocket: FakeWebSocket, EventSource: FakeEventSource });
    expect(factory.create(registry.getProvider('cms')!)).toBeInstanceOf(HttpTransport);
    expect(factory.create(registry.getProvider('sse')!)).toBeInstanceOf(SseTransport);
    expect(factory.create(registry.getProvider('ws')!)).toBeInstanceOf(WebSocketTransport);
    expect(factory.create(registry.getProvider('gql')!)).toBeInstanceOf(GraphqlTransport);
  });

  it('ксрирует транспорт по provider.id', () => {
    registry.register(providerDescriptor1);
    const factory = new TransportFactory({ WebSocket: FakeWebSocket, EventSource: FakeEventSource });
    const provider = registry.getProvider('cms')!;
    expect(factory.create(provider)).toBe(factory.create(provider));
  });

  it('HTTP-транспорт ходит через инжектированный fetch', async () => {
    registry.register(providerDescriptor1);
    const { fetch, calls } = makeFakeFetch(() => ({ echo: true }));
    const factory = new TransportFactory({ fetch, WebSocket: FakeWebSocket, EventSource: FakeEventSource });
    const transport = factory.create(registry.getProvider('cms')!);
    const provider = registry.getProvider('cms')!;
    await transport.request({
      provider,
      operation: {
        id: 'probe',
        provider,
        typeOp: 'query',
        cache: 'disabled',
        scope: 'public',
        path: '/probe',
        method: 'GET',
        state: { registered: true },
      },
      params: { a: 1 },
    });
    expect(calls.length).toBe(1);
    expect(calls[0].url).toBe('https://cms.example.com/api/probe?a=1');
  });
});