import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { ApiClient } from '../../src/core/api-client';
import { registerBuiltin } from '../../src/core/builtin/descriptors';
import { RuntimeRegistry } from '../../src/core/registry';
import { TransportFactory } from '../../src/core/transport/factory';
import {
  AssetNotFoundError,
  DuplicateRegistrationError,
  ScopeError,
  TransportError,
} from '../../src/errors';
import type { OperationDescriptor } from '../../src/types/descriptor';
import { providerDescriptor5 } from './fixtures';
import { makeFakeFetch, resetFakes } from './helpers';

const BUILTIN = 'liapoldus.builtin';

interface ContentFixture {
  base: Record<string, unknown>;
  overlays: Record<string, Record<string, unknown> | undefined>;
}

const contentsFix: Record<string, ContentFixture> = {
  abouts: {
    base: { id: 'abouts', collectionId: 'pages', strings: { title: 'About', body: 'About body', motto: 'Motto' } },
    overlays: {
      ru: { collectionId: 'pages', strings: { title: 'О нас', body: 'О нас body', motto: 'Девиз' } },
      de: { collectionId: 'pages', strings: { title: 'Über' } },
    },
  },
  hero: { base: { id: 'hero', collectionId: 'pages', strings: { title: 'Hero', body: 'Hero body' } }, overlays: {} },
  nav: { base: { id: 'nav', collectionId: 'strings', strings: { home: 'Home', favorites: 'Favorites' } }, overlays: {} },
};

interface AssetFixture {
  id: string;
  siteId: string;
  name: string;
  type: string;
  size: number;
  variants: Array<{ name: string; url: string }>;
}

const assetsFix: Record<string, AssetFixture> = {
  a1: { id: 'a1', siteId: 'site1', name: 'hero.jpg', type: 'image/jpeg', size: 2048, variants: [{ name: 'master', url: '/assets/a1/master.jpg' }, { name: 'thumb', url: '/assets/a1/thumb.jpg' }] },
  a2: { id: 'a2', siteId: 'site1', name: 'logo.svg', type: 'image/svg+xml', size: 512, variants: [{ name: 'master', url: '/assets/a2/master.svg' }] },
};

const formFix = {
  id: 'f1',
  fields: { email: { type: 'email', required: true }, name: { type: 'string', required: false } },
  validation: { email: { pattern: '^[^@]+@[^@]+$' } },
  submit: { endpointId: 'form.submit' },
};

function mergeContent(base: Record<string, unknown>, overlay: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = { ...base };
  for (const [k, v] of Object.entries(overlay)) {
    if (
      v && typeof v === 'object' && !Array.isArray(v) &&
      out[k] && typeof out[k] === 'object' && !Array.isArray(out[k])
    ) {
      out[k] = mergeContent(out[k] as Record<string, unknown>, v as Record<string, unknown>);
    } else {
      out[k] = v;
    }
  }
  return out;
}

function buildBackend() {
  return makeFakeFetch((call) => {
    const url = new URL(call.url, 'http://runtime.local');
    const path = url.pathname;
    const method = call.init.method ?? 'GET';

    let m = path.match(/^\/api\/contents\/([^/]+)$/);
    if (m && method === 'GET') {
      const c = contentsFix[m[1]];
      if (!c) return { status: 404, ok: false };
      const locale = url.searchParams.get('locale') ?? undefined;
      const overlay = locale != null ? c.overlays[locale] : undefined;
      return overlay ? mergeContent(c.base, overlay) : c.base;
    }

    if (path.endsWith('/contents/batch') && method === 'POST') {
      const body = JSON.parse(String(call.init.body)) as { ids?: string[]; locale?: string };
      const out: Record<string, unknown> = {};
      for (const id of body.ids ?? []) {
        const c = contentsFix[id];
        if (!c) continue;
        const overlay = body.locale != null ? c.overlays[body.locale] : undefined;
        out[id] = overlay ? mergeContent(c.base, overlay) : c.base;
      }
      return out;
    }

    m = path.match(/^\/api\/sites\/([^/]+)\/contents$/);
    if (m && method === 'GET') {
      const siteId = m[1];
      const collectionId = url.searchParams.get('collectionId') ?? undefined;
      const locale = url.searchParams.get('locale') ?? undefined;
      const out: Array<Record<string, unknown>> = [];
      for (const [id, c] of Object.entries(contentsFix)) {
        if (collectionId != null && c.base.collectionId !== collectionId) continue;
        const overlay = locale != null ? c.overlays[locale] : undefined;
        out.push({ id, collectionId: c.base.collectionId, fields: overlay ? mergeContent(c.base, overlay) : c.base });
      }
      void siteId;
      return out;
    }

    m = path.match(/^\/api\/assets\/([^/]+)$/);
    if (m && method === 'GET') {
      const a = assetsFix[m[1]];
      if (!a) return { status: 404, ok: false };
      return a;
    }

    m = path.match(/^\/api\/sites\/([^/]+)\/assets$/);
    if (m && method === 'GET') {
      const siteId = m[1];
      return Object.values(assetsFix).filter((a) => a.siteId === siteId);
    }

    if (path.endsWith('/assets/batch') && method === 'POST') {
      const body = JSON.parse(String(call.init.body)) as { ids?: string[] };
      const out: Record<string, unknown> = {};
      for (const id of body.ids ?? []) {
        if (assetsFix[id]) out[id] = assetsFix[id];
      }
      return out;
    }

    m = path.match(/^\/api\/forms\/([^/]+)$/);
    if (m && method === 'GET') return m[1] === 'f1' ? formFix : { status: 404, ok: false };

    m = path.match(/^\/api\/forms\/([^/]+)\/submissions$/);
    if (m && method === 'POST') return { submissionId: 'sub-1', state: 'ok' };

    return null;
  });
}

