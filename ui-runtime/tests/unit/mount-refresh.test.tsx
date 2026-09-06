import { act, render } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { createRuntimeStore } from '../../src/core/store';
import { TreeController } from '../../src/core/tree';
import { RuntimeProvider } from '../../src/react/context';
import { PageRenderer } from '../../src/react/render';
import { componentMapFromRegistry, registerBuiltinComponents } from '../../src/react/builtin';
import type { BootRuntime } from '../../src/core/boot';
import type { ResolvedRoute, RouteDescriptor } from '../../src/types/descriptor';
import type { TreeDeclaration } from '../../src/types/tree';

function fakeRuntime(): BootRuntime {
  const store = createRuntimeStore();
  const tree = new TreeController(store);
  const runtime = {
    siteId: 'site',
    environment: 'development',
    ready: true,
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
  return runtime;
}

describe('TreeController.refresh (Этап 5: пересборка vs синк данных)', () => {
  it('refresh пересчитывает content-binding после обновления контента', () => {
    registerBuiltinComponents();
    const runtime = fakeRuntime();
    const declaration: TreeDeclaration = {
      root: {
        instanceId: 'root',
        definitionId: 'Text',
        props: {},
        bindings: [{ property: 'text', source: { type: 'content', contentId: 'hero', path: 'title' } }],
        children: [],
      },
    };
    runtime.tree.load(declaration);
    runtime.store.getState().setContent({ hero: { title: 'Привет' } } as never);

    const { container } = render(
      <RuntimeProvider runtime={runtime}>
        <PageRenderer components={componentMapFromRegistry()} />
      </RuntimeProvider>,
    );
    const span = container.querySelector('span[data-component="Text"]') as HTMLElement;
    expect(span.innerHTML).toBe('');

    act(() => {
      runtime.tree.refresh();
    });
    expect(span.innerHTML).toContain('Привет');
  });

  it('rebuild резолвит routeParam-binding против актуального роута', () => {
    const runtime = fakeRuntime();
    const routeDescriptor: RouteDescriptor = {
      id: 'r1',
      matcher: '^/p/(?<id>[^/]+)$',
      priority: 10,
      action: { type: 'renderPage', pageId: 'home' },
    };
    const route: ResolvedRoute = { route: routeDescriptor, params: { id: '42' }, query: {} };
    runtime.store.getState().setRoute(route);
    runtime.tree.rebuild({
      root: {
        instanceId: 'root',
        definitionId: 'Text',
        props: {},
        bindings: [{ property: 'text', source: { type: 'routeParam', name: 'id' } }],
        children: [],
      },
    });
    const root = runtime.store.getState().tree?.root as { props: Record<string, unknown> };
    expect(root.props['text']).toBe('42');
  });

  it('refresh no-op без дерева (не падает)', () => {
    const store = createRuntimeStore();
    const tree = new TreeController(store);
    expect(() => tree.refresh()).not.toThrow();
  });
});