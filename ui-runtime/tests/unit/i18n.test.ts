import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { ApiClient } from '../../src/core/api-client';
import { registerBuiltin } from '../../src/core/builtin/descriptors';
import { ContentController } from '../../src/core/content';
import { I18n, type I18nEnv } from '../../src/core/i18n';
import { createRuntimeStore } from '../../src/core/store';
import { RuntimeRegistry } from '../../src/core/registry';
import { TransportFactory } from '../../src/core/transport/factory';
import { LocaleUnsupportedError } from '../../src/errors';
import type { TreeDeclaration } from '../../src/types/tree';
import { makeFakeFetch, resetFakes } from './helpers';

const BUILTIN = 'liapoldus.builtin';

const treeA: TreeDeclaration = {
  root: { id: 'root', definitionId: 'Page', props: { type: 'children', children: [] } },
};

const stringsByLocale: Record<string, Record<string, string>> = {
  ru: { home: 'Главная', favorites: 'Избранное' },
  de: { home: 'Start', favorites: 'Favoriten' },
};

function buildBackend() {
  return makeFakeFetch((call) => {
    const url = new URL(call.url, 'http://runtime.local');
    const path = url.pathname;
    const method = call.init.method ?? 'GET';

    const m = path.match(/^\/api\/sites\/([^/]+)\/contents$/);
    if (m && method === 'GET') {
      const collectionId = url.searchParams.get('collectionId') ?? undefined;
      const locale = url.searchParams.get('locale') ?? undefined;
      if (collectionId !== 'strings') return [];
      const fields = locale != null && stringsByLocale[locale]
        ? stringsByLocale[locale]
        : stringsByLocale.ru;
      return [{ id: 'nav', collectionId: 'strings', fields }];
    }
    return null;
  });
}

function api(registry: RuntimeRegistry, fetch: ReturnType<typeof makeFakeFetch>['fetch']) {
  const transport = new TransportFactory({ fetch }).create(registry.getProvider(BUILTIN)!);
  return new ApiClient(registry, { transport, scope: 'public' });
}

function fakeStorage(init: Record<string, string>): NonNullable<I18nEnv['storage']> {
  return {
    getItem: (k) => init[k] ?? null,
    setItem: (k, v) => {
      init[k] = v;
    },
  };
}

afterEach(() => resetFakes());
beforeEach(() => resetFakes());

