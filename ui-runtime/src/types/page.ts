/**
 * Страница и элементы — линейная модель (§1.3, docs/redesign/backend.md).
 * Страница — упорядоченный список элементов; позиция в листе = порядок рендера.
 * Данные приходят через props-биндинги (source), а не вложенные children.
 */

export type BindingSource =
  | { kind: 'content'; contentId: string; field: string }
  | { kind: 'form'; formId: string }
  | { kind: 'operation'; operationId: string }
  | { kind: 'query'; param: string }
  | { kind: 'routeGroup'; index: number };

export type ElementProp =
  | { kind: 'literal'; value?: unknown }
  | { kind: 'binding'; source: BindingSource };

/**
 * Элемент страницы в декларации/контракте (§1.3): id стабилен (назначается
 * сервером, не меняется при insert/move), componentId — id из ComponentRegistry,
 * props — литералы и/или биндинги.
 */
export interface ElementNode {
  id: string;
  componentId: string;
  props: Record<string, ElementProp>;
  /** явные биндинги элемента (редакторские; резолв идёт по props) */
  bindings?: BindingSource[];
}

/** Страница в runtime-контракте (/runtime/contract `pages[]`). */
export interface PageDescriptor {
  id: string;
  name: string;
  elements: ElementNode[];
}

/** Декларация страницы для загрузки в стор/контроллер (чанк или контракт). */
export interface PageDeclaration {
  snapshotId?: string;
  versionId?: string;
  /** страница, которой принадлежит лист (home-page для контракта) */
  pageId?: string;
  elements: ElementNode[];
}

/** Элемент с резолвленными bindings (props = static + значения из контекста). */
export interface ResolvedElementNode {
  id: string;
  componentId: string;
  props: Record<string, unknown>;
  /** binding-спеки сохраняются (для пересчёта/отладки) */
  bindings?: BindingSource[];
}

/** Разрешённая страница (в сторе). */
export interface ResolvedPageDeclaration {
  snapshotId?: string;
  versionId?: string;
  pageId?: string;
  elements: ResolvedElementNode[];
}

/** Контекст разрешения bindings (срез стора). */
export interface BindingContext {
  content: Record<string, unknown>;
  route: { path: string; params: Record<string, string>; query: Record<string, string>; groups: string[] } | null;
  operation: Record<string, { data?: unknown; error?: boolean }>;
  form: Record<string, { values: Record<string, unknown>; status: string }>;
}