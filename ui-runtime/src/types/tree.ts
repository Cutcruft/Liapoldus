export type BindingSource =
  | 'content'
  | 'asset'
  | 'i18n'
  | 'route'
  | 'operation'
  | 'form'
  | 'tokens';

export interface BindingReference {
  /** source контекста: content, i18n, route, operation, form… */
  source: BindingSource;
  /** id сущности внутри source (contentId, section, operationId…) */
  entityId?: string;
  /** dot-path с поддержкой `[]` для обхода массивов */
  path?: string;
  /** статический вариант (например `theme.tokens.primary`) */
  static?: string;
}

export interface Binding {
  /** позиция в props */
  key: string;
  ref: BindingReference;
  /** значение по умолчанию, если binding не резолвится */
  default?: unknown;
}

export interface PropertySlot {
  type: 'props' | 'binding' | 'text' | 'list' | 'children';
  /** для binding: список bindings */
  bindings?: Binding[];
  /** для props: статические props */
  value?: Record<string, unknown>;
  /** для text: ключ strings/content */
  ref?: BindingReference;
  /** для list: декларация перечисления */
  list?: {
    source: BindingReference;
    as?: string;
    children: TreeInstance[];
  };
  /** для children: дочерние инстансы */
  children?: TreeInstance[];
}

export interface TreeInstance {
  /** псевдо-id для auth-ссылок (markers) и updateBindings */
  id: string;
  /** id определения из ComponentRegistry */
  definitionId: string;
  props?: PropertySlot;
}

export interface TreeDeclaration {
  /** корень дерева */
  root: TreeInstance;
}

export interface ResolvedValue {
  value: unknown;
}

export interface BindingContext {
  content: Record<string, unknown>;
  i18n: Record<string, string>;
  route: { path: string; params: Record<string, string> };
  operation:
    | { error: true; message: string }
    | { data: unknown };
  form: Record<string, { status: string; values: Record<string, unknown> }>;
  theme?: Record<string, string>;
}