import type { ElementType } from 'react';
import type { Align, Justify, PositionalProps, Spacing } from './types';
import { alignClass, cx, gapClass, justifyClass, padClass } from './classes';

export interface StackProps extends PositionalProps {
  as?: ElementType;
  gap?: Spacing;
  align?: Align;
  justify?: Justify;
}

/** Вертикальная колонка (flex-col). */
export function Stack({ as: Tag = 'div', gap, align, justify, pad, className, children }: StackProps) {
  return (
    <Tag
      className={cx(
        'flex flex-col',
        gapClass(gap),
        alignClass(align),
        justifyClass(justify),
        padClass(pad),
        className,
      )}
    >
      {children}
    </Tag>
  );
}