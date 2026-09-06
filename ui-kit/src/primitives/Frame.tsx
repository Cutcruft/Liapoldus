import type { CSSProperties, ElementType } from 'react';
import type { Pad, PositionalProps } from './types';
import { cx, padClass } from './classes';

export interface FrameProps extends PositionalProps {
  as?: ElementType;
  /** Высота: число → фикс. px (inline style), 'full' → h-full (по умолчанию). */
  height?: number | 'full';
  overflow?: 'auto' | 'hidden' | 'scroll' | 'visible';
}

/** Скролл-область: заливка контейнера, прозрачная к своему скроллбару. */
export function Frame({ as: Tag = 'div', height = 'full', overflow = 'auto', pad, className, children }: FrameProps) {
  const style: CSSProperties | undefined =
    typeof height === 'number' ? { height } : undefined;
  return (
    <Tag
      style={style}
      className={cx(
        `overflow-${overflow}`,
        height === 'full' && 'h-full',
        padClass(pad),
        className,
      )}
    >
      {children}
    </Tag>
  );
}