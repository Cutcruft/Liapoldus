import type { Align, Justify, Pad, Spacing } from './types';

type ClassValue = string | false | null | undefined | ClassValue[];

/** Схлопывает классы (в т.ч. вложенные массивы), отбрасывая falsy-значения. */
export function cx(...parts: ClassValue[]): string {
  const out: string[] = [];
  for (const part of parts) {
    if (Array.isArray(part)) {
      out.push(cx(...part));
    } else if (part) {
      out.push(part);
    }
  }
  return out.join(' ');
}

/** `gap-<n>` для Tailwind: 1 → 4px. Отсутствующий → ''. */
export function gapClass(gap: Spacing | undefined): string {
  return gap === undefined ? '' : `gap-${gap}`;
}

/** `items-<align>` flex/grid cross-axis. */
export function alignClass(align: Align | undefined): string {
  return align === undefined ? '' : `items-${align}`;
}

/** `justify-<justify>` flex/grid main-axis. */
export function justifyClass(justify: Justify | undefined): string {
  return justify === undefined ? '' : `justify-${justify}`;
}

/** `p-*`/`px-*`/`py-*` из Pad (единое значение или {x, y}). */
export function padClass(pad: Pad | undefined): string {
  if (pad === undefined) return '';
  if (typeof pad === 'number') return `p-${pad}`;
  return cx(pad.x !== undefined && `px-${pad.x}`, pad.y !== undefined && `py-${pad.y}`);
}