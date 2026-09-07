import { describe, expect, it, vi } from 'vitest';
import {
  PageLoader,
  getPageTree,
  hasPageTree,
  parseManifest,
  registerPage,
  resolveBuildBase,
} from '../../src/core/pages';
import type { ChunkImporter } from '../../src/core/pages';
import type { PageDeclaration } from '../../src/types/page';

const tree = (pageId: string, elementId = 'root'): PageDeclaration => ({
  pageId,
  elements: [{ id: elementId, componentId: 'text', props: {} }],
});

const MANIFEST = JSON.stringify({
  shared: { react: 'X', '@liapoldus/ui-runtime': 'Y' },
  deps: { lodash: { name: 'lodash', version: '4.17.21' } },
  externals: ['react'],
  styles: ['dist/_deps/a.css'],
  pages: [
    { pageId: 'page_a', chunk: 'pages/page_a.js', definitions: ['text'] },
    { pageId: 'page_b', chunk: 'pages/page_b.js', definitions: ['hero'] },
  ],
  homePage: 'page_a',
});

function bodyManifest(raw: string) {
  return { ok: true, status: 200, async text() { return raw; } } as Response;
}

describe('pages (Этап 5 п.4: постраничная доставка)', () => {
  it('registerPage/getPageTree/hasPageTree — статический реестр чанков', () => {
    registerPage('p1', tree('p1'));
    expect(hasPageTree('p1')).toBe(true);
    expect(hasPageTree('nope')).toBe(false);
    expect(getPageTree('p1')?.elements[0].id).toBe('root');
  });

  it('parseManifest читает camelCase-манифест Go-сборки', () => {
    const m = parseManifest(MANIFEST);
    expect(m.homePage).toBe('page_a');
    expect(m.pages).toHaveLength(2);
    expect(m.pages[0]).toMatchObject({ pageId: 'page_a', chunk: 'pages/page_a.js', definitions: ['text'] });
    expect(m.shared['react']).toBe('X');
    expect(m.deps['lodash']).toEqual({ name: 'lodash', version: '4.17.21' });
    expect(m.externals).toContain('react');
    expect(m.styles).toContain('dist/_deps/a.css');
  });

  it('parseManifest терпит пустой/мусорный манифест', () => {
    const empty = parseManifest('{}');
    expect(empty.pages).toEqual([]);
    expect(empty.shared).toEqual({});
    expect(() => parseManifest('not json')).toThrow();
  });

  it('resolveBuildBase: явный корень, вывод из URL сервировки и baseUrl+latest', () => {
    expect(resolveBuildBase('http://x', 's', 'prod', 'v1', 'http://cdn/build/s/prod/v1')).toBe('http://cdn/build/s/prod/v1');
    expect(resolveBuildBase('http://x', 's', 'prod', 'v2')).toBe('http://x/build/s/prod/v2');
    expect(resolveBuildBase('http://x', 's', 'prod')).toBe('http://x/build/s/prod/latest');
    expect(resolveBuildBase(undefined, 's', 'prod')).toBeNull();
  });

  it('resolveBuildBase выводит корень из URL когда страница сервится из /build', () => {
    const pathname = '/build/site_dr/production/snap-1/index.html';
    const mockLocation = { origin: 'https://cdn.example', pathname };
    const prev = globalThis.location;
    Object.defineProperty(globalThis, 'location', { value: mockLocation, configurable: true });
    try {
      expect(resolveBuildBase(undefined as unknown as string, 'site_dr', 'production')).toBe(
        'https://cdn.example/build/site_dr/production/snap-1',
      );
      // несовпадающий siteId в URL → фолбэк на baseUrl
      expect(resolveBuildBase(undefined as unknown as string, 'other', 'production')).toBeNull();
    } finally {
      Object.defineProperty(globalThis, 'location', { value: prev, configurable: true });
    }
  });

  it('PageLoader: манифест + import() чанка с кэшем', async () => {
    const fetchLike = vi.fn(async () => bodyManifest(MANIFEST)) as unknown as ChunkImporter;
    const imported: string[] = [];
    const importer: ChunkImporter = async (url) => {
      imported.push(url);
      registerPage('page_a', tree('page_a'));
    };
    registerPage('page_b', tree('page_b'));

    const loader = new PageLoader('https://x/build/s/prod/v1', fetchLike as never, importer);
    expect(loader.enabled).toBe(true);
    expect(await loader.chunkUrl('page_a')).toBe('https://x/build/s/prod/v1/pages/page_a.js');
    expect(await loader.load('page_a')).toBe(true);
    expect(await loader.load('page_a')).toBe(true);
    expect(imported).toEqual(['https://x/build/s/prod/v1/pages/page_a.js']);
    expect(await loader.load('missing')).toBe(false);
    expect(imported).toHaveLength(1);
  });

  it('PageLoader: ensureHome грузит домашний чанк; сбой import() → false и повтор возможен', async () => {
    let calls = 0;
    let fail = true;
    const importer: ChunkImporter = async () => {
      calls++;
      if (fail) throw new Error('boom');
      registerPage('page_a', tree('page_a'));
      return {};
    };
    const fetchLike = vi.fn(async () => bodyManifest(MANIFEST));
    const loader = new PageLoader('https://x/build/s/prod/v1', fetchLike as never, importer);
    expect(await loader.ensureHome()).toBe(false);
    expect(calls).toBe(1);
    fail = false;
    expect(await loader.ensureHome()).toBe(true);
    // сбой не закэширован: повторная попытка после успеха — кэш
    expect(calls).toBe(2);
  });

  it('PageLoader: onLoaded уведомляет после успешной загрузки чанка', async () => {
    const importer: ChunkImporter = async (url) => {
      registerPage(url.endsWith('page_a.js') ? 'page_a' : 'page_b', tree('page_a'));
      return {};
    };
    const loader = new PageLoader('https://x/build', vi.fn(async () => bodyManifest(MANIFEST)) as never, importer);
    const listener = vi.fn();
    const off = loader.onLoaded(listener);
    await loader.load('page_b');
    expect(listener).toHaveBeenCalledTimes(1);
    off();
    await loader.load('page_a');
    expect(listener).toHaveBeenCalledTimes(1);
  });

  it('PageLoader: выключен без buildRoot (нет раздачи) — no-op, без fetch', async () => {
    const fetchLike = vi.fn();
    const loader = new PageLoader(null, fetchLike as never);
    expect(loader.enabled).toBe(false);
    expect(await loader.manifest()).toBeNull();
    expect(await loader.load('page_a')).toBe(false);
    expect(await loader.chunkUrl('page_a')).toBeNull();
    expect(fetchLike).not.toHaveBeenCalled();
  });

  it('PageLoader: parseManifest сбоя фетча манифеста → null и повторный фетч', async () => {
    const fetchLike = vi.fn(async () => ({ ok: false, status: 404 }) as Response);
    const loader = new PageLoader('https://x/build', fetchLike as never);
    expect(await loader.manifest()).toBeNull();
    // фетч манифеста кэшируется даже на ошибке (перезагрузка чанка невозможна)
    expect(await loader.manifest()).toBeNull();
    expect(fetchLike).toHaveBeenCalledTimes(1);
  });
});