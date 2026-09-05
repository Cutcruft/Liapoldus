import { createElement, type ComponentType, type ReactElement } from 'react';
import type { ResolvedRoute } from '../types/descriptor';
import type { ResolvedTreeInstance } from '../types/tree';
import { useRoute, useTree } from './hooks';

export type ComponentMap = Record<string, ComponentType<any>>;

/** Рендер инстанса дерева: definitionId → компонент из карты, props + children. */
function InstanceNode(props: { instance: ResolvedTreeInstance; components: ComponentMap }): ReactElement | null {
  const { instance, components } = props;
  const Comp = components[instance.definitionId];
  const children = instance.children.map((child) => (
    <InstanceNode key={child.instanceId} instance={child} components={components} />
  ));
  if (!Comp) {
    return createElement('div', { 'data-unknown-component': instance.definitionId }, `Unknown: ${instance.definitionId}`);
  }
  return createElement(Comp, instance.props, children);
}

export interface PageRendererProps {
  /** карта компонентов по definitionId */
  components: ComponentMap;
  /** явная декларация/корень для тестов; по умолчанию — store.tree.root */
  root?: ResolvedTreeInstance | null;
}

/**
 * PageRenderer (§18#1-#3): строит дерево из декларации — root + children в порядке;
 * неизвестный definitionId → placeholder (не падает). Сравнение по instanceId — без remount при rebuild.
 */
export function PageRenderer({ components, root }: PageRendererProps) {
  const tree = useTree();
  const actual = root ?? (tree?.root as ResolvedTreeInstance | undefined) ?? null;
  if (!actual) return null;
  return <InstanceNode instance={actual} components={components} />;
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

function pageIdOf(route: ResolvedRoute | null): string | undefined {
  if (!route) return undefined;
  const action = route.route.action;
  return action.type === 'renderPage' ? action.pageId : undefined;
}