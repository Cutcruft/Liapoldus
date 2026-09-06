import type { ComponentNode } from '../../runtime';
import type { JSONSchema } from './schemas';

/** Иммутабельные утилиты дерева компонентов. Все функции возвращают новый Node. */

export function findNode(tree: ComponentNode, id: string): ComponentNode | undefined {
  if (tree.id === id) return tree;
  for (const child of tree.children ?? []) {
    const hit = findNode(child, id);
    if (hit) return hit;
  }
  return undefined;
}

export function findParent(tree: ComponentNode, id: string): ComponentNode | undefined {
  for (const child of tree.children ?? []) {
    if (child.id === id) return tree;
    const hit = findParent(child, id);
    if (hit) return hit;
  }
  return undefined;
}

/** Заменяет узел id на next (children next берутся целиком). */
export function replaceNode(tree: ComponentNode, id: string, next: ComponentNode): ComponentNode {
  if (tree.id === id) return next;
  const children = tree.children ?? [];
  const nextChildren = children.map((c) => replaceNode(c, id, next));
  return { ...tree, children: nextChildren };
}

/** Вставляет child в parentId на index (default: в конец). Возвращает новый tree. */
export function insertChild(
  tree: ComponentNode,
  parentId: string,
  child: ComponentNode,
  index?: number,
): ComponentNode {
  if (!findNode(tree, parentId)) return tree;
  return insertChildInner(tree, parentId, child, index);
}

function insertChildInner(
  tree: ComponentNode,
  parentId: string,
  child: ComponentNode,
  index?: number,
): ComponentNode {
  if (tree.id === parentId) {
    const children = tree.children ?? [];
    const at = index === undefined ? children.length : Math.max(0, Math.min(index, children.length));
    const next = [...children];
    next.splice(at, 0, child);
    return { ...tree, children: next };
  }
  const children = tree.children ?? [];
  const nextChildren = children.map((c) => insertChildInner(c, parentId, child, index));
  return { ...tree, children: nextChildren };
}

/** Удаляет узел id (не root). Возвращает новый tree. */
export function removeNode(tree: ComponentNode, id: string): ComponentNode {
  if (tree.id === id) return tree;
  const children = tree.children ?? [];
  const kept: ComponentNode[] = [];
  for (const child of children) {
    if (child.id !== id) kept.push(removeNode(child, id));
  }
  return { ...tree, children: kept };
}

/** Перемещает узел id вверх/вниз среди соседей одного родителя. */
export function moveNode(tree: ComponentNode, id: string, dir: 'up' | 'down'): ComponentNode {
  const parent = findParent(tree, id);
  if (!parent) return tree;
  const siblings = parent.children ?? [];
  const idx = siblings.findIndex((c) => c.id === id);
  const target = dir === 'up' ? idx - 1 : idx + 1;
  if (idx === -1 || target < 0 || target >= siblings.length) return tree;
  const next = [...siblings];
  const [node] = next.splice(idx, 1);
  next.splice(target, 0, node!);
  return replaceNode(tree, parent.id, { ...parent, children: next });
}

/** Путь от корня до узла (включая корень). */
export function nodePath(tree: ComponentNode, id: string): ComponentNode[] {
  if (tree.id === id) return [tree];
  for (const child of tree.children ?? []) {
    const rest = nodePath(child, id);
    if (rest.length > 0) return [tree, ...rest];
  }
  return [];
}

let uidCounter = 0;
/** Уникальный id узла; сбрасывается в тестах через resetUid(). */
export function uid(): string {
  uidCounter += 1;
  return `n${uidCounter}`;
}

export function resetUid(): void {
  uidCounter = 0;
}

/** Создаёт узел заданного типа с props и пустыми bindings. */
export function makeNode(type: string, props: Record<string, unknown> = {}, schema?: JSONSchema): ComponentNode {
  return { id: uid(), type, props: { ...(schema?.default ?? {}), ...props }, bindings: {} };
}