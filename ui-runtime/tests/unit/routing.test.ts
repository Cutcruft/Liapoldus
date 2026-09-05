import { describe, expect, it } from 'vitest';
import { DescriptorValidationError, RouteNotFoundError } from '../../src/errors';
import { validateRouteDescriptor } from '../../src/core/descriptor';
import { applyTargetTemplate } from '../../src/core/matcher';
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

const home = routeDescriptor1;
const article = routeDescriptor2;
const old = routeDescriptor4;
const legacy = routeDescriptor5;
const robots = routeDescriptor6;

const TABLE = [home, article, old, legacy, robots];

function makeRouter(table: RouteDescriptor[]) {
  const store = createRuntimeStore();
  const router = new Router(store, { window: { location: { assign: () => undefined } } });
  router.addRoutes(table);
  return { store, router };
}

describe('Router (§12)', () => {
  it('match: именованные группы regex → params', () => {
    const { router } = makeRouter(TABLE);
    const res = router.match('/articles/42');
    expect(res).not.toBeNull();
    expect(res?.route.id).toBe('route.article');
    expect(res?.params).toEqual({ articleId: '42' });
  });

  it('match: незахватывающий путь → null', () => {
    const { router } = makeRouter(TABLE);
    expect(router.match('/articles/abc')).toBeNull();
  });

  it('match: redirect-роут возвращает status и target', () => {
    const { router } = makeRouter(TABLE);
    const res = router.match('/old');
    expect(res?.route.id).toBe('route.old');
    if (res?.route.action.type !== 'redirect') throw new Error('ожидался redirect');
    expect(res.route.action.status).toBe(301);
    expect(res.route.action.target).toBe('/new');
  });

  it('приоритет: более высокий priority выигрывает независимо от порядка', () => {
    const { router } = makeRouter([routeDescriptor7, article]);
    const res = router.match('/articles/42');
    expect(res?.route.id).toBe('route.article');
  });

  it('navigate → store.route обновлён + renderPage(pageId)', () => {
    const { store, router } = makeRouter(TABLE);
    const pages: string[] = [];
    router.on('renderPage', (e) => pages.push(e.pageId));
    router.navigate('/articles/42');
    expect(store.getState().route?.route.id).toBe('route.article');
    expect(store.getState().route?.params).toEqual({ articleId: '42' });
    expect(pages).toEqual(['page.article']);
  });

  it('navigate → redirect: клиентский переход к /new (стор меняет путь)', () => {
    const { store, router } = makeRouter(TABLE);
    router.navigate('/old');
    expect(store.getState().route?.route.id).toBe('route.home');
  });

  it('navigate → serveAsset: полный переход location.assign, SPA не рендерит', () => {
    const assigned: string[] = [];
    const store = createRuntimeStore();
    const router = new Router(store, {
      window: { location: { assign: (url) => assigned.push(url) } },
    });
    router.addRoutes(TABLE);
    router.navigate('/robots.txt');
    expect(assigned).toEqual(['/robots.txt']);
    expect(store.getState().route).toBeNull();
  });

  it('redirect: группа regex захвачена в target + keepQuery', () => {
    expect(applyTargetTemplate(legacy.action.type === 'redirect' ? legacy.action.target : '', legacy.matcher, '/legacy/foo')).toBe('/modern/foo');
    if (legacy.action.type !== 'redirect') throw new Error('ожидался redirect');
    expect(legacy.action.keepQuery).toBe(true);

    const modern: RouteDescriptor = {
      id: 'route.modern',
      matcher: '^/modern/(.*)$',
      priority: 6,
      action: { type: 'renderPage', pageId: 'page.modern' },
    };
    const { router } = makeRouter([legacy, modern]);
    router.navigate('/legacy/foo?a=1');
    expect(router.match('/legacy/foo?a=1')?.route.id).toBe('route.legacy');
  });

  it('нет совпадений: match → null, navigate → RouteNotFoundError', () => {
    const { router } = makeRouter(TABLE);
    expect(router.match('/nope')).toBeNull();
    expect(() => router.navigate('/nope')).toThrow(RouteNotFoundError);
  });

  it('валидация матчегора без якорей → DescriptorValidationError', () => {
    expect(() =>
      validateRouteDescriptor({ id: 'r', matcher: '/articles', priority: 0, action: { type: 'renderPage', pageId: 'p' } }),
    ).toThrow(DescriptorValidationError);
  });

  it('порядок: равенство priority решается порядком регистрации (первый)', () => {
    const rA: RouteDescriptor = { id: 'a', matcher: '^/dupe$', priority: 10, action: { type: 'renderPage', pageId: 'pA' } };
    const rB: RouteDescriptor = { id: 'b', matcher: '^/dupe$', priority: 10, action: { type: 'renderPage', pageId: 'pB' } };
    const { router } = makeRouter([rA, rB]);
    expect(router.match('/dupe')?.route.id).toBe('a');
  });

  it('redirect status вне {301,302,307,308} → DescriptorValidationError', () => {
    expect(() =>
      validateRouteDescriptor({ id: 'r', matcher: '^/x$', priority: 0, action: { type: 'redirect', target: '/y', status: 200 } }),
    ).toThrow(DescriptorValidationError);
  });
});