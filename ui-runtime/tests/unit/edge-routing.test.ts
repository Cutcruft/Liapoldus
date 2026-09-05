import { describe, expect, it } from 'vitest';
import { createMatcher } from '../../src/core/matcher';
import { RequestRouter } from '../../src/core/request-router';
import { Router } from '../../src/core/router';
import { createRuntimeStore } from '../../src/core/store';
import type { RouteDescriptor } from '../../src/types/descriptor';
import {
  routeDescriptor1,
  routeDescriptor2,
  routeDescriptor4,
  routeDescriptor5,
  routeDescriptor6,
  routeDescriptor7,
} from './fixtures';

const TABLE: RouteDescriptor[] = [
  routeDescriptor2,
  routeDescriptor7,
  routeDescriptor4,
  routeDescriptor5,
  routeDescriptor6,
  routeDescriptor1,
];

const HTML_SHELL = '<!doctype html><html><body>lap</body></html>';
const ROBOTS = 'User-agent: *\nDisallow:';

function makeRouter() {
  const deps = {
    match: createMatcher(TABLE),
    htmlShell: HTML_SHELL,
    assets: {
      'asset.robots': { assetId: 'asset.robots', bytes: ROBOTS, mime: 'text/plain', etag: '"r1"' },
    },
  };
  return new RequestRouter(deps);
}

describe('RequestRouter (§12a)', () => {
  it('handle(GET /articles/42) → renderPage: shell + text/html', () => {
    const res = makeRouter().handle({ method: 'GET', path: '/articles/42' });
    expect(res.status).toBe(200);
    expect(res.body).toBe(HTML_SHELL);
    expect(res.headers['content-type']).toBe('text/html');
  });

  it('handle(GET /articles/abc) → нет совпадений → 404', () => {
    expect(makeRouter().handle({ method: 'GET', path: '/articles/abc' }).status).toBe(404);
  });

  it('handle(GET /robots.txt) → serveAsset: bytes + text/plain + cache/ETag', () => {
    const res = makeRouter().handle({ method: 'GET', path: '/robots.txt' });
    expect(res.status).toBe(200);
    expect(res.body).toBe(ROBOTS);
    expect(res.headers['content-type']).toBe('text/plain');
    expect(res.headers['cache-control']).toContain('immutable');
    expect(res.headers['etag']).toBe('"r1"');
  });

  it('handle(GET /old) → redirect 301 → Location /new (без query)', () => {
    const res = makeRouter().handle({ method: 'GET', path: '/old' });
    expect(res.status).toBe(301);
    expect(res.location).toBe('/new');
  });

  it('handle(GET /legacy/x?a=1) с keepQuery → 308 → /modern/x?a=1', () => {
    const res = makeRouter().handle({ method: 'GET', path: '/legacy/x?a=1' });
    expect(res.status).toBe(308);
    expect(res.location).toBe('/modern/x?a=1');
  });

  it('redirect без keepQuery → query не копируется', () => {
    const res = makeRouter().handle({ method: 'GET', path: '/old?a=1' });
    expect(res.status).toBe(301);
    expect(res.location).toBe('/new');
  });

  it('match по той же таблице: приоритет/порядок идентичны клиентскому Router', () => {
    const client = new Router(createRuntimeStore());
    client.addRoutes(TABLE);
    const edge = makeRouter();
    for (const path of ['/articles/42', '/old', '/legacy/x?a=1', '/robots.txt', '/', '/missing']) {
      expect(edge.match(path)?.route.id).toBe(client.match(path)?.route.id);
    }
    expect(edge.match('/articles/42')?.route.id).toBe('route.article');
  });

  it('renderPage отдаёт одну и ту же оболочку каждый запрос (cache)', () => {
    const router = makeRouter();
    const r1 = router.handle({ method: 'GET', path: '/articles/42' });
    const r2 = router.handle({ method: 'GET', path: '/' });
    expect(r1.body).toBe(HTML_SHELL);
    expect(r2.body).toBe(HTML_SHELL);
    expect(r1.body).toBe(r2.body);
    expect(r1.headers['cache-control']).toContain('public');
  });

  it('serveAsset отдаёт правильный content-type по mime и корректный ETag', () => {
    const router = makeRouter();
    const res = router.handle({ method: 'GET', path: '/robots.txt' });
    expect(res.headers['content-type']).toBe('text/plain');
    expect(res.headers['etag']).toBe('"r1"');
  });
});