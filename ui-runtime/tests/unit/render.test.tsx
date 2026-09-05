import { act, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';
import type { ComponentType, ReactNode } from 'react';
import type { BootRuntime } from '../../src/core/boot';
import { PageRenderer, RouteOutlet, RuntimeProvider } from '../../src/react';
import type { TreeDeclaration } from '../../src/types/tree';
import { bootFromContract, routeHome, routeRedirect } from './react-harness';
import { resetFakes } from './helpers';

function Scaffold({ runtime, children }: { runtime: BootRuntime; children: ReactNode }) {
  return <RuntimeProvider runtime={runtime}>{children}</RuntimeProvider>;
}

type ElementProps = { title?: string; label?: string; children?: ReactNode };

const ROOT: Record<string, ComponentType<ElementProps>> = {
  'page.home': ({ title, children }) => (
    <section data-testid="root" data-title={String(title)}>
      {children}
    </section>
  ),
  box: ({ title, children }) => (
    <div data-testid="box" data-title={String(title)}>
      {children}
    </div>
  ),
  item: ({ label }) => <span data-testid="item">{String(label)}</span>,
};

function treeWith(): TreeDeclaration {
  return {
    snapshotId: 's1',
    versionId: 'v1',
    root: {
      instanceId: 'root',
      definitionId: 'page.home',
      props: { title: 'Root title' },
      bindings: [],
      children: [
        {
          instanceId: 'box-a',
          definitionId: 'box',
          props: { title: 'A' },
          bindings: [],
          children: [{ instanceId: 'item-1', definitionId: 'item', props: { label: 'L1' }, bindings: [], children: [] }],
        },
        {
          instanceId: 'box-b',
          definitionId: 'box',
          props: { title: 'B' },
          bindings: [],
          children: [],
        },
        {
          instanceId: 'box-c',
          definitionId: 'box',
          props: { title: 'C' },
          bindings: [],
          children: [],
        },
      ],
    },
  };
}

function treeB(): TreeDeclaration {
  return {
    ...treeWith(),
    snapshotId: 's2',
    versionId: 'v2',
  };
}

describe('18. render (PageRenderer / RouteOutlet)', () => {
  beforeEach(() => {
    resetFakes();
  });

  it('1. PageRenderer строит дерево: root + children в правильном порядке', async () => {
    const { runtime } = await bootFromContract();
    act(() => runtime.store.getState().setTree(treeWith()));

    render(
      <Scaffold runtime={runtime}>
        <PageRenderer components={ROOT} />
      </Scaffold>,
    );
    await waitFor(() => expect(screen.getByTestId('root').getAttribute('data-title')).toBe('Root title'));

    const boxes = screen.getAllByTestId('box');
    expect(boxes.map((b) => b.getAttribute('data-title'))).toEqual(['A', 'B', 'C']);
    expect(boxes[0].contains(screen.getByTestId('item'))).toBe(true);
  });

  it('2. каждый instanceId получает props + резолвленные bindings', async () => {
    const { runtime } = await bootFromContract();
    const decl = treeWith();
    // резолвленный binding: title берётся из контента
    decl.root.props = { title: 'Fallback' };
    decl.root.bindings = [{ property: 'title', source: { type: 'content', contentId: 'labels', path: 'root' } }];
    runtime.store.getState().setContent({ labels: { root: 'Из контента' } });
    act(() => runtime.tree.load(decl));

    render(
      <Scaffold runtime={runtime}>
        <PageRenderer components={ROOT} />
      </Scaffold>,
    );
    const rootEl = await screen.findByTestId('root');
    await waitFor(() => expect(rootEl.getAttribute('data-title')).toBe('Из контента'));
    await waitFor(() => expect(screen.getByTestId('item').textContent).toBe('L1'));
  });

  it('3. неизвестный definitionId → placeholder, дерево не падает', async () => {
    const { runtime } = await bootFromContract();
    const decl = treeWith();
    const child = decl.root.children.find((c) => c.instanceId === 'box-c');
    if (child) child.definitionId = 'missing.component';
    act(() => runtime.store.getState().setTree(decl));

    render(
      <Scaffold runtime={runtime}>
        <PageRenderer components={ROOT} />
      </Scaffold>,
    );
    await waitFor(() =>
      expect(document.querySelector('[data-unknown-component="missing.component"]')).toBeTruthy(),
    );
    // соседние узлы продолжают рендериться
    expect(screen.getAllByTestId('box').length).toBeGreaterThanOrEqual(1);
  });

  it('4. RouteOutlet рендерит pageId по текущему роуту', async () => {
    const { runtime } = await bootFromContract({ routes: [routeHome()] });

    function Home() {
      return <div data-testid="home">Home page</div>;
    }

    render(
      <Scaffold runtime={runtime}>
        <div data-testid="wrapper">
          <RouteOutlet pages={{ 'page.home': Home }} fallback={() => <span>none</span>} />
        </div>
      </Scaffold>,
    );
    await waitFor(() => expect(screen.getByTestId('wrapper').textContent).toBe('none'));

    act(() => runtime.router.navigate('/'));
    await waitFor(() => expect(screen.getByTestId('home')).toBeTruthy());
  });

  it('5. redirect-роут при навигации выполняет переход (страница для старого пути не рендерится)', async () => {
    const { runtime } = await bootFromContract({ routes: [routeHome(), routeRedirect()] });

    function Home() {
      return <div data-testid="home">Home page</div>;
    }

    render(
      <Scaffold runtime={runtime}>
        <RouteOutlet pages={{}} fallback={() => <span>fallback</span>} />
        <RouteOutlet pages={{ 'page.home': Home }} />
      </Scaffold>,
    );

    act(() => runtime.router.navigate('/old'));
    await waitFor(() => expect(screen.getByTestId('home')).toBeTruthy());
    // переход прошёл через redirect: итоговый роут — home (renderPage), а не legacy
    expect(runtime.store.getState().route?.route.id).toBe('home');
  });

  it('6. одинаковые декларации не вызывают remount (сравнение по instanceId)', async () => {
    const { runtime } = await bootFromContract();
    act(() => runtime.store.getState().setTree(treeWith()));

    render(
      <Scaffold runtime={runtime}>
        <PageRenderer components={ROOT} />
      </Scaffold>,
    );
    await waitFor(() => expect(screen.getByTestId('root').getAttribute('data-title')).toBe('Root title'));
    const elBefore = screen.getByTestId('item');

    // payload изменился (snapshot v2), но instanceId те же → элемент не пересоздаётся
    act(() => runtime.store.getState().setTree(treeB()));
    await waitFor(() => expect(screen.getByTestId('item')).toBe(elBefore));
  });
});