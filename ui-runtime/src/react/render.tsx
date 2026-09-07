import { createElement, type ComponentType, type ReactElement } from 'react';
import type { ResolvedRoute } from '../types/descriptor';
import type { ResolvedElementNode } from '../types/page';
import { useRoute, useTree } from './hooks';

export type ComponentMap = Record<string, ComponentType<any>>;

/** Рендер элемента страницы: componentId → компонент из карты, props из resolved-элемента. */
function ElementNode(props: { element: ResolvedElementNode; components: ComponentMap }): ReactElement | null {
  const { element, components } = props;
  const Comp = components[element.componentId];
  if (!Comp) {
    return createElement('div', { 'data-unknown-component': element.componentId }, `Unknown: ${element.componentId}`);
  }
  return createElement(Comp, element.props);
}

export interface PageRendererProps {
  /** карта компонентов по componentId */
  components: ComponentMap;
  /** явный лист элементов для тестов; по умолчанию — store.tree.elements */
  elements?: ResolvedElementNode[] | null;
}

/**
 * PageRenderer (§18#1-#3): рендерит линейный лист элементов страницы в порядке
 * (позиция в листе = порядок рендера); неизвестный componentId → placeholder (не падает).
 *
 * R10 P1: если у страницы есть layout (effective layoutSectionId из
 * ResolvedPageDeclaration — per-page override уже смержен с site-дефолтом на
 * уровне декларации), лист оборачивается в layout-компонент с `children`
 * (спека §1.3: layout — section с `acceptsPageContent`, получает готовый
 * линейный список) или рендерится голым, когда layout-компонент неизвестен.
 * Ключи по element.id — без remount при rebuild.
 */
export function PageRenderer({ components, elements }: PageRendererProps) {
  const tree = useTree();
  const actual = elements ?? tree?.elements ?? null;
  if (!actual || actual.length === 0) return null;

  const list = (
    <>
      {actual.map((element) => (
        <ElementNode key={element.id} element={element} components={components} />
      ))}
    </>
  );

  const layoutId = elements ? undefined : tree?.layoutSectionId;
  const Layout = layoutId ? components[layoutId] : undefined;
  if (Layout) {
    return createElement(Layout, { children: list });
  }
  return list;
}

export interface RouteOutletProps {
  /** pageId → компонент страницы */
  pages: Record<string, ComponentType<Record<string, unknown>>>;
  fallback?: ComponentType<Record<string, unknown>> | null;
}

/** RouteOutlet (§18#4-#5): рендерит страницу по pageId текущего роута. */
export function RouteOutlet({ pages, fallback }: RouteOutletProps) {
  const route = useRoute();
  const pageId = pageIdOf(route);
  const Page = pageId ? pages[pageId] : undefined;
  if (!Page) {
    return fallback ? createElement(fallback, { route: route as ResolvedRoute | undefined }) : null;
  }
  return createElement(Page, { route: route ?? undefined });
}

/** pageId активного renderPage-роута (для догрузчика код-сплит чанков). */
export function pageIdOf(route: ResolvedRoute | null): string | undefined {
  if (!route) return undefined;
  const action = route.route.action;
  return action.type === 'renderPage' ? action.pageId : undefined;
}