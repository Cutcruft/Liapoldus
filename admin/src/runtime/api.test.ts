import { describe, expect, it } from 'vitest';
import { createAdminApi } from './api';

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

function mockFetch(handler: (url: string, init: RequestInit) => Promise<Response>) {
  return async (input: string | URL | RequestInfo, init?: RequestInit) =>
    handler(String(input), init ?? {});
}

describe('createAdminApi', () => {
  it('добавляет Authorization Bearer если токен задан', async () => {
    const api = createAdminApi({
      baseUrl: 'http://admin',
      getToken: () => 'secret',
      fetchFn: mockFetch(async (_url, init) => {
        expect(init.headers).toMatchObject({ Authorization: 'Bearer secret' });
        return jsonResponse(200, []);
      }),
    });
    await api.request('GET', '/api/sites');
  });

  it('не добавляет токен при пустом', async () => {
    const api = createAdminApi({
      baseUrl: '',
      getToken: () => null,
      fetchFn: mockFetch(async (_url, init) => {
        expect((init.headers as Record<string, string>).Authorization).toBeUndefined();
        return jsonResponse(200, []);
      }),
    });
    await api.request('GET', '/api/sites');
  });

  it('GET → JSON и ok', async () => {
    const api = createAdminApi({
      baseUrl: '',
      getToken: () => null,
      fetchFn: mockFetch(async () => jsonResponse(200, [{ id: 'site1' }])),
    });
    const res = await api.request('GET', '/api/sites');
    expect(res.ok).toBe(true);
    expect(res.status).toBe(200);
    expect(res.body).toEqual([{ id: 'site1' }]);
  });

  it('400 → validation-ошибка из {error}', async () => {
    const api = createAdminApi({
      baseUrl: '',
      getToken: () => null,
      fetchFn: mockFetch(async () => jsonResponse(400, { error: 'slug already taken' })),
    });
    const res = await api.request('POST', '/api/sites', { slug: 'x' });
    expect(res.ok).toBe(false);
    expect(res.error).toEqual({
      kind: 'validation',
      status: 400,
      message: 'slug already taken',
    });
  });

  it('401 → http-ошибка', async () => {
    const api = createAdminApi({
      baseUrl: '',
      getToken: () => 'bad',
      fetchFn: mockFetch(async () => jsonResponse(401, { error: 'unauthorized' })),
    });
    const res = await api.request('GET', '/api/sites');
    expect(res.ok).toBe(false);
    expect(res.error?.kind).toBe('http');
    expect(res.error?.status).toBe(401);
  });

  it('сетевая ошибка → transport', async () => {
    const api = createAdminApi({
      baseUrl: '',
      getToken: () => null,
      fetchFn: mockFetch(async () => {
        throw new Error('ECONNREFUSED');
      }),
    });
    const res = await api.request('GET', '/api/sites');
    expect(res.ok).toBe(false);
    expect(res.error?.kind).toBe('transport');
    expect(res.error?.message).toContain('ECONNREFUSED');
  });

  it('FormData не доклеивает Content-Type', async () => {
    const api = createAdminApi({
      baseUrl: '',
      getToken: () => null,
      fetchFn: mockFetch(async (_url, init) => {
        expect((init.headers as Record<string, string>)['Content-Type']).toBeUndefined();
        expect(init.body).toBeInstanceOf(FormData);
        return jsonResponse(201, { id: 'asset1' });
      }),
    });
    const form = new FormData();
    form.append('file', new Blob(['x'], { type: 'text/plain' }), 'a.txt');
    await api.request('POST', '/api/sites/s1/assets', form);
  });
});