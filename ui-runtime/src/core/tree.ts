import { UnknownEntityError } from '../errors';
import type { RuntimeStore } from './store';
import type {
  BindingContext,
  BindingSource,
  ElementNode,
  PageDeclaration,
  ResolvedElementNode,
  ResolvedPageDeclaration,
} from '../types/page';

type TreeListener = () => void;

function getPath(value: unknown, path: string): unknown {
  const tokens = path.split('.').filter((t) => t !== '');
  let v: unknown = value;
  for (let i = 0; i < tokens.length; i++) {
    const t = tokens[i];
    if (t === '[]') {
      if (!Array.isArray(v)) return undefined;
      const rest = tokens.slice(i + 1).join('.');
      return rest ? v.map((item) => getPath(item, rest)) : v.slice();
    }
    if (v === null || typeof v !== 'object') return undefined;
    const rec = v as Record<string, unknown>;
    v = Array.isArray(v) && /^\d+$/.test(t) ? v[Number(t)] : rec[t];
  }
  return v;
}

function operationValue(entry: unknown): unknown {
  if (entry === null || typeof entry !== 'object') return entry;
  const rec = entry as { error?: boolean; data?: unknown };
  if (rec.error === true) return undefined;
  return rec.data !== undefined ? rec.data : entry;
}

/** Резолвит binding-источник против контекста (§1.3) — одно значение на источник. */
function resolveBinding(source: BindingSource, ctx: BindingContext): unknown {
  switch (source.kind) {
    case 'content':
      return getPath(ctx.content[source.contentId], source.field);
    case 'form':
      return ctx.form[source.formId]?.values;
    case 'operation':
      return operationValue(ctx.operation[source.operationId]);
    case 'query':
      return ctx.route?.query[source.param] ?? ctx.route?.params[source.param];
    case 'routeGroup':
      return ctx.route?.groups[source.index];
  }
}

/** Резолвит один элемент: literal → as-is, binding → значение из контекста. */
function resolveElement(el: ElementNode, ctx: BindingContext): ResolvedElementNode {
  const props: Record<string, unknown> = {};
  for (const [name, prop] of Object.entries(el.props)) {
    if (prop.kind === 'binding') {
      const value = resolveBinding(prop.source, ctx);
      if (value !== undefined) props[name] = value;
    } else {
      props[name] = prop.value;
    }
  }
  return { id: el.id, componentId: el.componentId, props, bindings: el.bindings };
}

/** Разрешает декларацию страницы; возвращает новую декларацию с resolved элементами. */
function resolveDeclaration(declaration: PageDeclaration, ctx: BindingContext): ResolvedPageDeclaration {
  return {
    snapshotId: declaration.snapshotId,
    versionId: declaration.versionId,
    pageId: declaration.pageId,
    elements: declaration.elements.map((el) => resolveElement(el, ctx)),
  };
}

/** Управляет страницей: load/rebuild/updateBindings с резолвом bindings (§11). */
export class TreeController {
  private listeners = new Map<'rebuild' | 'update', TreeListener[]>();
  private lastKey = '';
  private raw: PageDeclaration | null = null;

  constructor(private store: RuntimeStore) {}

  private signature(d: PageDeclaration): string {
    return JSON.stringify(d);
  }

  private context(): BindingContext {
    const st = this.store.getState();
    return {
      content: st.content as unknown as Record<string, unknown>,
      route: st.route
        ? {
            path: st.route.route.id,
            params: st.route.params,
            query: st.route.query,
            groups: st.route.groups ?? [],
          }
        : null,
      operation: st.operationResults as BindingContext['operation'],
      form: st.forms,
    };
  }

  load(declaration: PageDeclaration): void {
    this.raw = declaration;
    this.lastKey = this.signature(declaration);
    this.store.getState().setTree(resolveDeclaration(declaration, this.context()));
    this.emit('update');
  }

  rebuild(next: PageDeclaration): void {
    const key = this.signature(next);
    if (key === this.lastKey) return;
    this.raw = next;
    this.lastKey = key;
    this.store.getState().setTree(resolveDeclaration(next, this.context()));
    this.emit('rebuild');
    this.emit('update');
  }

  /** Разрешённый лист элементов текущей страницы (null до load). */
  get elements(): ResolvedElementNode[] | null {
    const tree = this.store.getState().tree;
    return tree ? tree.elements : null;
  }

  /**
   * Пересчитывает bindings текущей страницы против актуального контекста
   * (смена роута/контента/результатов операций/форм) и публикует `update`.
   * В отличие от rebuild не сравнивает сигнатуру декларации — декларация та же,
   * меняются только источники данных.
   */
  refresh(): void {
    if (!this.raw) return;
    this.store.getState().setTree(resolveDeclaration(this.raw, this.context()));
    this.emit('update');
  }

  /** Резолвит декларацию с bindings в разрешённую страницу. */
  resolve(declaration: PageDeclaration, context?: BindingContext): ResolvedPageDeclaration {
    const ctx = context ?? this.context();
    return resolveDeclaration(declaration, ctx);
  }

  /** Обновляет data-значения props по id элемента, не создавая onRebuild (§11#4). */
  updateBindings(patch: Record<string, Record<string, unknown>>): void {
    const current = this.store.getState().tree;
    if (!current) return;
    for (const id of Object.keys(patch)) {
      if (!current.elements.some((el) => el.id === id)) {
        throw new UnknownEntityError(`Элемент '${id}' не найден в странице`);
      }
    }
    const elements = current.elements.map((el) => {
      const values = patch[el.id];
      return values ? { ...el, props: { ...el.props, ...values } } : el;
    });
    this.store.getState().setTree({ ...current, elements });
    this.emit('update');
  }

  onRebuild(listener: TreeListener): () => void {
    return this.on('rebuild', listener);
  }

  onUpdate(listener: TreeListener): () => void {
    return this.on('update', listener);
  }

  private on(event: 'rebuild' | 'update', listener: TreeListener): () => void {
    const list = this.listeners.get(event) ?? [];
    list.push(listener);
    this.listeners.set(event, list);
    return () => {
      this.listeners.set(
        event,
        (this.listeners.get(event) ?? []).filter((l) => l !== listener),
      );
    };
  }

  private emit(event: 'rebuild' | 'update'): void {
    for (const listener of this.listeners.get(event) ?? []) listener();
  }
}