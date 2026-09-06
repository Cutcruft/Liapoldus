import { Children, isValidElement, type PropsWithChildren } from 'react';
import { cx } from './classes';

export interface SplitPaneProps extends PropsWithChildren {
  /** horizontal → панели слева/справа; vertical → сверху/снизу */
  direction?: 'horizontal' | 'vertical';
  /** доля первой панели (0.05..0.95), по умолчанию 0.5 */
  ratio?: number;
  /** минимальный размер первой панели (px) */
  min?: number;
  className?: string;
}

const clamp = (n: number) => Math.min(0.95, Math.max(0.05, n));

/** Две панели с фиксированной пропорцией первой (inline flex-basis). */
export function SplitPane({ direction = 'horizontal', ratio = 0.5, min, className, children }: SplitPaneProps) {
  const parts = Children.toArray(children).filter(isValidElement);
  const vertical = direction === 'vertical';
  const basis = `${clamp(ratio) * 100}%`;
  const firstStyle = vertical
    ? { height: basis, minHeight: min }
    : { width: basis, minWidth: min };

  return (
    <div className={cx('flex', vertical ? 'flex-col' : 'flex-row', className)}>
      {parts[0] !== undefined && (
        <div className="grow-0 shrink-0 overflow-auto" style={firstStyle}>
          {parts[0]}
        </div>
      )}
      {parts[1] !== undefined && (
        <div className="grow min-h-0 min-w-0 overflow-auto">{parts[1]}</div>
      )}
    </div>
  );
}