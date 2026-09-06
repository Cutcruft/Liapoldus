import type { ElementType } from 'react';
import type { PositionalProps } from './types';
import { cx, padClass } from './classes';

export interface BoxProps extends PositionalProps {
  as?: ElementType;
  grow?: boolean;
}

/** Базовый блок с паддингом и опцией растяжения в flex-контейнере. */
export function Box({ as: Tag = 'div', pad, grow, className, children }: BoxProps) {
  return (
    <Tag className={cx(padClass(pad), grow && 'grow', className)}>{children}</Tag>
  );
}