import { UnknownEntityError } from '../errors';
import type { RuntimeStore } from './store';
import type {
  BindingContext,
  BindingSource,
  ResolvedTreeInstance,
  TreeDeclaration,
  TreeInstance,
} from '../types/tree';

export interface TreeOptions {
  /** резолв binding source `runtime` */
  resolveRuntime?: (source: string) => unknown;
}

type TreeListener = () => void;

/** Разрешает декларацию; возвращает новую декларацию с resolved root. */
function resolveDeclaration(
  declaration: TreeDeclaration,
  ctx: BindingContext,
  resolveRuntime?: (source: string) => unknown,
): TreeDeclaration {
  return {
    ...declaration,
    root: resolveInstance(declaration.root, ctx, resolveRuntime),
  };
}

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

function resolveBinding(
  source: BindingSource,
  ctx: BindingContext,
  parentProps: Record<string, unknown>,
  resolveRuntime?: (source: string) => unknown,
): unknown {
  switch (source.type) {
    case 'content':
      return getPath(ctx.content[source.contentId], source.path);
    case 'routeParam':
      return ctx.route?.params[source.name];
    case 'routeQuery':
      return ctx.route?.query[source.name];
    case 'operation':
      return getPath(operationValue(ctx.operation[source.operationId]), source.path);
    case 'form':
      return getPath(ctx.form[source.formId]?.values, source.path);
    case 'props':
      return getPath(parentProps, source.path);
    case 'runtime':
      return resolveRuntime?.(source.source);
  }
}

function resolveInstance(
  instance: TreeInstance,
  ctx: BindingContext,
  resolveRuntime?: (source: string) => unknown,
): ResolvedTreeInstance {
  let props: Record<string, unknown> = { ...instance.props };
  for (const binding of instance.bindings) {
    const value = resolveBinding(binding.source, ctx, props, resolveRuntime);
    if (value !== undefined) {
      props = { ...props, [binding.property]: value };
    }
  }
  const children = instance.children.map((child) =>
    resolveInstance(child, ctx, resolveRuntime),
  );
  return {
    instanceId: instance.instanceId,
    definitionId: instance.definitionId,
    props,
    bindings: instance.bindings,
    children,
  };
}

function findInstance(root: ResolvedTreeInstance, instanceId: string): ResolvedTreeInstance | null {
  if (root.instanceId === instanceId) return root;
  for (const child of root.children) {
    const found = findInstance(child, instanceId);
    if (found) return found;
  }
  return null;
}

function cloneTree(root: ResolvedTreeInstance): ResolvedTreeInstance {
  return {
    ...root,
    props: { ...root.props },
    children: root.children.map(cloneTree),
  };
}

/** Управляет деревом страницы: load/rebuild/updateBindings с резолвом bindings (§11). */
export class TreeController {
  private listeners = new Map<'rebuild' | 'update', TreeListener[]>();
  private lastKey = '';
  private resolveRuntime?: (source: string) => unknown;

  constructor(
    private store: RuntimeStore,
    opts?: TreeOptions,
  ) {
    this.resolveRuntime = opts?.resolveRuntime;
  }

  private signature(d: TreeDeclaration): string {
    return JSON.stringify(d);
  }

  private context(): BindingContext {
    const st = this.store.getState();
    return {
      content: st.content as unknown as Record<string, unknown>,
      route: st.route
        ? { path: st.route.route.id, params: st.route.params, query: st.route.query }
        : null,
      operation: st.operationResults as BindingContext['operation'],
      form: st.forms,
    };
  }

  load(declaration: TreeDeclaration): void {
    this.lastKey = this.signature(declaration);
    this.store.getState().setTree(resolveDeclaration(declaration, this.context(), this.resolveRuntime));
    this.emit('update');
  }

  rebuild(next: TreeDeclaration): void {
    const key = this.signature(next);
    if (key === this.lastKey) return;
    this.lastKey = key;
    this.store.getState().setTree(resolveDeclaration(next, this.context(), this.resolveRuntime));
    this.emit('rebuild');
    this.emit('update');
  }

  get root(): ResolvedTreeInstance | null {
    const tree = this.store.getState().tree;
    return tree ? (tree.root as ResolvedTreeInstance) : null;
  }

  /** Резолвит декларацию с bindings в разрешённое дерево. */
  resolve(declaration: TreeDeclaration, context?: BindingContext): TreeDeclaration {
    const ctx = context ?? this.context();
    return resolveDeclaration(declaration, ctx, this.resolveRuntime);
  }

  /** Обновляет data-значения props по instanceId, не создавая onRebuild (§11#4). */
  updateBindings(patch: Record<string, Record<string, unknown>>): void {
    const current = this.store.getState().tree;
    if (!current) return;
    const clone = cloneTree(current.root as ResolvedTreeInstance);
    for (const instanceId of Object.keys(patch)) {
      if (!findInstance(clone, instanceId)) {
        throw new UnknownEntityError(`Инстанс '${instanceId}' не найден в дереве`);
      }
    }
    for (const [instanceId, values] of Object.entries(patch)) {
      const target = findInstance(clone, instanceId);
      if (target) {
        target.props = { ...target.props, ...values };
      }
    }
    this.store.getState().setTree({ ...current, root: clone });
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