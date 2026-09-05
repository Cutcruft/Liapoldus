import { describe, it, expect, beforeEach, vi } from 'vitest';
import { HttpTransport } from '../../src/core/transport/http';
import { RuntimeRegistry } from '../../src/core/registry';
import { TransportError } from '../../src/errors';
import {
  providerDescriptor1,
  operationDescriptor1,
  operationDescriptor3,
  operationDescriptor4,
  operationDescriptor2,
} from './fixtures';
import { makeFakeFetch, fakeFetchNever, type FetchCall } from './helpers';

function flattenHeaders(init: RequestInit): Record<string, string> {
  return { ...(init.headers as Record<string, string>) };
}

describe('HttpTransport', () => {
  let registry: RuntimeRegistry;

  beforeEach(() => {
    registry = new RuntimeRegistry();
    registry.register(providerDescriptor1);
    registry.register(operationDescriptor1);
    registry.register(operationDescriptor2);
    registry.register(operationDescriptor3);
    registry.register(operationDescriptor4);
  });

  it('query-операция: параметры в path-шаблон + query-string', async () => {
    const { fetch, calls } = makeFakeFetch(() => ({ id: '123', title: 'Хедер' }));
    const transport = new HttpTransport(registry.getProvider('cms')!, { fetch });
    const provider = registry.getProvider('cms')!;
    const res = await transport.request({
      provider,
      operation: registry.getOperation('content.get')!,
      params: { id: '123', locale: 'ru' },
    });
    expect(calls[0].url).toBe('https://cms.example.com/api/contents/123?locale=ru');
    expect(calls[0].init.method).toBe('GET');
    expect(res.body).toEqual({ id: '123', title: 'Хедер' });
  });

  it('query-операция с несколькими query-параметрами', async () => {
    const { fetch, calls } = makeFakeFetch(() => []);
    const transport = new HttpTransport(registry.getProvider('cms')!, { fetch });
    const provider = registry.getProvider('cms')!;
    await transport.request({
      provider,
      operation: registry.getOperation('content.list')!,
      params: { locale: 'ru', cursor: 'abc', pageSize: 20 },
    });
    expect(calls[0].url).toBe('https://cms.example.com/api/contents?locale=ru&cursor=abc&pageSize=20');
  });

  it('mutation path-операция: параметр в path, тело в json', async () => {
    const { fetch, calls } = makeFakeFetch(() => ({ ok: true }));
    const transport = new HttpTransport(registry.getProvider('cms')!, { fetch });
    const provider = registry.getProvider('cms')!;
    await transport.request({
      provider,
      operation: registry.getOperation('site.register')!,
      params: { input: { email: 'a@b.c' } },
    });
    expect(calls[0].url).toBe('https://cms.example.com/api/register');
    expect(calls[0].init.method).toBe('POST');
    expect(JSON.parse(calls[0].init.body as string)).toEqual({ email: 'a@b.c' });
  });

  it('body-операция: слияние headers (defaults → operation → request)', async () => {
    const callHeaders: Record<string, string> = {};
    let sentBody: string | undefined;
    const { fetch } = makeFakeFetch((call: FetchCall) => {
      Object.assign(callHeaders, flattenHeaders(call.init));
      sentBody = call.init.body as string;
      return [];
    });
    const provider = registry.getProvider('cms')!;
    const opWithHeaders = { ...operationDescriptor4, headers: { 'X-Operation': 'op-data' } };
    const transport = new HttpTransport(provider, { fetch });
    const res = await transport.request({
      provider,
      operation: opWithHeaders as never,
      params: { input: { ids: [1, 2] } },
      headers: { 'X-Request': 'req-head', 'X-App-Version': '3' },
    });
    expect(res.body).toEqual([]);
    expect(callHeaders['X-App-Version']).toBe('3');
    expect(callHeaders['X-Operation']).toBe('op-data');
    expect(callHeaders['X-Request']).toBe('req-head');
    expect(JSON.parse(sentBody!)).toEqual({ ids: [1, 2] });
  });

  it('таймаут из defaults.provider (40мс) → TransportError', async () => {
    vi.useFakeTimers();
    try {
      const transport = new HttpTransport(registry.getProvider('cms')!, { fetch: fakeFetchNever() });
      const provider = registry.getProvider('cms')!;
      const promise = transport.request({ provider, operation: registry.getOperation('content.get')!, params: { id: '1' } });
      const assertion = expect(promise).rejects.toBeInstanceOf(TransportError);
      await vi.advanceTimersByTimeAsync(50);
      await assertion;
    } finally {
      vi.useRealTimers();
    }
  });

  it('сетевой сбой → TransportError', async () => {
    const { fetch } = makeFakeFetch(() => new TypeError('network down'));
    const transport = new HttpTransport(registry.getProvider('cms')!, { fetch });
    const provider = registry.getProvider('cms')!;
    await expect(transport.request({ provider, operation: registry.getOperation('content.get')!, params: { id: '1' } })).rejects.toBeInstanceOf(TransportError);
  });

  it('не-2xx ответ → TransportError с code=transport_error и status', async () => {
    const { fetch } = makeFakeFetch(() => {
      return {
        ok: false,
        status: 500,
        async json() {
          return { error: 'boom' };
        },
        async text() {
          return '{"error":"boom"}';
        },
      } as never;
    });
    const transport = new HttpTransport(registry.getProvider('cms')!, { fetch });
    const provider = registry.getProvider('cms')!;
    const promise = transport.request({ provider, operation: registry.getOperation('content.get')!, params: { id: '1' } });
    await expect(promise).rejects.toMatchObject({
      code: 'transport_error',
      cause: { status: 500, httpError: true },
    });
  });

  it('GET без params: URL без query-string', async () => {
    const { fetch, calls } = makeFakeFetch(() => []);
    const transport = new HttpTransport(registry.getProvider('cms')!, { fetch });
    const provider = registry.getProvider('cms')!;
    await transport.request({ provider, operation: registry.getOperation('content.get')!, params: { id: 'x' } });
    expect(calls[0].url).toBe('https://cms.example.com/api/contents/x');
  });

  it('provider без baseUrl → относительный путь (тот же origin)', async () => {
    registry.register({ kind: 'provider', id: 'local', protocol: 'http' });
    registry.register({ ...operationDescriptor1, id: 'local.q', providerId: 'local' });
    const { fetch, calls } = makeFakeFetch(() => ({ ok: true }));
    const transport = new HttpTransport(registry.getProvider('local')!, { fetch });
    const provider = registry.getProvider('local')!;
    await transport.request({ provider, operation: registry.getOperation('local.q')!, params: { id: '1' } });
    expect(calls[0].url).toBe('/contents/1');
  });
});