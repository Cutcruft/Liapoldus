import { describe, expect, it } from 'vitest';
import { createRuntimeStore } from '../../src/core/store';
import type { AssetMeta } from '../../src/types/asset';
import type { RouteDescriptor } from '../../src/types/descriptor';
import type { TreeDeclaration } from '../../src/types/tree';

const treeA: TreeDeclaration = {
  root: { id: 'root', definitionId: 'Page', props: { type: 'children', children: [] } },
};
const treeB: TreeDeclaration = {
  root: { id: 'root', definitionId: 'PageAlt', props: { type: 'children', children: [] } },
};
const route: RouteDescriptor = {
  kind: 'route',
  id: 'home',
  path: '/',
  matcher: { kind: 'regex', source: '/^\\/$/', priority: 0 },
  operationId: 'page@home',
};
const asset: AssetMeta = {
  id: 'a1',
  name: 'x.png',
  type: 'image/png',
  size: 100,
  variants: [{ name: 'master', url: '/x.png' }],
};

describe('runtime store', () => {
  it('создаётся с дефолтным состоянием', () => {
    const s = createRuntimeStore();
    const st = s.getState();
    expect(st.ready).toBe(false);
    expect(st.tree).toBeNull();
    expect(st.routes).toEqual([]);
    expect(st.tokens).toEqual({});
    expect(st.content).toEqual({});
    expect(st.assets).toEqual({});
    expect(st.locale).toBe('');
  });

  it('setReady() → ready: true', () => {
    const s = createRuntimeStore();
    s.getState().setReady();
    expect(s.getState().ready).toBe(true);
  });

  it('каждое действие обновляет свой срез, не трогая остальные', () => {
    const s = createRuntimeStore();
    const st = s.getState;

    st().setTree(treeA);
    expect(st().tree).toBe(treeA);
    expect(st().tokens).toEqual({});

    st().setRoutes([route]);
    expect(st().routes).toHaveLength(1);
    expect(st().tree).toBe(treeA);

    st().applyTokens({ '--x': '#fff' });
    expect(st().tokens['--x']).toBe('#fff');
    expect(st().routes).toHaveLength(1);

    st().setContent({ c1: { text: 'a' } });
    expect(st().content.c1).toEqual({ text: 'a' });
    expect(st().tokens['--x']).toBe('#fff');

    st().setAssets({ a1: asset });
    expect(st().assets.a1).toBe(asset);
    expect(st().content).toEqual({ c1: { text: 'a' } });

    st().setOperationResult('k1', { v: 1 });
    expect(st().operationResults.k1).toEqual({ v: 1 });
    expect(st().assets).toEqual({ a1: asset });

    st().setLocale('ru');
    expect(st().locale).toBe('ru');
    expect(st().operationResults).toEqual({ k1: { v: 1 } });

    st().setFormState('f1', { values: { email: 'a@b.c' } });
    expect(st().forms.f1).toMatchObject({ status: 'idle', values: { email: 'a@b.c' } });
    expect(st().locale).toBe('ru');
  });

  it('subscribeSlice(content) реагирует только на изменение content, не на setTree', () => {
    const s = createRuntimeStore();
    s.getState().setContent({ c1: { text: 'a' } });
    const calls: string[] = [];
    s.subscribeSlice((st) => st.content, () => calls.push('content'));

    s.getState().setContent({ c1: { text: 'b' } });
    expect(calls).toHaveLength(1);

    s.getState().setTree(treeA);
    expect(calls).toHaveLength(1);
  });

  it('изменение content.c1 не перерисовывает подписчика tree', () => {
    const s = createRuntimeStore();
    s.getState().setTree(treeA);
    s.getState().setContent({ c1: { text: 'a' } });
    const treeCalls: string[] = [];
    s.subscribeSlice((st) => st.tree, () => treeCalls.push('tree'));

    s.getState().setContent({ c1: { text: 'b' } });
    expect(treeCalls).toHaveLength(0);

    s.getState().setTree(treeB);
    expect(treeCalls).toHaveLength(1);
  });

  it('setLocale обновляет срез locale, не трогая content', () => {
    const s = createRuntimeStore();
    const content = { c1: { text: 'a' } };
    s.getState().setContent(content);
    s.getState().setLocale('ru');

    expect(s.getState().locale).toBe('ru');
    expect(s.getState().content).toBe(content);
  });

  it('setAssets пишет в store.assets, не трогая content', () => {
    const s = createRuntimeStore();
    const content = { c1: { text: 'a' } };
    s.getState().setContent(content);
    s.getState().setAssets({ a1: asset });

    expect(s.getState().assets).toEqual({ a1: asset });
    expect(s.getState().content).toBe(content);
  });
});