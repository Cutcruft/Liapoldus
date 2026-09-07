import { beforeEach, describe, expect, it } from 'vitest';
import { render, act } from '@testing-library/react';
import { resolvePageHead, resolvedHeadHasContent } from '../../src/core/head';
import { applyHead, HeadController, type HeadDocument } from '../../src/react/head-controller';
import { RuntimeProvider } from '../../src/react/context';
import { createRuntimeStore } from '../../src/core/store';
import { TreeController } from '../../src/core/tree';
import type { BootRuntime } from '../../src/core/boot';
import type { SiteHead } from '../../src/types/descriptor';

function headDoc(): HeadDocument {
  return {
    get title() {
      return document.title;
    },
    set title(v: string) {
      document.title = v;
    },
    createElement: (tag) => document.createElement(tag),
    head: {
      querySelector: (sel) => document.head.querySelector(sel),
      appendChild: (node) => void document.head.appendChild(node),
    },
  };
}

beforeEach(() => {
  document.head.innerHTML = '';
  document.title = '';
});

function fakeRuntime(overrides?: { siteHead?: SiteHead }): BootRuntime {
  const store = createRuntimeStore();
  const tree = new TreeController(store);
  return {
    siteId: 'site',
    environment: 'development',
    ready: true,
    siteHead: overrides?.siteHead,
    defaultLayoutSectionId: undefined,
    registry: {} as never,
    store: store as never,
    router: {} as never,
    sync: {} as never,
    client: {} as never,
    i18n: {} as never,
    tokens: {} as never,
    tree,
    forms: {} as never,
    assets: {} as never,
    dispose: () => undefined,
  } as unknown as BootRuntime;
}

describe('resolvePageHead (R10 P1): client-side merge site → page', () => {
  it('page скаляры перекрывают site-дефолты; og/meta мержатся по ключу', () => {
    const resolved = resolvePageHead(
      {
        titleTemplate: '{title} — ACME',
        description: 'site desc',
        og: { title: 'site og', type: 'website' },
        meta: { themeColor: '#000' },
      },
      {
        title: 'Page',
        description: 'page desc',
        og: { title: 'page og' },
        meta: { robots: 'noindex' },
      },
    );
    expect(resolved.title).toBe('Page — ACME');
    expect(resolved.description).toBe('page desc');
    expect(resolved.og).toEqual({ title: 'page og', type: 'website' });
    expect(resolved.meta).toEqual({ themeColor: '#000', robots: 'noindex' });
  });

  it('без page head — используются только site-дефолты', () => {
    const resolved = resolvePageHead({ description: 'site', titleTemplate: 'SITE' }, undefined);
    expect(resolved.title).toBe('SITE');
    expect(resolved.description).toBe('site');
    expect(resolved.robots).toBeUndefined();
    expect(resolvedHeadHasContent(resolved)).toBe(true);
  });

  it('пустой head → no content', () => {
    expect(resolvedHeadHasContent(resolvePageHead(undefined, {}))).toBe(false);
  });

  it('titleTemplate без {title}: page title перекрывает site-шаблон целиком', () => {
    const resolved = resolvePageHead({ titleTemplate: 'Fixed' }, { title: 'Page' });
    expect(resolved.title).toBe('Page');
  });

  it('page title + шаблон с {title} → "{title}" подставляется', () => {
    const resolved = resolvePageHead({ titleTemplate: '{title} — ACME' }, { title: 'Page' });
    expect(resolved.title).toBe('Page — ACME');
  });
});

describe('applyHead: пишет meta/link чисто, без дубликатов при обновлении', () => {
  it('первый apply создаёт title/meta/link; повторный обновляет content без новых узлов', () => {
    const doc = headDoc();
    const first = applyHead(
      doc,
      {
        title: 'Hello',
        description: 'desc',
        canonical: 'https://x.test/',
        og: { title: 'Hello' },
        meta: { robots: 'index' },
      },
      new Map(),
    );
    expect(doc.title).toBe('Hello');
    expect(document.head.querySelector('meta[name="description"]')?.getAttribute('content')).toBe('desc');
    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe('https://x.test/');
    expect(document.head.querySelector('meta[property="og:title"]')?.getAttribute('content')).toBe('Hello');

    const before = document.head.querySelectorAll('meta, link').length;
    const second = applyHead(
      doc,
      {
        title: 'Hello',
        description: 'new',
        canonical: 'https://x.test/',
        og: { title: 'Hello' },
        meta: { robots: 'index' },
      },
      first,
    );
    expect(document.head.querySelector('meta[name="description"]')?.getAttribute('content')).toBe('new');
    expect(document.head.querySelectorAll('meta, link').length).toBe(before);
    void second;
  });

  it('старые meta, не нужные в новом head, удаляются', () => {
    const doc = headDoc();
    const first = applyHead(doc, { og: { title: 'A', type: 'website' } }, new Map());
    expect(document.head.querySelector('meta[property="og:title"]')).not.toBeNull();
    applyHead(doc, { og: { title: 'A' } }, first);
    expect(document.head.querySelector('meta[property="og:type"]')).toBeNull();
    expect(document.head.querySelector('meta[property="og:title"]')).not.toBeNull();
  });
});

describe('HeadController (R10 P1): мержит site+page и пишет в document.head', () => {
  it('через store.tree (резолвленная декларация) выводит title/description; site-дефолт подставляется', () => {
    const runtime = fakeRuntime({
      siteHead: { titleTemplate: '{title} — ACME', description: 'site desc' },
    });
    runtime.tree.load({
      pageId: 'page.home',
      elements: [],
      head: { title: 'Home', description: 'page desc' },
    } as never);

    render(
      <RuntimeProvider runtime={runtime}>
        <HeadController document={headDoc()} />
      </RuntimeProvider>,
    );
    act(() => {});
    expect(document.title).toBe('Home — ACME');
    expect(document.head.querySelector('meta[name="description"]')?.getAttribute('content')).toBe('page desc');

    // смена страницы: дерево с новой декларацией → head обновляется без дубликатов
    act(() => {
      runtime.tree.load({
        pageId: 'page.about',
        elements: [],
        head: { title: 'About', description: 'about desc' },
      } as never);
    });
    expect(document.title).toBe('About — ACME');
    expect(document.head.querySelectorAll('meta[name="description"]')).toHaveLength(1);
  });

  it('без head в дереве и без site head → document.head не трогается', () => {
    const runtime = fakeRuntime();
    runtime.tree.load({ pageId: 'page.home', elements: [] } as never);

    render(
      <RuntimeProvider runtime={runtime}>
        <HeadController document={headDoc()} />
      </RuntimeProvider>,
    );
    act(() => {});
    expect(document.title).toBe('');
    expect(document.head.querySelectorAll('meta')).toHaveLength(0);
  });
});