import { act, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';
import type { ComponentType, ReactNode } from 'react';
import type { BootRuntime } from '../../src/core/boot';
import { PageRenderer, RouteOutlet, RuntimeProvider } from '../../src/react';
import type { ElementProp, PageDeclaration, ResolvedPageDeclaration } from '../../src/types/page';
import { bootFromContract, routeHome, routeRedirect } from './react-harness';
import { resetFakes } from './helpers';

function Scaffold({ runtime, children }: { runtime: BootRuntime; children: ReactNode }) {
  return <RuntimeProvider runtime={runtime}>{children}</RuntimeProvider>;
}

type ElementProps = { title?: string; label?: string };

const ROOT: Record<string, ComponentType<ElementProps>> = {
  'page.home': ({ title }) => <section data-testid="root" data-title={String(title)} />,
  box: ({ title }) => <div data-testid="box" data-title={String(title)} />,
  item: ({ label }) => <span data-testid="item">{String(label)}</span>,
};

const L = (value?: unknown): ElementProp => ({ kind: 'literal', value });

function treeWith(): PageDeclaration {
  return {
    snapshotId: 's1',
    versionId: 'v1',
    elements: [
      { id: 'root', componentId: 'page.home', props: { title: L('Root title') } },
      { id: 'box-a', componentId: 'box', props: { title: L('A') } },
      { id: 'box-b', componentId: 'box', props: { title: L('B') } },
      { id: 'box-c', componentId: 'box', props: { title: L('C') } },
      { id: 'item-1', componentId: 'item', props: { label: L('L1') } },
    ],
  };
}

function treeB(): ResolvedPageDeclaration {
  return {
    snapshotId: 's2',
    versionId: 'v2',
    elements: [
      { id: 'root', componentId: 'page.home', props: { title: 'Root title' } },
      { id: 'box-a', componentId: 'box', props: { title: 'A' } },
      { id: 'box-b', componentId: 'box', props: { title: 'B' } },
      { id: 'box-c', componentId: 'box', props: { title: 'C' } },
      { id: 'item-1', componentId: 'item', props: { label: 'L1' } },
    ],
  };
}

describe('18. render (PageRenderer / RouteOutlet)', () => {
  beforeEach(() => {
    resetFakes();
  });

  it('1. PageRenderer рендерит лист элементов в порядке следования', async () => {
    const { runtime } = await bootFromContract();
    act(() => runtime.store.getState().setTree(treeB()));

    render(
      <Scaffold runtime={runtime}>
        <PageRenderer components={ROOT} />
      </Scaffold>,
    );
    await waitFor(() => expect(screen.getByTestId('root').getAttribute('data-title')).toBe('Root title'));

    const boxes = screen.getAllByTestId('box');
    expect(boxes.map((b) => b.getAttribute('data-title'))).toEqual(['A', 'B', 'C']);
    expect(screen.getByTestId('item').textContent).toBe('L1');
  });

  it('2. каждый элемент получает props + резолвленные bindings', async () => {
    const { runtime } = await bootFromContract();
    const decl = treeWith();
    // резолвленный binding: title берётся из контента
    decl.elements[0].props = {
      title: { kind: 'binding', source: { kind: 'content', contentId: 'labels', field: 'root' } },
    };
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

  it('3. неизвестный componentId → placeholder, страница не падает', async () => {
    const { runtime } = await bootFromContract();
    const decl = treeWith();
    const target = decl.elements.find((c) => c.id === 'box-c');
    if (target) target.componentId = 'missing.component';
    act(() => runtime.store.getState().setTree(decl));

    render(
      <Scaffold runtime={runtime}>
        <PageRenderer components={ROOT} />
      </Scaffold>,
    );
    await waitFor(() =>
      expect(document.querySelector('[data-unknown-component="missing.component"]')).toBeTruthy(),
    );
    // соседние элементы продолжают рендериться
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

  it('6. одинаковые декларации не вызывают remount (сравнение по element.id)', async () => {
    const { runtime } = await bootFromContract();
    act(() => runtime.store.getState().setTree(treeB()));

    render(
      <Scaffold runtime={runtime}>
        <PageRenderer components={ROOT} />
      </Scaffold>,
    );
    await waitFor(() => expect(screen.getByTestId('root').getAttribute('data-title')).toBe('Root title'));
    const elBefore = screen.getByTestId('item');

    // payload изменился (snapshot v2), но element.id те же → элемент не пересоздаётся
    act(() => runtime.store.getState().setTree(treeB()));
    await waitFor(() => expect(screen.getByTestId('item')).toBe(elBefore));
  });

  it('7. layout-section оборачивает лист в layout-компонент с children (§1.3)', async () => {
    const { runtime } = await bootFromContract();
    const withLayout = { ...treeB(), layoutSectionId: 'layout.shell' };
    act(() => runtime.store.getState().setTree(withLayout));

    const Layout = ({ children }: { children?: ReactNode }) => (
      <div data-testid="layout">
        <header>header</header>
        {children}
        <footer>footer</footer>
      </div>
    );
    const map = { ...ROOT, 'layout.shell': Layout };

    render(
      <Scaffold runtime={runtime}>
        <PageRenderer components={map} />
      </Scaffold>,
    );
    await waitFor(() => expect(screen.getByTestId('layout')).toBeTruthy());
    expect(screen.getByTestId('layout').textContent).toContain('header');
    // лист страницы внутри layout (children)
    expect(screen.getByTestId('layout').querySelector('[data-testid="root"]')).not.toBeNull();
    expect(screen.getByTestId('layout').textContent).toContain('footer');
  });

  it('8. layout с неизвестным компонентом → голый лист (не падает)', async () => {
    const { runtime } = await bootFromContract();
    act(() => runtime.store.getState().setTree({ ...treeB(), layoutSectionId: 'layout.missing' }));

    render(
      <Scaffold runtime={runtime}>
        <PageRenderer components={ROOT} />
      </Scaffold>,
    );
    await waitFor(() => expect(screen.getByTestId('root')).toBeTruthy());
    // лист рендерится без обёртки layout
    expect(screen.queryByTestId('layout')).toBeNull();
  });
});