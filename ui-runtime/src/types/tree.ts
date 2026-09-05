/**
 * Дерево страницы и bindings — каноническая форма (json-descriptors.md §5/§6,
 * тест-спека §11). Дерево описывает только структуру; данные — через bindings.
 */

export type BindingSource =
  | { type: 'content'; contentId: string; path: string }
  | { type: 'routeParam'; name: string }
  | { type: 'routeQuery'; name: string }
  | { type: 'operation'; operationId: string; path: string }
  | { type: 'form'; formId: string; path: string }
  | { type: 'props'; path: string }
  | { type: 'runtime'; source: string };

export interface Binding {
  /** позиция в props */
  property: string;
  source: BindingSource;
}

export interface TreeInstance {
  instanceId: string;
  /** id определения из ComponentRegistry */
  definitionId: string;
  /** статические props */
  props: Record<string, unknown>;
  bindings: Binding[];
  children: TreeInstance[];
}

export interface TreeDeclaration {
  snapshotId?: string;
  versionId?: string;
  root: TreeInstance;
}

/** Инстанс с резолвленными bindings (props = static + значения из контекста). */
export interface ResolvedTreeInstance {
  instanceId: string;
  definitionId: string;
  props: Record<string, unknown>;
  /** сами binding-спеки сохраняются (для пересчёта через updateBindings) */
  bindings: Binding[];
  children: ResolvedTreeInstance[];
}

/** Контекст разрешения bindings (срез стора). */
export interface BindingContext {
  content: Record<string, unknown>;
  route: { path: string; params: Record<string, string>; query: Record<string, string> } | null;
  operation: Record<string, { data?: unknown; error?: boolean }>;
  form: Record<string, { values: Record<string, unknown>; status: string }>;
}

export function isResolvedRoot(value: TreeInstance | ResolvedTreeInstance): value is ResolvedTreeInstance {
  return 'children' in value && 'bindings' in value;
}