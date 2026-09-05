import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiClient } from '../../src/core/api-client';
import { registerBuiltin } from '../../src/core/builtin/descriptors';
import { RuntimeRegistry } from '../../src/core/registry';
import { TransportFactory } from '../../src/core/transport/factory';
import {
  OperationKindMismatchError,
  ScopeError,
  UnknownEntityError,
} from '../../src/errors';
import type { EndpointDescriptor, OperationDescriptor } from '../../src/types/descriptor';
import {
  operationDescriptor1,
  operationDescriptor2,
  operationDescriptor3,
  operationDescriptor7,
  providerDescriptor1,
  providerDescriptor3,
} from './fixtures';
import { FakeWebSocket, makeFakeFetch, resetFakes } from './helpers';

const BASE = Date.parse('2026-09-01T00:00:05.000Z');

const articlesListOp: OperationDescriptor = {
  kind: 'operation',
  id: 'articles.list',
  typeOp: 'query',
  providerId: 'cms',
  method: 'GET',
  path: '/articles',
  params: { in: 'query' },
  cache: 'disabled',
};

const contactSubmitOp: OperationDescriptor = {
  kind: 'operation',
  id: 'contact.submit',
  typeOp: 'mutation',
  providerId: 'cms',
  method: 'POST',
  path: '/contact/submit',
  params: { in: 'body' },
  cache: 'disabled',
  scope: 'server',
};

const contactSubmitEp: EndpointDescriptor = {
  kind: 'endpoint',
  id: 'contact.submit',
  path: '/contact/submit',
  method: 'POST',
  operationId: 'contact.submit',
};

function httpRegistry(): RuntimeRegistry {
  const r = new RuntimeRegistry();
  r.register(providerDescriptor1);
  for (const d of [
    operationDescriptor1,
    operationDescriptor2,
    operationDescriptor3,
    articlesListOp,
    contactSubmitOp,
  ]) {
    r.register(d);
  }
  r.register(contactSubmitEp);
  return r;
}

function cmsTransport(registry: RuntimeRegistry, fetch: ReturnType<typeof makeFakeFetch>['fetch']) {
  return new TransportFactory({ fetch }).create(registry.getProvider('cms')!);
}

afterEach(() => {
  resetFakes();
  vi.useRealTimers();
});

beforeEach(() => resetFakes());

