import type { ElementType } from 'react';

export interface SpacerProps {
  as?: ElementType;
  className?: string;
}

/** Пружина: занимает свободное место вдоль main-axis flex-контейнера. */
export function Spacer({ as: Tag = 'div', className }: SpacerProps) {
  const classNames = ['flex-1', className].filter(Boolean).join(' ');
  return <Tag aria-hidden={true} className={classNames} />;
}