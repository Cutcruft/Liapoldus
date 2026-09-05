import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { ApiClient } from '../../src/core/api-client';
import { AssetResolver } from '../../src/core/assets';
import { registerBuiltin } from '../../src/core/builtin/descriptors';
import { ContentController } from '../../src/core/content';
import { createRuntimeStore } from '../../src/core/store';
import { RuntimeRegistry } from '../../src/core/registry';
import { TransportFactory } from '../../src/core/transport/factory';
import { AssetNotFoundError } from '../../src/errors';
import { makeFakeFetch, resetFakes } from './helpers';

const BUILTIN = 'liapoldus.builtin';

interface AssetFixture {
  id: string;
  name: string;
  type: string;
  size: number;
  variants: Array<{ name: string; url: string }>;
}

const assetsFix: Record<string, AssetFixture> = {
  a1: { id: 'a1', name: 'hero.jpg', type: 'image/jpeg', size: 2048, variants: [
    { name: 'master', url: '/assets/a1/master.jpg' },
    { name: 'thumb', url: '/assets/a1/thumb.jpg' },
  ] },
  a2: { id: 'a2', name: 'logo.svg', type: 'image/svg+xml', size: 512, variants: [
    { name: 'master', url: '/assets/a2/master.svg' },
  ] },
};

const heroContent = {
  id: 'hero',
  collectionId: 'pages',
  title: 'Hero',
  image: { assetId: 'a1', variant: 'thumb' },
};

function buildBackend() {
  return makeFakeFetch((call) => {
    const url = new URL(call.url, 'http://runtime.local');
    const path = url.pathname;
    const method = call.init.method ?? 'GET';

    let m = path.match(/^\/api\/assets\/([^/]+)$/);
    if (m && method === 'GET') {
      const a = assetsFix[m[1]];
      if (!a) return { status: 404, ok: false };
      return a;
    }

    m = path.match(/^\/api\/contents\/([^/]+)$/);
    if (m && method === 'GET') return m[1] === 'hero' ? heroContent : { status: 404, ok: false };

    return null;
  });
}

function api(registry: RuntimeRegistry, fetch: ReturnType<typeof makeFakeFetch>['fetch']) {
  const transport = new TransportFactory({ fetch }).create(registry.getProvider(BUILTIN)!);
  return new ApiClient(registry, { transport, scope: 'public' });
}

afterEach(() => resetFakes());
beforeEach(() => resetFakes());

describe('asset resolver', () => {
  function setup() {
    const registry = new RuntimeRegistry();
    registerBuiltin(registry);
    const { fetch, calls } = buildBackend();
    const client = api(registry, fetch);
    const store = createRuntimeStore();
    const resolver = new AssetResolver(store, client.builtin.asset);
    return { registry, fetch, calls, store, client, resolver };
  }

  it('get() → метаданные ассета и кэш в store.assets', async () => {
    const { resolver, store } = setup();
    const meta = await resolver.get('a1');

    expect(meta).toMatchObject({ id: 'a1', variants: expect.arrayContaining([
      { name: 'master', url: '/assets/a1/master.jpg' },
      { name: 'thumb', url: '/assets/a1/thumb.jpg' },
    ]) });
    expect(store.getState().assets.a1).toBe(meta);
  });

  it('get() повторно не ходит в транспорт (кэш в сторе)', async () => {
    const { resolver, calls } = setup();
    await resolver.get('a1');
    await resolver.get('a1');

    expect(calls.filter((c) => c.url.includes('/api/assets/a1'))).toHaveLength(1);
  });

  it('get() неизвестный ассет → AssetNotFoundError', async () => {
    const { resolver } = setup();
    await expect(resolver.get('none')).rejects.toThrow(AssetNotFoundError);
  });

  it('url(ref) с variant → URL нужного варианта', async () => {
    const { resolver, store } = setup();
    store.getState().setAssets({ a1: assetsFix.a1 });

    expect(resolver.url({ assetId: 'a1', variant: 'thumb' })).toBe('/assets/a1/thumb.jpg');
  });

  it('url(ref) без variant → master', async () => {
    const { resolver, store } = setup();
    store.getState().setAssets({ a2: assetsFix.a2 });

    expect(resolver.url({ assetId: 'a2' })).toBe('/assets/a2/master.svg');
  });

  it('url(ref) неизвестного ассета → undefined', async () => {
    const { resolver } = setup();
    expect(resolver.url({ assetId: 'none' })).toBeUndefined();
  });

  it('url(ref) с неизвестным вариантом → undefined', async () => {
    const { resolver, store } = setup();
    store.getState().setAssets({ a1: assetsFix.a1 });

    expect(resolver.url({ assetId: 'a1', variant: 'blur' })).toBeUndefined();
  });

  it('resolveDeep заменяет вложенные assetRef на URL (deep)', async () => {
    const { resolver, store } = setup();
    store.getState().setAssets({ a1: assetsFix.a1, a2: assetsFix.a2 });
    const data = {
      title: 'X',
      image: { assetId: 'a1', variant: 'thumb' },
      meta: { logo: { assetId: 'a2' }, width: 10 },
    };

    const out = resolver.resolveDeep(data);

    expect(out).toEqual({
      title: 'X',
      image: '/assets/a1/thumb.jpg',
      meta: { logo: '/assets/a2/master.svg', width: 10 },
    });
  });

  it('resolveDeep без структуры-указателя — те же данные', async () => {
    const { resolver } = setup();
    const data = { id: 'hero', strings: { title: 'Hero' } };

    expect(resolver.resolveDeep(data)).toEqual(data);
  });

  it('resolveDeep: неизвестный ассет не заменяется', async () => {
    const { resolver } = setup();
    const data = { image: { assetId: 'none', variant: 'thumb' } };
    const out = resolver.resolveDeep(data);

    expect(out).not.toEqual('/assets/none/thumb.jpg');
    expect(out).toEqual({ image: { assetId: 'none', variant: 'thumb' } });
  });

  it('resolveDeep не мутирует исходные данные (deep copy)', async () => {
    const { resolver, store } = setup();
    store.getState().setAssets({ a1: assetsFix.a1 });
    const data = { image: { assetId: 'a1' }, nested: { list: [{ n: 1 }] } };

    resolver.resolveDeep(data);

    expect(data.image).toEqual({ assetId: 'a1' });
    expect((data.nested as { list: Array<{ n: number }> }).list[0]).toEqual({ n: 1 });
  });

  it('ContentController.get через resolveDeep кладёт в store.content готовые URL без лишних запросов', async () => {
    const { resolver, store, client, calls } = setup();
    store.getState().setAssets({ a1: assetsFix.a1 });
    const controller = new ContentController(store, client.builtin.content, { siteId: 'site1', assetResolver: resolver });

    await controller.get('hero', { locale: 'ru' });

    expect(store.getState().content.hero.image).toBe('/assets/a1/thumb.jpg');
    expect(store.getState().content.hero.title).toBe('Hero');
    const assetCalls = calls.filter((c) => c.url.includes('/api/assets/'));
    expect(assetCalls).toHaveLength(0);
  });
});