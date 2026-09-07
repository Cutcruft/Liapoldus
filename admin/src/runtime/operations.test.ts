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

function noBody(status: number): Response {
  return new Response(null, { status });
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

  it('listBuilds: count в detail', async () => {
    const api = fixtureApi(async (url) => {
      expect(url).toBe('/api/sites/s1/builds');
      return json(200, [
        { id: 'b1', siteId: 's1', snapshotId: 'snap1', environment: 'production', status: 'ready', log: [], artifactDir: 'x', createdAt: '' },
      ]);
    });
    const res = await runOperation(api, 'listBuilds', { siteId: 's1' }, t);
    expect(res.ok).toBe(true);
    expect(res.label).toBe('Список сборок');
    expect(res.detail).toBe('1 шт.');
    expect(res.data).toEqual([
      { id: 'b1', siteId: 's1', snapshotId: 'snap1', environment: 'production', status: 'ready', log: [], artifactDir: 'x', createdAt: '' },
    ]);
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

  it('createBuild: POST → путь без query-мусора, статус в detail', async () => {
    const api = fixtureApi(async (url, init) => {
      expect(url).toBe('/api/sites/s1/builds');
      expect(init.method).toBe('POST');
      expect(JSON.parse(init.body as string)).toEqual({ snapshotId: 'snap1', environment: 'development' });
      return json(201, { id: 'b1', siteId: 's1', snapshotId: 'snap1', environment: 'development', status: 'ready', log: [], artifactDir: '' });
    });
    const res = await runOperation(api, 'createBuild', { siteId: 's1', snapshotId: 'snap1', environment: 'development' }, t);
    expect(res.ok).toBe(true);
    expect(res.detail).toBe('ready');
    expect(res.data).toMatchObject({ status: 'ready' });
  });

  it('getBuild: GET по buildId', async () => {
    const api = fixtureApi(async (url) => {
      expect(url).toBe('/api/builds/b1');
      return json(200, { id: 'b1', status: 'building' });
    });
    const res = await runOperation(api, 'getBuild', { buildId: 'b1' }, t);
    expect(res.ok).toBe(true);
    expect(res.data).toMatchObject({ status: 'building' });
  });

  it('deleteSnapshot: DELETE по snapshotId', async () => {
    const api = fixtureApi(async (url, init) => {
      expect(url).toBe('/api/snapshots/snap1');
      expect(init.method).toBe('DELETE');
      return noBody(204);
    });
    const res = await runOperation(api, 'deleteSnapshot', { snapshotId: 'snap1' }, t);
    expect(res.ok).toBe(true);
  });

  it('createContent: POST с телом {collectionId, id, fields}', async () => {
    const api = fixtureApi(async (url, init) => {
      expect(url).toBe('/api/sites/s1/contents');
      expect(init.method).toBe('POST');
      expect(JSON.parse(init.body as string)).toEqual({
        collectionId: 'strings',
        id: 'nav.home',
        fields: { title: 'Главная' },
      });
      return json(201, { id: 'nav.home', collectionId: 'strings', fields: { title: 'Главная' } });
    });
    const res = await runOperation(
      api,
      'createContent',
      { siteId: 's1', collectionId: 'strings', id: 'nav.home', fields: { title: 'Главная' } },
      t,
    );
    expect(res.ok).toBe(true);
    expect(res.detail).toBe('OK');
  });

  it('createContent: id опционален и не попадает в тело', async () => {
    const api = fixtureApi(async (url, init) => {
      expect(JSON.parse(init.body as string)).toEqual({ collectionId: 'c', fields: {} });
      return json(201, { id: 'x', collectionId: 'c', fields: {} });
    });
    const res = await runOperation(api, 'createContent', { siteId: 's1', collectionId: 'c' }, t);
    expect(res.ok).toBe(true);
  });

  it('deleteContent: DELETE → 204', async () => {
    const api = fixtureApi(async (url, init) => {
      expect(url).toBe('/api/sites/s1/contents/abc1');
      expect(init.method).toBe('DELETE');
      return noBody(204);
    });
    const res = await runOperation(api, 'deleteContent', { siteId: 's1', contentId: 'abc1' }, t);
    expect(res.ok).toBe(true);
    expect(res.detail).toBe('OK');
  });

  it('deleteTranslation: DELETE → 204', async () => {
    const api = fixtureApi(async (url, init) => {
      expect(url).toBe('/api/sites/s1/contents/abc1/translations/en');
      expect(init.method).toBe('DELETE');
      return noBody(204);
    });
    const res = await runOperation(api, 'deleteTranslation', { siteId: 's1', contentId: 'abc1', locale: 'en' }, t);
    expect(res.ok).toBe(true);
  });

  it('uploadAsset: POST multipart → FormData передаётся телом, mime не трогаем', async () => {
    const form = new FormData();
    form.append('file', new Blob(['x']), 'logo.png');
    const api = fixtureApi(async (url, init) => {
      expect(url).toBe('/api/sites/s1/assets');
      expect(init.method).toBe('POST');
      expect(init.body).toBe(form);
      expect((init.headers as Record<string, string> | undefined)?.['Content-Type']).toBeUndefined();
      return json(201, { id: 'a1', siteId: 's1', name: 'logo.png', mime: 'image/png', size: 1, variants: [] });
    });
    const res = await runOperation(api, 'uploadAsset', { siteId: 's1', form }, t);
    expect(res.ok).toBe(true);
    expect(res.detail).toBe('logo.png');
    expect(res.data).toMatchObject({ id: 'a1', name: 'logo.png' });
  });

  it('getAsset: GET → метаданные, name в detail', async () => {
    const api = fixtureApi(async (url) => {
      expect(url).toBe('/api/assets/a1');
      return json(200, { id: 'a1', siteId: 's1', name: 'logo.png', mime: 'image/png', size: 1, variants: [] });
    });
    const res = await runOperation(api, 'getAsset', { assetId: 'a1' }, t);
    expect(res.ok).toBe(true);
    expect(res.detail).toBe('logo.png');
    expect(res.data).toMatchObject({ id: 'a1' });
  });

  it('deleteAsset: DELETE по assetId → 204', async () => {
    const api = fixtureApi(async (url, init) => {
      expect(url).toBe('/api/assets/a1');
      expect(init.method).toBe('DELETE');
      return noBody(204);
    });
    const res = await runOperation(api, 'deleteAsset', { assetId: 'a1' }, t);
    expect(res.ok).toBe(true);
    expect(res.detail).toBe('OK');
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

describe('runOperation: роуты', () => {
  it('getRoute: GET по routeId', async () => {
    const api = fixtureApi(async (url) => {
      expect(url).toBe('/api/sites/s1/routes/r1');
      return json(200, {
        id: 'r1',
        siteId: 's1',
        matcher: '/about',
        priority: 1,
        action: { type: 'redirect', target: '/o-nas', status: 308, keepQuery: true },
      });
    });
    const res = await runOperation(api, 'getRoute', { siteId: 's1', routeId: 'r1' }, t);
    expect(res.ok).toBe(true);
    expect(res.detail).toBe('OK');
    expect(res.data).toMatchObject({ matcher: '/about' });
  });

  it('updateRoute: PUT → тело patch (matcher+action)', async () => {
    const action = { type: 'redirect', target: '/o-nas', status: 302, keepQuery: false };
    const api = fixtureApi(async (url, init) => {
      expect(url).toBe('/api/sites/s1/routes/r1');
      expect(init.method).toBe('PUT');
      expect(JSON.parse(init.body as string)).toEqual({ matcher: '/o-nas', priority: 2, action });
      return json(200, { id: 'r1', siteId: 's1', matcher: '/o-nas', priority: 2, action });
    });
    const res = await runOperation(
      api,
      'updateRoute',
      { siteId: 's1', routeId: 'r1', patch: { matcher: '/o-nas', priority: 2, action } },
      t,
    );
    expect(res.ok).toBe(true);
    expect(res.data).toMatchObject({ matcher: '/o-nas' });
  });
});

describe('runOperation: формы', () => {
  it('getForm: GET → name в detail', async () => {
    const api = fixtureApi(async (url) => {
      expect(url).toBe('/api/sites/s1/forms/form.contact');
      return json(200, { id: 'form.contact', siteId: 's1', name: 'Contact', definition: {} });
    });
    const res = await runOperation(api, 'getForm', { siteId: 's1', formId: 'form.contact' }, t);
    expect(res.ok).toBe(true);
    expect(res.detail).toBe('Contact');
    expect(res.data).toMatchObject({ id: 'form.contact' });
  });

  it('updateForm: PUT → тело {name, definition} (patch)', async () => {
    const definition = { id: 'form.contact', fields: [], submit: { endpoint: 'form.contact' } };
    const api = fixtureApi(async (url, init) => {
      expect(url).toBe('/api/sites/s1/forms/form.contact');
      expect(init.method).toBe('PUT');
      expect(JSON.parse(init.body as string)).toEqual({ name: 'Contact v2', definition });
      return json(200, { id: 'form.contact', siteId: 's1', name: 'Contact v2', definition });
    });
    const res = await runOperation(
      api,
      'updateForm',
      { siteId: 's1', formId: 'form.contact', patch: { name: 'Contact v2', definition } },
      t,
    );
    expect(res.ok).toBe(true);
    expect(res.detail).toBe('OK');
  });

  it('deleteForm: DELETE → 204', async () => {
    const api = fixtureApi(async (url, init) => {
      expect(url).toBe('/api/sites/s1/forms/form.contact');
      expect(init.method).toBe('DELETE');
      return noBody(204);
    });
    const res = await runOperation(api, 'deleteForm', { siteId: 's1', formId: 'form.contact' }, t);
    expect(res.ok).toBe(true);
    expect(res.detail).toBe('OK');
  });

  it('listSubmissions: GET → count в detail', async () => {
    const api = fixtureApi(async (url) => {
      expect(url).toBe('/api/sites/s1/forms/form.contact/submissions');
      return json(200, [{ id: 'subm_1', formId: 'form.contact', siteId: 's1', payload: {}, createdAt: '2026-01-01' }]);
    });
    const res = await runOperation(api, 'listSubmissions', { siteId: 's1', formId: 'form.contact' }, t);
    expect(res.ok).toBe(true);
    expect(res.detail).toBe('1 шт.');
    expect(res.data).toEqual(expect.arrayContaining([expect.objectContaining({ id: 'subm_1' })]));
  });
});

describe('runOperation: операции/эндпоинты (R6)', () => {
  it('listOperations: GET → count в detail', async () => {
    const api = fixtureApi(async (url) => {
      expect(url).toBe('/api/sites/s1/operations');
      return json(200, [{ id: 'content.get' }, { id: 'reviews.list' }]);
    });
    const res = await runOperation(api, 'listOperations', { siteId: 's1' }, t);
    expect(res.ok).toBe(true);
    expect(res.label).toBe('Операции');
    expect(res.detail).toBe('2 шт.');
  });

  it('createOperation: POST с телом дескриптора', async () => {
    const api = fixtureApi(async (url, init) => {
      expect(url).toBe('/api/sites/s1/operations');
      expect(init.method).toBe('POST');
      expect(JSON.parse(init.body as string)).toEqual({
        id: 'reviews.list',
        provider: 'cms',
        typeOp: 'query',
        method: 'GET',
        path: '/api/reviews',
        cache: 'disabled',
        ttl: null,
        scope: 'public',
        resultType: 'review[]',
        params: { in: 'query' },
        poll: {},
        subscribe: {},
      });
      return json(201, { id: 'reviews.list', siteId: 's1', system: false, method: 'GET', path: '/api/reviews' });
    });
    const res = await runOperation(
      api,
      'createOperation',
      {
        siteId: 's1',
        id: 'reviews.list',
        provider: 'cms',
        typeOp: 'query',
        method: 'GET',
        path: '/api/reviews',
        cache: 'disabled',
        scope: 'public',
        resultType: 'review[]',
        params: { in: 'query' },
      },
      t,
    );
    expect(res.ok).toBe(true);
    expect(res.detail).toBe('reviews.list');
  });

  it('updateOperation: PUT по operationId с id в URL (не в теле)', async () => {
    const api = fixtureApi(async (url, init) => {
      expect(url).toBe('/api/sites/s1/operations/form.submit');
      expect(init.method).toBe('PUT');
      expect(JSON.parse(init.body as string)).not.toHaveProperty('id');
      return json(200, { id: 'form.submit', siteId: 's1', system: false });
    });
    const res = await runOperation(api, 'updateOperation', { siteId: 's1', operationId: 'form.submit', path: '/api/forms/x' }, t);
    expect(res.ok).toBe(true);
    expect(res.detail).toBe('OK');
  });

  it('deleteOperation: DELETE по operationId → 204', async () => {
    const api = fixtureApi(async (url, init) => {
      expect(url).toBe('/api/sites/s1/operations/reviews.list');
      expect(init.method).toBe('DELETE');
      return noBody(204);
    });
    const res = await runOperation(api, 'deleteOperation', { siteId: 's1', operationId: 'reviews.list' }, t);
    expect(res.ok).toBe(true);
    expect(res.detail).toBe('OK');
  });

  it('createEndpoint: POST {id, method, path, operationId}', async () => {
    const api = fixtureApi(async (url, init) => {
      expect(url).toBe('/api/sites/s1/endpoints');
      expect(init.method).toBe('POST');
      expect(JSON.parse(init.body as string)).toEqual({
        id: 'booking',
        method: 'POST',
        path: '/booking',
        operationId: 'contact.save',
      });
      return json(201, { id: 'booking', siteId: 's1', system: false });
    });
    const res = await runOperation(
      api,
      'createEndpoint',
      { siteId: 's1', id: 'booking', method: 'POST', path: '/booking', operationId: 'contact.save' },
      t,
    );
    expect(res.ok).toBe(true);
    expect(res.detail).toBe('booking');
  });

  it('listEndpoints: GET → count в detail', async () => {
    const api = fixtureApi(async (url) => {
      expect(url).toBe('/api/sites/s1/endpoints');
      return json(200, [{ id: 'booking' }]);
    });
    const res = await runOperation(api, 'listEndpoints', { siteId: 's1' }, t);
    expect(res.ok).toBe(true);
    expect(res.detail).toBe('1 шт.');
  });

  it('updateEndpoint: PUT по endpointId', async () => {
    const api = fixtureApi(async (url, init) => {
      expect(url).toBe('/api/sites/s1/endpoints/booking');
      expect(init.method).toBe('PUT');
      expect(JSON.parse(init.body as string)).not.toHaveProperty('id');
      return json(200, { id: 'booking', siteId: 's1', system: false });
    });
    const res = await runOperation(api, 'updateEndpoint', { siteId: 's1', endpointId: 'booking', path: '/book' }, t);
    expect(res.ok).toBe(true);
    expect(res.detail).toBe('OK');
  });

  it('deleteEndpoint: DELETE по endpointId → 204', async () => {
    const api = fixtureApi(async (url, init) => {
      expect(url).toBe('/api/sites/s1/endpoints/booking');
      expect(init.method).toBe('DELETE');
      return noBody(204);
    });
    const res = await runOperation(api, 'deleteEndpoint', { siteId: 's1', endpointId: 'booking' }, t);
    expect(res.ok).toBe(true);
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