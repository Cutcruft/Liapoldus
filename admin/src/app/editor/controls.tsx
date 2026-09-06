import type { ReactNode } from 'react';

const BTN =
  'inline-flex items-center gap-1 rounded border border-neutral-300 px-2 py-1 text-xs text-neutral-600 hover:border-blue-300 hover:text-blue-600 disabled:opacity-40 disabled:pointer-events-none';

export function ToolButton({
  onClick,
  disabled,
  title,
  children,
}: {
  onClick: () => void;
  disabled?: boolean;
  title?: string;
  children: ReactNode;
}) {
  return (
    <button type="button" onClick={onClick} disabled={disabled} title={title} className={BTN}>
      {children}
    </button>
  );
}