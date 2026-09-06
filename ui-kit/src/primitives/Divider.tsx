import type { Spacing } from './types';
import { cx } from './classes';

export interface DividerProps {
  orientation?: 'horizontal' | 'vertical';
  /** внешние отступы вокруг разделителя (my / mx, ключи шкалы Spacing) */
  space?: Spacing;
  className?: string;
}

/** Тонкая линия-разделитель. Горизонтальная растягивается на всю ширину. */
export function Divider({ orientation = 'horizontal', space, className }: DividerProps) {
  const vertical = orientation === 'vertical';
  return (
    <div
      role="separator"
      aria-orientation={vertical ? 'vertical' : 'horizontal'}
      className={cx(
        vertical
          ? ['self-stretch', 'w-px', space !== undefined && `mx-${space}`]
          : ['w-full', 'h-px', space !== undefined && `my-${space}`],
        'bg-border/60',
        className,
      )}
    />
  );
}