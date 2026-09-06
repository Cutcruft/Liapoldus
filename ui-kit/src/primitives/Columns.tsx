import type { ElementType } from 'react';
import type { PositionalProps, Spacing } from './types';
import { cx, gapClass, padClass } from './classes';

export interface ColumnsProps extends PositionalProps {
  as?: ElementType;
  /** число равных колонок (1..6, вне диапазона — клампится) */
  count?: number;
  gap?: Spacing;
}

/** Сетка равных колонок (grid-cols-N). */
export function Columns({ as: Tag = 'div', count = 1, gap, pad, className, children }: ColumnsProps) {
  const cols = Math.min(6, Math.max(1, Math.floor(count)));
  return (
    <Tag className={cx('grid', `grid-cols-${cols}`, gapClass(gap), padClass(pad), className)}>
      {children}
    </Tag>
  );
}