function builtinApi(registry: RuntimeRegistry, fetch: ReturnType<typeof makeFakeFetch>['fetch'], scope?: 'public' | 'server') {
  const transport = new TransportFactory({ fetch }).create(registry.getProvider(BUILTIN)!);
  return new ApiClient(registry, { transport, scope });
}

afterEach(() => resetFakes());
beforeEach(() => resetFakes());

describe('builtin', () => {
  it('content.get(id, {locale}) → ContentData смёржен под локаль (перевод приоритетнее base)', async () => {
    const registry = new RuntimeRegistry();
    registerBuiltin(registry);
    const { fetch } = buildBackend();
    const api = builtinApi(registry, fetch);

    const res = await api.builtin.content.get('abouts', { locale: 'ru' });
    expect(res).toHaveProperty('id', 'abouts');
    expect((res as { strings: Record<string, string> }).strings).toEqual({
      title: 'О нас',
      body: 'О нас body',
      motto: 'Девиз',
    });
  });

  it('content.get с частичным переводом → непереведённые поля остаются из base', async () => {
    const registry = new RuntimeRegistry();
    registerBuiltin(registry);
    const { fetch } = buildBackend();
    const api = builtinApi(registry, fetch);

    const res = (await api.builtin.content.get('abouts', { locale: 'de' })) as { strings: Record<string, string> };
    expect(res.strings.title).toBe('Über');
    expect(res.strings.body).toBe('About body');
    expect(res.strings.motto).toBe('Motto');
  });

  it('content.get с неизвестной локалью → base (серверный фолбэк)', async () => {
    const registry = new RuntimeRegistry();
    registerBuiltin(registry);
    const { fetch } = buildBackend();
    const api = builtinApi(registry, fetch);

    const res = (await api.builtin.content.get('abouts', { locale: 'xx' })) as { strings: Record<string, string> };
    expect(res.strings.title).toBe('About');
    expect(res.strings.body).toBe('About body');
  });

  it('content.get без locale → base', async () => {
    const registry = new RuntimeRegistry();
    registerBuiltin(registry);
    const { fetch } = buildBackend();
    const api = builtinApi(registry, fetch);

    const res = (await api.builtin.content.get('abouts')) as { strings: Record<string, string> };
    expect(res.strings.title).toBe('About');
  });

  it('content.get строит URL с ?locale=ru', async () => {
    const registry = new RuntimeRegistry();
    registerBuiltin(registry);
    const { fetch, calls } = buildBackend();
    const api = builtinApi(registry, fetch);

    await api.builtin.content.get('abouts', { locale: 'ru' });
    expect(calls[0].url).toBe('/api/contents/abouts?locale=ru');
  });

  it('content.get отсутствующего контента → TransportError', async () => {
    const registry = new RuntimeRegistry();
    registerBuiltin(registry);
    const { fetch } = buildBackend();
    const api = builtinApi(registry, fetch);

    await expect(api.builtin.content.get('missing')).rejects.toThrow(TransportError);
  });

  it('content.list(siteId, {locale}) → массив {id, collectionId, fields} под локалью', async () => {
    const registry = new RuntimeRegistry();
    registerBuiltin(registry);
    const { fetch, calls } = buildBackend();
    const api = builtinApi(registry, fetch);

    const list = await api.builtin.content.list('site1', { locale: 'ru' });
    expect(list.length).toBe(3);
    const abouts = list.find((e) => e.id === 'abouts')!;
    expect(abouts.collectionId).toBe('pages');
    expect((abouts.fields as { strings: Record<string, string> }).strings.title).toBe('О нас');
    expect(calls[0].url).toContain('/api/sites/site1/contents?locale=ru');
  });

  it('content.list(siteId, {collectionId, locale}) → UI-строки коллекции strings', async () => {
    const registry = new RuntimeRegistry();
    registerBuiltin(registry);
    const { fetch, calls } = buildBackend();
    const api = builtinApi(registry, fetch);

    const list = await api.builtin.content.list('site1', { collectionId: 'strings', locale: 'ru' });
    expect(list).toHaveLength(1);
    expect(list[0].id).toBe('nav');
    expect(calls[0].url).toContain('collectionId=strings');
  });

  it('content.batch(siteId, {ids}, {locale}) → {[id]: ContentData}; отсутствующий id пропускается', async () => {
    const registry = new RuntimeRegistry();
    registerBuiltin(registry);
    const { fetch, calls } = buildBackend();
    const api = builtinApi(registry, fetch);

    const batch = await api.builtin.content.batch('site1', { ids: ['abouts', 'hero', 'missing'] }, { locale: 'ru' });
    expect(Object.keys(batch)).toEqual(['abouts', 'hero']);
    expect((batch.abouts as { strings: Record<string, string> }).strings.title).toBe('О нас');
    const body = JSON.parse(String(calls[0].init.body)) as { ids: string[]; locale: string };
    expect(body.ids).toEqual(['abouts', 'hero', 'missing']);
    expect(body.locale).toBe('ru');
  });

  it('asset.get → AssetMeta (мастер + variants); неизвестный id → AssetNotFoundError', async () => {
    const registry = new RuntimeRegistry();
    registerBuiltin(registry);
    const { fetch } = buildBackend();
    const api = builtinApi(registry, fetch);

    const meta = await api.builtin.asset.get('a1');
    expect(meta.id).toBe('a1');
    expect(meta.variants.length).toBe(2);
    expect(meta.variants[1].name).toBe('thumb');

    await expect(api.builtin.asset.get('nope')).rejects.toThrow(AssetNotFoundError);
  });

  it('asset.list(siteId) → массив; asset.batch(ids) → {[id]: AssetMeta}', async () => {
    const registry = new RuntimeRegistry();
    registerBuiltin(registry);
    const { fetch } = buildBackend();
    const api = builtinApi(registry, fetch);

    const list = await api.builtin.asset.list('site1');
    expect(list.map((a) => a.id)).toEqual(['a1', 'a2']);

    const batch = await api.builtin.asset.batch(['a1', 'nope']);
    expect(Object.keys(batch)).toEqual(['a1']);
  });

  it('form.get(formId) → FormDefinition (схема/валидация/submit-конфиг)', async () => {
    const registry = new RuntimeRegistry();
    registerBuiltin(registry);
    const { fetch } = buildBackend();
    const api = builtinApi(registry, fetch);

    const form = await api.builtin.form.get('f1');
    expect(form.id).toBe('f1');
    expect(form.fields).toHaveProperty('email');
    expect(form.submit.endpointId).toBe('form.submit');
  });

  it('form.submit — server-only: из компонента (public) → ScopeError; в server-контексте отправляет payload', async () => {
    const registry = new RuntimeRegistry();
    registerBuiltin(registry);
    const { fetch, calls } = buildBackend();

    const publicApi = builtinApi(registry, fetch);
    await expect(publicApi.builtin.form.submit({ formId: 'f1', values: { email: 'a@b.c' } })).rejects.toThrow(ScopeError);

    const serverApi = builtinApi(registry, fetch, 'server');
    const res = await serverApi.builtin.form.submit({ formId: 'f1', values: { email: 'a@b.c' }, submittedAt: 1234 });
    expect(res).toEqual({ submissionId: 'sub-1', state: 'ok' });
    const body = JSON.parse(String(calls[0].init.body)) as { formId: string; values: Record<string, unknown>; submittedAt: number };
    expect(body.formId).toBe('f1');
    expect(body.values).toEqual({ email: 'a@b.c' });
    expect(body.submittedAt).toBe(1234);
  });

  it('builtin-операции регистрируются автоматически и не перезаписываются дубликатом', async () => {
    const registry = new RuntimeRegistry();
    registerBuiltin(registry);
    expect(registry.hasProvider(BUILTIN)).toBe(true);
    expect(registry.hasOperation('content.get')).toBe(true);
    expect(registry.hasOperation('asset.batch')).toBe(true);
    expect(registry.hasOperation('form.submit')).toBe(true);
    expect(registry.hasEndpoint('form.submit')).toBe(true);

    expect(() => registerBuiltin(registry)).not.toThrow();

    const dup: OperationDescriptor = {
      kind: 'operation',
      id: 'content.get',
      typeOp: 'query',
      providerId: BUILTIN,
      method: 'GET',
      path: '/hijack',
      cache: 'disabled',
    };
    expect(() => registry.registerOperation(dup)).toThrow(DuplicateRegistrationError);
  });

  it('builtin-звонки можно замокать пред-регистрацией без перезаписи', async () => {
    const registry = new RuntimeRegistry();
    registry.register(providerDescriptor5);
    const mockOp: OperationDescriptor = {
      kind: 'operation',
      id: 'content.get',
      typeOp: 'query',
      providerId: 'noBase',
      method: 'GET',
      path: '/mock/contents/{contentId}',
      params: { in: 'query', fields: { contentId: { required: true } } },
      cache: 'disabled',
    };
    registry.register(mockOp);
    registerBuiltin(registry);

    const { fetch, calls } = makeFakeFetch(() => ({ mocked: true }));
    const transport = new TransportFactory({ fetch }).create(registry.getProvider('noBase')!);
    const api = new ApiClient(registry, { transport });

    const res = await api.builtin.content.get('x');
    expect(res).toEqual({ mocked: true });
    expect(calls[0].url).toContain('/mock/contents/x');
  });
});