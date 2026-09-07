import type { ElementNode } from '../../runtime';
import type { JSONSchema } from './schemas';

/**
 * Иммутабельные утилиты линейного листа элементов (§1.3). Все функции
 * возвращают новый массив; элемент идентифицируется по стабильному id.
 * Позиция в листе = порядок рендера; вложенности нет.
 */

export function findElement(list: ElementNode[], id: string): ElementNode | undefined {
  return list.find((el) => el.id === id);
}

export function indexOfElement(list: ElementNode[], id: string): number {
  return list.findIndex((el) => el.id === id);
}

/** Заменяет элемент id на next (без изменения позиции). Возвращает исходный list, если id не найден. */
export function replaceElement(list: ElementNode[], id: string, next: ElementNode): ElementNode[] {
  const idx = indexOfElement(list, id);
  if (idx === -1) return list;
  const out = list.slice();
  out[idx] = next;
  return out;
}

/** Вставляет element в лист на index (default: в конец). */
export function insertElement(list: ElementNode[], element: ElementNode, index?: number): ElementNode[] {
  const at = index === undefined ? list.length : Math.max(0, Math.min(index, list.length));
  const out = list.slice();
  out.splice(at, 0, element);
  return out;
}

/** Удаляет элемент id. Возвращает исходный list, если id не найден. */
export function removeElement(list: ElementNode[], id: string): ElementNode[] {
  const idx = indexOfElement(list, id);
  if (idx === -1) return list;
  const out = list.slice();
  out.splice(idx, 1);
  return out;
}

/** Перемещает элемент id вверх/вниз среди соседей. Возвращает исходный list на границах. */
export function moveElement(list: ElementNode[], id: string, dir: 'up' | 'down'): ElementNode[] {
  const idx = indexOfElement(list, id);
  if (idx === -1) return list;
  const target = dir === 'up' ? idx - 1 : idx + 1;
  if (target < 0 || target >= list.length) return list;
  const out = list.slice();
  const [node] = out.splice(idx, 1);
  out.splice(target, 0, node!);
  return out;
}

/** Первый элемент листа (для дефолтного выбора в плоском редакторе), если есть. */
export function firstId(list: ElementNode[]): string | undefined {
  return list[0]?.id;
}

/** Путь к текстовому прото-значению элемента для панели листа (порядок: label/text/title). */
export function elementSummary(el: ElementNode): string | undefined {
  const prop = (name: string) => {
    const p = el.props[name];
    return p && p.kind === 'literal' && p.value !== undefined && p.value !== null
      ? String(p.value)
      : undefined;
  };
  return prop('label') ?? prop('text') ?? prop('title');
}

let uidCounter = 0;
/** Уникальный id элемента; сбрасывается в тестах через resetUid(). */
export function uid(): string {
  uidCounter += 1;
  return `n${uidCounter}`;
}

export function resetUid(): void {
  uidCounter = 0;
}

/** Создаёт элемент заданного типа (componentId) с литеральными props из дефолтов схемы. */
export function makeElement(componentId: string, schema?: JSONSchema): ElementNode {
  const defaults = schema?.default ?? {};
  const props: ElementNode['props'] = {};
  for (const [k, v] of Object.entries(defaults)) props[k] = { kind: 'literal', value: v };
  return { id: uid(), componentId, props };
}