describe('ApiClient', () => {
  it('query проходит через транспорт и возвращает результат', async () => {
    const registry = httpRegistry();
    const { fetch, calls } = makeFakeFetch(() => [{ id: 1 }]);
    const api = new ApiClient(registry, { transport: cmsTransport(registry, fetch) });

    const res = await api.query('articles.list', { page: 2 });
    expect(res).toEqual([{ id: 1 }]);
    expect(calls.length).toBe(1);
    expect(calls[0].url).toContain('/articles?page=2');
  });

  it('query неизвестной операции → UnknownEntityError', async () => {
    const registry = httpRegistry();
    const { fetch } = makeFakeFetch(() => null);
    const api = new ApiClient(registry, { transport: cmsTransport(registry, fetch) });

    await expect(api.query('unknown')).rejects.toThrow(UnknownEntityError);
  });

  it('mutate для query-операции → OperationKindMismatchError', async () => {
    const registry = httpRegistry();
    const { fetch } = makeFakeFetch(() => null);
    const api = new ApiClient(registry, { transport: cmsTransport(registry, fetch) });

    await expect(api.mutate('articles.list')).rejects.toThrow(OperationKindMismatchError);
  });

  it('query для mutation-операции → OperationKindMismatchError', async () => {
    const registry = httpRegistry();
    const { fetch } = makeFakeFetch(() => null);
    const api = new ApiClient(registry, { transport: cmsTransport(registry, fetch) });

    await expect(api.query('contact.submit')).rejects.toThrow(OperationKindMismatchError);
  });

  it('callEndpoint (server-контекст) возвращает результат server-операции', async () => {
    const registry = httpRegistry();
    const { fetch, calls } = makeFakeFetch(() => ({ submissionId: 's1', state: 'ok' }));
    const api = new ApiClient(registry, {
      transport: cmsTransport(registry, fetch),
      scope: 'server',
    });

    const res = await api.callEndpoint('contact.submit', { name: 'x' });
    expect(res).toEqual({ submissionId: 's1', state: 'ok' });
    expect(calls.length).toBe(1);
  });

  it('callEndpoint для несуществующего endpointe → UnknownEntityError', async () => {
    const registry = httpRegistry();
    const { fetch } = makeFakeFetch(() => null);
    const api = new ApiClient(registry, {
      transport: cmsTransport(registry, fetch),
      scope: 'server',
    });

    await expect(api.callEndpoint('articles.list')).rejects.toThrow(UnknownEntityError);
  });

  it('callEndpoint из public-контекста → ScopeError', async () => {
    const registry = httpRegistry();
    const { fetch } = makeFakeFetch(() => null);
    const api = new ApiClient(registry, { transport: cmsTransport(registry, fetch) });

    await expect(api.callEndpoint('contact.submit', {})).rejects.toThrow(ScopeError);
  });

  it('builtin.content.batch / asset.get / form.get делегируют в операции', async () => {
    const registry = new RuntimeRegistry();
    registerBuiltin(registry);
    const { fetch } = makeFakeFetch((call) => {
      if (call.url.includes('/contents/batch')) return { batched: true };
      if (call.url.includes('/assets/a1')) {
        return { id: 'a1', name: 'A', type: 'image/png', size: 10, variants: [{ name: 'master', url: '/a1.png' }] };
      }
      if (call.url.includes('/forms/f1')) return { id: 'f1', fields: {} };
      return null;
    });
    const transport = new TransportFactory({ fetch }).create(registry.getProvider('liapoldus.builtin')!);
    const api = new ApiClient(registry, { transport });

    const batch = await api.builtin.content.batch('site1', { ids: ['a', 'b'] });
    expect(batch).toEqual({ batched: true });

    const asset = await api.builtin.asset.get('a1');
    expect(asset.id).toBe('a1');
    expect(asset.variants[0].name).toBe('master');

    const form = await api.builtin.form.get('f1');
    expect(form.id).toBe('f1');
  });

  it('builtin.form.submit вне server-контекста → ScopeError', async () => {
    const registry = new RuntimeRegistry();
    registerBuiltin(registry);
    const { fetch } = makeFakeFetch(() => ({ submissionId: 's1', status: 'ok' }));
    const transport = new TransportFactory({ fetch }).create(registry.getProvider('liapoldus.builtin')!);
    const api = new ApiClient(registry, { transport });

    await expect(api.builtin.form.submit({ formId: 'f1', values: { email: 'a@b.c' } })).rejects.toThrow(ScopeError);
  });

  it('query с cache: immutable — второй вызов с тем же input берётся из кэша', async () => {
    const registry = httpRegistry();
    let n = 0;
    const { fetch, calls } = makeFakeFetch(() => {
      n++;
      return { content: `x${n}` };
    });
    const api = new ApiClient(registry, { transport: cmsTransport(registry, fetch) });

    const first = await api.query('content.get', { id: 'a', locale: 'ru' });
    const second = await api.query('content.get', { id: 'a', locale: 'ru' });
    expect(second).toEqual(first);
    expect(calls.length).toBe(1);
    expect(n).toBe(1);

    await api.query('content.get', { id: 'b', locale: 'ru' });
    expect(calls.length).toBe(2);
  });

  it('query с cache: disabled — каждый вызов делает запрос', async () => {
    const registry = httpRegistry();
    const { fetch, calls } = makeFakeFetch(() => []);
    const api = new ApiClient(registry, { transport: cmsTransport(registry, fetch) });

    await api.query('articles.list', { page: 1 });
    await api.query('articles.list', { page: 1 });
    expect(calls.length).toBe(2);
  });

  it('poll стартует short polling (запросы с ?locale) и отписывается', async () => {
    vi.useFakeTimers({ now: BASE });
    const registry = httpRegistry();
    const { fetch, calls } = makeFakeFetch(() => ({ items: [] }));
    const api = new ApiClient(registry, { transport: cmsTransport(registry, fetch) });

    const seen: unknown[] = [];
    const unsub = api.poll('content.get', (d) => seen.push(d), {
      schedule: '*/2 * * * * *',
      input: { id: 'a', locale: 'ru' },
      immediate: false,
    });

    await vi.advanceTimersByTimeAsync(999);
    expect(calls.length).toBe(0);

    await vi.advanceTimersByTimeAsync(1);
    expect(calls.length).toBe(1);
    expect(calls[0].url).toContain('locale=ru');

    await vi.advanceTimersByTimeAsync(2000);
    expect(calls.length).toBe(2);
    expect(seen.length).toBe(2);

    unsub();
    await vi.advanceTimersByTimeAsync(10_000);
    expect(calls.length).toBe(2);
  });

  it('subscribe с push:true → push-канал; функция отписывает', async () => {
    const registry = new RuntimeRegistry();
    registry.register(providerDescriptor3);
    registry.register(operationDescriptor7);
    const transport = new TransportFactory({ WebSocket: FakeWebSocket }).create(registry.getProvider('ws')!);
    const api = new ApiClient(registry, { transport });

    const seen: unknown[] = [];
    const unsub = api.subscribe('ws.subscribe', (d) => seen.push(d));
    await new Promise((r) => setTimeout(r, 0));

    FakeWebSocket.instances[0].emitMessage(JSON.stringify({ n: 1 }));
    FakeWebSocket.instances[0].emitMessage(JSON.stringify({ n: 2 }));
    await Promise.resolve();
    expect(seen).toEqual([{ n: 1 }, { n: 2 }]);

    unsub();
    expect(FakeWebSocket.instances[0].closed).toBe(true);
  });
});