import { describe, expect, it, vi } from 'vitest';
import { UnknownEntityError } from '../../src/errors';
import { TreeController } from '../../src/core/tree';
import { createRuntimeStore } from '../../src/core/store';
import type { ResolvedRoute, RouteDescriptor } from '../../src/types/descriptor';
import type { BindingSource, ElementNode, ElementProp, PageDeclaration } from '../../src/types/page';

const articleRoute: RouteDescriptor = {
  id: 'route.article',
  matcher: '^/articles/(?<articleId>[0-9]+)$',
  priority: 10,
  action: { type: 'renderPage', pageId: 'page.article' },
};

const literal = (value?: unknown): ElementProp => ({ kind: 'literal', value });
const bind = (source: BindingSource): ElementProp => ({ kind: 'binding', source });

function el(id: string, props: Record<string, ElementProp> = {}, over?: Partial<ElementNode>): ElementNode {
  return { id, componentId: 'Node', props, ...over };
}

function declaration(elements: ElementNode[], over: Partial<PageDeclaration> = {}): PageDeclaration {
  return { elements, ...over };
}

describe('TreeController (§11): лист страницы', () => {
  it('load() кладёт декларацию в стор; элементы доступны', () => {
    const store = createRuntimeStore();
    const controller = new TreeController(store);
    const decl = declaration([el('root')]);
    controller.load(decl);
    expect(store.getState().tree).not.toBeNull();
    expect(store.getState().tree?.elements[0].id).toBe('root');
    expect(controller.elements).toHaveLength(1);
  });

  it('rebuild с другой декларацией → onRebuild + стор tree обновлён', () => {
    const store = createRuntimeStore();
    const controller = new TreeController(store);
    controller.load(declaration([el('root'), el('a', { tag: literal('x') })]));
    const spy = vi.fn();
    controller.onRebuild(spy);
    controller.rebuild(declaration([el('root', {}, { componentId: 'PageAlt' })]));
    expect(spy).toHaveBeenCalledTimes(1);
    expect(store.getState().tree?.elements[0].componentId).toBe('PageAlt');
  });

  it('rebuild с идентичной декларацией → no-op', () => {
    const store = createRuntimeStore();
    const controller = new TreeController(store);
    const decl = declaration([el('root')]);
    controller.load(decl);
    const first = store.getState().tree;
    const spy = vi.fn();
    controller.onRebuild(spy);
    controller.rebuild(decl);
    expect(spy).not.toHaveBeenCalled();
    expect(store.getState().tree).toBe(first);
  });

  it('updateBindings({i2}) → обновляет значение без onRebuild', () => {
    const store = createRuntimeStore();
    const controller = new TreeController(store);
    const decl = declaration([el('root'), el('i2', { title: literal('Old') })]);
    controller.load(decl);
    const spy = vi.fn();
    controller.onRebuild(spy);
    controller.updateBindings({ i2: { title: 'New' } });
    expect(store.getState().tree?.elements[1].props.title).toBe('New');
    expect(spy).not.toHaveBeenCalled();
  });

  it('updateBindings для несуществующего id элемента → UnknownEntityError', () => {
    const store = createRuntimeStore();
    const controller = new TreeController(store);
    controller.load(declaration([el('root')]));
    expect(() => controller.updateBindings({ ghost: { x: 1 } })).toThrow(UnknownEntityError);
  });

  it('binding content резолвится: contentId + field из стора → значение', () => {
    const store = createRuntimeStore();
    store.getState().setContent({ c1: { title: 'X' } });
    const controller = new TreeController(store);
    const decl = declaration([el('root', { title: bind({ kind: 'content', contentId: 'c1', field: 'title' }) })]);
    controller.load(decl);
    expect(store.getState().tree?.elements[0].props.title).toBe('X');
  });

  it('binding query резолвится из store.route: query[param] → params[param]', () => {
    const store = createRuntimeStore();
    const resolved: ResolvedRoute = { route: articleRoute, params: { articleId: '42' }, query: { ref: 'r1' } };
    store.getState().setRoute(resolved);
    const controller = new TreeController(store);
    const decl = declaration([
      el('root', { ref: bind({ kind: 'query', param: 'ref' }) }),
      el('q2', { articleId: bind({ kind: 'query', param: 'articleId' }) }),
    ]);
    controller.load(decl);
    expect(store.getState().tree?.elements[0].props.ref).toBe('r1');
    expect(store.getState().tree?.elements[1].props.articleId).toBe('42');
  });

  it('binding routeGroup резолвится из позиционных групп regex', () => {
    const store = createRuntimeStore();
    const resolved: ResolvedRoute = {
      route: articleRoute,
      params: { articleId: '42' },
      query: {},
      groups: ['42'],
    };
    store.getState().setRoute(resolved);
    const controller = new TreeController(store);
    const decl = declaration([el('root', { articleId: bind({ kind: 'routeGroup', index: 0 }) })]);
    controller.load(decl);
    expect(store.getState().tree?.elements[0].props.articleId).toBe('42');
  });

  it('operation и form резолвятся целиком: data → операция, values → форма', () => {
    const store = createRuntimeStore();
    store.getState().setOperationResult('op1', {
      data: { items: [{ title: 'A' }, { title: 'B' }] },
    });
    store.getState().setFormState('f1', { values: { email: 'a@b.c' } });
    const controller = new TreeController(store);
    const decl = declaration([
      el('root', { items: bind({ kind: 'operation', operationId: 'op1' }) }),
      el('f2', { email: bind({ kind: 'form', formId: 'f1' }) }),
    ]);
    controller.load(decl);
    expect(store.getState().tree?.elements[0].props.items).toEqual({
      items: [{ title: 'A' }, { title: 'B' }],
    });
    expect(store.getState().tree?.elements[1].props.email).toEqual({ email: 'a@b.c' });
  });

  it('operation с error=true → значение undefined', () => {
    const store = createRuntimeStore();
    store.getState().setOperationResult('opErr', { data: undefined, error: true });
    const controller = new TreeController(store);
    controller.load(declaration([el('root', { items: bind({ kind: 'operation', operationId: 'opErr' }) })]));
    expect(store.getState().tree?.elements[0].props.items).toBeUndefined();
  });

  it('rebuild сохраняет unresolved bindings (не ломает страницу без данных)', () => {
    const store = createRuntimeStore();
    const controller = new TreeController(store);
    const decl = declaration([
      el('root', { title: bind({ kind: 'content', contentId: 'missing', field: 'x' }) }),
      el('child'),
    ]);
    controller.load(decl);
    controller.rebuild(decl);
    expect(store.getState().tree?.elements).toHaveLength(2);
    expect(store.getState().tree?.elements[0].props.title).toBeUndefined();
  });

  it('refresh() пересчитывает bindings при смене данных без смены декларации', () => {
    const store = createRuntimeStore();
    const controller = new TreeController(store);
    const decl = declaration([el('root', { title: bind({ kind: 'content', contentId: 'c1', field: 'title' }) })]);
    store.getState().setContent({ c1: { title: 'Old' } });
    controller.load(decl);
    expect(store.getState().tree?.elements[0].props.title).toBe('Old');
    store.getState().setContent({ c1: { title: 'New' } });
    controller.refresh();
    expect(store.getState().tree?.elements[0].props.title).toBe('New');
  });
});