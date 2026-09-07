import type { ReactNode } from 'react';

/** Подпись + контент поля формы; implicit-association для getByLabelText. */
export function Field({
  label,
  required,
  className,
  children,
}: {
  label: string;
  required?: boolean;
  className?: string;
  children: ReactNode;
}) {
  return (
    <label className={`flex flex-col gap-1 text-sm ${className ?? ''}`}>
      <span className="text-neutral-500">
        {label}
        {required ? ' *' : ''}
      </span>
      {children}
    </label>
  );
}