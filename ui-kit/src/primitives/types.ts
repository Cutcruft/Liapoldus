import type { ReactNode } from 'react';

/**
 * Общие типы layout-примитивов пакета @liapoldus/ui-kit.
 * Все значения маппятся на статические Tailwind-утилиты (v4); call-site пишет
 * только props, без CSS.
 */

/** Шкала отступов/промежутков: значение × 4px (совпадает с Tailwind spacing). */
export type Spacing = 0 | 1 | 2 | 3 | 4 | 5 | 6 | 8 | 10 | 12 | 16;

/** Вертикальное выравнивание flex/grid-контейнера → `items-*`. */
export type Align = 'start' | 'center' | 'end' | 'stretch' | 'baseline';

/** Горизонтальное распределение flex-контейнера → `justify-*`. */
export type Justify = 'start' | 'center' | 'end' | 'between' | 'around' | 'evenly';

/** Паддинг: единое значение или {x, y}. */
export type Pad = Spacing | { x?: Spacing; y?: Spacing };

/** Размер стороны Sidebar → фиксированные `w-*`. */
export type PaneWidth = 'xs' | 'sm' | 'md' | 'lg' | 'xl';

/** Позиционные props, общие для контейнеров. */
export interface PositionalProps {
  pad?: Pad;
  className?: string;
  children?: ReactNode;
}