import type { ElementType } from 'react';
import type { PositionalProps, Spacing } from './types';
import { cx, gapClass, padClass } from './classes';

export interface GridProps extends PositionalProps {
  as?: ElementType;
  /** число колонок (1..12, клампится) */
  cols?: number;
  /** число строк (1..12, клампится) */
  rows?: number;
  gap?: Spacing;
}

/** Явная сетка grid: колонки/строки/промежутки. */
export function Grid({ as: Tag = 'div', cols, rows, gap, pad, className, children }: GridProps) {
  return (
    <Tag
      className={cx(
        'grid',
        cols !== undefined && `grid-cols-${Math.min(12, Math.max(1, cols))}`,
        rows !== undefined && `grid-rows-${Math.min(12, Math.max(1, rows))}`,
        gapClass(gap),
        padClass(pad),
        className,
      )}
    >
      {children}
    </Tag>
  );
}