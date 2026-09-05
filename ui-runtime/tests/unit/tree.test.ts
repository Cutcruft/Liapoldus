import { describe, expect, it, vi } from 'vitest';
import { UnknownEntityError } from '../../src/errors';
import { TreeController } from '../../src/core/tree';
import { createRuntimeStore } from '../../src/core/store';
import type { ResolvedRoute, RouteDescriptor } from '../../src/types/descriptor';
import type { TreeDeclaration, TreeInstance } from '../../src/types/tree';

const articleRoute: RouteDescriptor = {
  id: 'route.article',
  matcher: '^/articles/(?<articleId>[0-9]+)$',
  priority: 10,
  action: { type: 'renderPage', pageId: 'page.article' },
};

function instance(over: Partial<TreeInstance> & { instanceId: string }): TreeInstance {
  return { definitionId: 'Node', props: {}, bindings: [], children: [], ...over };
}

function declaration(root: TreeInstance): TreeDeclaration {
  return { root };
}

describe('TreeController (§11)', () => {
  it('load() кладёт декларацию в стор; root доступен', () => {
    const store = createRuntimeStore();
    const controller = new TreeController(store);
    const decl = declaration(instance({ instanceId: 'root' }));
    controller.load(decl);
    expect(store.getState().tree).not.toBeNull();
    expect(store.getState().tree?.root.instanceId).toBe('root');
  });

  it('rebuild с другой декларацией → onRebuild + стор tree обновлён', () => {
    const store = createRuntimeStore();
    const controller = new TreeController(store);
    controller.load(declaration(instance({ instanceId: 'root', definitionId: 'Page' })));
    const spy = vi.fn();
    controller.onRebuild(spy);
    controller.rebuild(declaration(instance({ instanceId: 'root', definitionId: 'PageAlt' })));
    expect(spy).toHaveBeenCalledTimes(1);
    expect(store.getState().tree?.root.definitionId).toBe('PageAlt');
  });

  it('rebuild с идентичной декларацией → no-op', () => {
    const store = createRuntimeStore();
    const controller = new TreeController(store);
    const decl = declaration(instance({ instanceId: 'root' }));
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
    const decl = declaration(
      instance({
        instanceId: 'root',
        children: [instance({ instanceId: 'i2', props: { title: 'Old' } })],
      }),
    );
    controller.load(decl);
    const spy = vi.fn();
    controller.onRebuild(spy);
    controller.updateBindings({ i2: { title: 'New' } });
    const root = store.getState().tree?.root;
    expect(root?.children[0].props.title).toBe('New');
    expect(spy).not.toHaveBeenCalled();
  });

  it('updateBindings для несуществующего instanceId → UnknownEntityError', () => {
    const store = createRuntimeStore();
    const controller = new TreeController(store);
    controller.load(declaration(instance({ instanceId: 'root' })));
    expect(() => controller.updateBindings({ ghost: { x: 1 } })).toThrow(UnknownEntityError);
  });

  it('binding content резолвится: contentId + path из стора → значение', () => {
    const store = createRuntimeStore();
    store.getState().setContent({ c1: { title: 'X' } });
    const controller = new TreeController(store);
    const decl = declaration(
      instance({
        instanceId: 'root',
        props: {},
        bindings: [{ property: 'title', source: { type: 'content', contentId: 'c1', path: 'title' } }],
      }),
    );
    controller.load(decl);
    expect(store.getState().tree?.root.props.title).toBe('X');
  });

  it('binding routeParam резолвится из store.route (params.articleId)', () => {
    const store = createRuntimeStore();
    const resolved: ResolvedRoute = { route: articleRoute, params: { articleId: '42' }, query: {} };
    store.getState().setRoute(resolved);
    const controller = new TreeController(store);
    const decl = declaration(
      instance({
        instanceId: 'root',
        props: {},
        bindings: [{ property: 'articleId', source: { type: 'routeParam', name: 'articleId' } }],
      }),
    );
    controller.load(decl);
    expect(store.getState().tree?.root.props.articleId).toBe('42');
  });

  it('вложенные binding sources (operation, form) резолвятся по правилам', () => {
    const store = createRuntimeStore();
    store.getState().setOperationResult('op1', {
      data: { items: [{ title: 'A' }, { title: 'B' }] },
    });
    store.getState().setFormState('f1', { values: { email: 'a@b.c' } });
    const controller = new TreeController(store);
    const decl = declaration(
      instance({
        instanceId: 'root',
        props: {},
        bindings: [
          { property: 'items', source: { type: 'operation', operationId: 'op1', path: 'items.[].title' } },
          { property: 'email', source: { type: 'form', formId: 'f1', path: 'email' } },
        ],
      }),
    );
    controller.load(decl);
    expect(store.getState().tree?.root.props.items).toEqual(['A', 'B']);
    expect(store.getState().tree?.root.props.email).toBe('a@b.c');
  });

  it('rebuild сохраняет unresolved bindings (не ломает дерево без данных)', () => {
    const store = createRuntimeStore();
    const controller = new TreeController(store);
    const decl = declaration(
      instance({
        instanceId: 'root',
        props: {},
        bindings: [{ property: 'title', source: { type: 'content', contentId: 'missing', path: 'x' } }],
        children: [instance({ instanceId: 'child' })],
      }),
    );
    controller.load(decl);
    controller.rebuild(decl);
    expect(store.getState().tree?.root.instanceId).toBe('root');
    expect(store.getState().tree?.root.children).toHaveLength(1);
    expect(store.getState().tree?.root.props.title).toBeUndefined();
  });
});