import { Children, isValidElement, type PropsWithChildren } from 'react';
import type { PaneWidth } from './types';
import { cx } from './classes';

const WIDTHS: Record<PaneWidth, string> = {
  xs: 'w-40',
  sm: 'w-52',
  md: 'w-64',
  lg: 'w-72',
  xl: 'w-80',
};

export interface SidebarProps extends PropsWithChildren {
  /** сторона панели: left (по умолчанию) или right */
  side?: 'left' | 'right';
  width?: PaneWidth;
  className?: string;
  /** класс для основного контента, если нужен свой скролл/фон */
  contentClassName?: string;
}

/** Композиция "панель + контент": первый ребёнок — неподвижная панель, второй — контент. */
export function Sidebar({ side = 'left', width = 'md', className, contentClassName, children }: SidebarProps) {
  const parts = Children.toArray(children).filter(isValidElement);
  return (
    <div className={cx('flex h-full', side === 'right' && 'flex-row-reverse', className)}>
      {parts[0] !== undefined && (
        <div className={cx('shrink-0 grow-0', WIDTHS[width])}>{parts[0]}</div>
      )}
      {parts[1] !== undefined && (
        <div className={cx('grow min-w-0 overflow-auto', contentClassName)}>{parts[1]}</div>
      )}
    </div>
  );
}