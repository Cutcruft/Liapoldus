import { describe, expect, it } from 'vitest';
import { createAdminApi, type AdminApi } from './api';
import { OPERATIONS, operationByKind, runOperation } from './operations';
import { makeTranslate } from './i18n';
import { STRINGS } from './strings';

const t = makeTranslate(STRINGS);

function fixtureApi(handler: (url: string, init: RequestInit) => Promise<Response>): AdminApi {
  return createAdminApi({
    baseUrl: '',
    getToken: () => null,
    fetchFn: async (input: string | URL | RequestInfo, init?: RequestInit) =>
      handler(String(input), init ?? {}),
  });
}

function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

describe('operations: реестр', () => {
  it('содержит все createPage/saveTree/runtimeStatus и ищется по kind', () => {
    expect(operationByKind('createSite').labelKey).toBe('op.createSite');
    expect(operationByKind('saveTree').route({ pageId: 'p' })).toBe('/api/pages/{pageId}/tree');
    expect(operationByKind('runtimeStatus').route({})).toBe('/api/runtime/status');
  });

  it('неизвестный kind бросает', () => {
    expect(() => operationByKind('nope' as never)).toThrow();
  });
});

describe('runOperation', () => {
  it('listSites: count в detail', async () => {
    const api = fixtureApi(async (url) => {
      expect(url).toBe('/api/sites');
      return json(200, [{ id: 's1' }, { id: 's2' }]);
    });
    const res = await runOperation(api, 'listSites', {}, t);
    expect(res.ok).toBe(true);
    expect(res.label).toBe('Список сайтов');
    expect(res.detail).toBe('2 шт.');
    expect(res.data).toEqual([{ id: 's1' }, { id: 's2' }]);
  });

  it('createSite: POST → тело и label/detail', async () => {
    const api = fixtureApi(async (url, init) => {
      expect(url).toBe('/api/sites');
      expect(init.method).toBe('POST');
      expect(JSON.parse(init.body as string)).toEqual({
        name: 'Demo',
        slug: 'demo',
        defaultLocale: 'ru',
        hosts: ['demo.example.com'],
      });
      return json(201, { id: 'site1', name: 'Demo', slug: 'demo', defaultLocale: 'ru', hosts: [] });
    });
    const res = await runOperation(api, 'createSite', { name: 'Demo', slug: 'demo', hosts: ['demo.example.com'] }, t);
    expect(res.ok).toBe(true);
    expect(res.detail).toBe('Создан «Demo»');
  });

  it('saveTree: версия из ответа', async () => {
    const api = fixtureApi(async (url, init) => {
      expect(url).toBe('/api/pages/p1/tree');
      expect(init.method).toBe('PUT');
      expect(JSON.parse(init.body as string)).toEqual({ root: { id: 'root', type: 'Container' } });
      return json(200, { id: 'page1', version: 7 });
    });
    const res = await runOperation(api, 'saveTree', { pageId: 'p1', root: { id: 'root', type: 'Container' } }, t);
    expect(res.detail).toBe('Сохранено, версия 7');
  });

  it('runtimeStatus: 404 → miss', async () => {
    const api = fixtureApi(async () => json(404, { error: 'not found' }));
    const res = await runOperation(api, 'runtimeStatus', {}, t);
    expect(res.ok).toBe(false);
    expect(res.detail).toBe('Рантайм не отвечает');
    expect(res.error?.status).toBe(404);
  });

  it('ошибка → error-результат с label', async () => {
    const api = fixtureApi(async () => json(400, { error: 'bad slug' }));
    const res = await runOperation(api, 'createSite', { name: 'x', slug: 'x' }, t);
    expect(res.ok).toBe(false);
    expect(res.status).toBe('error');
    expect(res.label).toBe('Создать сайт');
    expect(res.error?.kind).toBe('validation');
  });
});

describe('OPERS_PATH (pathWithQuery через executor)', () => {
  it('подставляет siteId и убирает его из query', async () => {
    const api = fixtureApi(async (url) => {
      expect(url).toBe('/api/sites/s1/contents?collectionId=main');
      return json(200, []);
    });
    const path = api.pathWithQuery(
      OPERATIONS.listContents.route({ siteId: 's1' }),
      { siteId: 's1' },
      { collectionId: 'main' },
    );
    expect(path).toBe('/api/sites/s1/contents?collectionId=main');
  });
});