describe('i18n', () => {
  it('detect(): localStorage приоритетнее navigator.language/default', () => {
    const store = createRuntimeStore();
    const storage = fakeStorage({ 'liapoldus.locale': 'ru' });
    const i18n = new I18n(
      { defaultLocale: 'de', supportedLocales: ['en', 'ru', 'de'] },
      store,
      { storage, navigatorLanguage: 'en-US' },
    );

    i18n.detect();

    expect(store.getState().locale).toBe('ru');
    expect(storage.getItem('liapoldus.locale')).toBe('ru');
  });

  it('detect(): пустой localStorage → navigator.language (нормализуется)', () => {
    const store = createRuntimeStore();
    const i18n = new I18n(
      { defaultLocale: 'ru', supportedLocales: ['en', 'ru'] },
      store,
      { storage: fakeStorage({}), navigatorLanguage: 'en-US' },
    );

    i18n.detect();

    expect(store.getState().locale).toBe('en');
  });

  it('detect(): ни localStorage, ни navigator → defaultLocale', () => {
    const store = createRuntimeStore();
    const i18n = new I18n(
      { defaultLocale: 'ru', supportedLocales: ['en', 'ru'] },
      store,
      { storage: fakeStorage({}) },
    );

    i18n.detect();

    expect(store.getState().locale).toBe('ru');
  });

  it('setLocale("ru-RU") → нормализованная "ru", запись в storage, syncContent("ru")', () => {
    const store = createRuntimeStore();
    const storage = fakeStorage({});
    const called: string[] = [];
    const i18n = new I18n(
      { defaultLocale: 'en', supportedLocales: ['en', 'ru'], syncContent: (l) => void called.push(l) },
      store,
      { storage },
    );

    i18n.setLocale('ru-RU');

    expect(store.getState().locale).toBe('ru');
    expect(storage.getItem('liapoldus.locale')).toBe('ru');
    expect(called).toEqual(['ru']);
  });

  it('resolveLocale("en-GB") → "en" при supported ["en","ru"]', () => {
    const i18n = new I18n({ defaultLocale: 'ru', supportedLocales: ['en', 'ru'] }, createRuntimeStore());
    expect(i18n.resolveLocale('en-GB')).toBe('en');
  });

  it('resolveLocale("xx") при пустых supportedLocales → принимается', () => {
    const i18n = new I18n({ defaultLocale: 'en', supportedLocales: [] }, createRuntimeStore());
    expect(i18n.resolveLocale('xx')).toBe('xx');
  });

  it('resolveLocale неизвестная + strict → LocaleUnsupportedError', () => {
    const i18n = new I18n(
      { defaultLocale: 'en', supportedLocales: ['en', 'ru'], strict: true },
      createRuntimeStore(),
    );
    expect(() => i18n.resolveLocale('xx')).toThrow(LocaleUnsupportedError);
  });

  it('t("nav.home") читает строку из store.content', () => {
    const store = createRuntimeStore();
    store.getState().setContent({ nav: { home: 'Главная', favorites: 'Избранное' } });
    store.getState().setLocale('ru');
    const i18n = new I18n({ defaultLocale: 'ru' }, store);

    expect(i18n.t('nav.home')).toBe('Главная');
  });

  it('t() несуществующий ключ → возвращает ключ', () => {
    const store = createRuntimeStore();
    const i18n = new I18n({ defaultLocale: 'ru' }, store);

    expect(i18n.t('no.such.key')).toBe('no.such.key');
  });

  it('t(key, params) → интерполяция {name}', () => {
    const store = createRuntimeStore();
    store.getState().setContent({ greet: { default: 'Привет, {name}!' } });
    const i18n = new I18n({ defaultLocale: 'ru' }, store);

    expect(i18n.t('greet.default', { name: 'Liapoldus' })).toBe('Привет, Liapoldus!');
  });

  it('onLocaleChange() вызывается при setLocale', () => {
    const store = createRuntimeStore();
    const i18n = new I18n({ defaultLocale: 'ru' }, store);
    const seen: string[] = [];
    i18n.onLocaleChange((l) => seen.push(l));

    i18n.setLocale('ru');

    expect(seen).toEqual(['ru']);
  });

  it('строки попадают в store.content через content.list; setLocale переспрашивает', async () => {
    const registry = new RuntimeRegistry();
    registerBuiltin(registry);
    const { fetch } = buildBackend();
    const store = createRuntimeStore();
    const controller = new ContentController(store, api(registry, fetch).builtin.content, { siteId: 'site1' });
    const i18n = new I18n(
      {
        defaultLocale: 'ru',
        supportedLocales: ['ru', 'de'],
        syncContent: (locale) => {
          return controller.list({ collectionId: 'strings', locale }).then(() => undefined);
        },
      },
      store,
      { storage: fakeStorage({}), navigatorLanguage: 'de-DE' },
    );

    await i18n.setLocale('ru');

    expect(store.getState().content.nav).toEqual({ home: 'Главная', favorites: 'Избранное' });
  });

  it('пере-синк строк не пересобирает дерево (slice tree не меняется)', async () => {
    const registry = new RuntimeRegistry();
    registerBuiltin(registry);
    const { fetch } = buildBackend();
    const store = createRuntimeStore();
    store.getState().setTree(treeA);
    const controller = new ContentController(store, api(registry, fetch).builtin.content, { siteId: 'site1' });
    const i18n = new I18n(
      {
        defaultLocale: 'ru',
        supportedLocales: ['ru', 'de'],
        syncContent: (locale) => {
          return controller.list({ collectionId: 'strings', locale }).then(() => undefined);
        },
      },
      store,
      { storage: fakeStorage({}) },
    );

    const treeCalls: string[] = [];
    const contentCalls: string[] = [];
    store.subscribeSlice((st) => st.tree, () => treeCalls.push('tree'));
    store.subscribeSlice((st) => st.content, () => contentCalls.push('content'));

    await i18n.setLocale('de');

    expect(contentCalls.length).toBeGreaterThan(0);
    expect(treeCalls).toHaveLength(0);
    expect(store.getState().tree).toBe(treeA);
  });
});