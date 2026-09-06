import type { ElementType } from 'react';
import type { Align, Justify, PositionalProps, Spacing } from './types';
import { alignClass, cx, gapClass, justifyClass, padClass } from './classes';

export interface InlineProps extends PositionalProps {
  as?: ElementType;
  gap?: Spacing;
  align?: Align;
  justify?: Justify;
  wrap?: boolean;
}

/** Горизонтальный ряд (flex-row), при `wrap` — перенос строк. */
export function Inline({ as: Tag = 'div', gap, align, justify, wrap, pad, className, children }: InlineProps) {
  return (
    <Tag
      className={cx(
        'flex flex-row',
        gapClass(gap),
        alignClass(align),
        justifyClass(justify),
        wrap && 'flex-wrap',
        padClass(pad),
        className,
      )}
    >
      {children}
    </Tag>
  );
}