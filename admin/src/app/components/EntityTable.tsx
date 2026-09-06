import type { ReactNode } from 'react';

export interface Column<T> {
  key: string;
  label: string;
  className?: string;
  render: (row: T) => ReactNode;
}

/** Простая таблица: строка заголовков + строки данных, сетка колонок задаётся классом. */
export function EntityTable<T>({
  columns,
  rows,
  gridClass,
  getKey,
}: {
  columns: Column<T>[];
  rows: T[];
  gridClass: string;
  getKey: (row: T) => string;
}) {
  return (
    <div className="overflow-hidden rounded border border-neutral-200">
      <div className={`grid items-center gap-3 border-b border-neutral-200 bg-neutral-50 px-4 py-2 text-xs font-medium text-neutral-500 ${gridClass}`}>
        {columns.map((c) => (
          <span key={c.key}>{c.label}</span>
        ))}
      </div>
      {rows.map((row) => (
        <div key={getKey(row)} className={`grid items-center gap-3 border-b border-neutral-100 px-4 py-2 text-sm ${gridClass}`}>
          {columns.map((c) => (
            <span key={c.key} className={c.className}>
              {c.render(row)}
            </span>
          ))}
        </div>
      ))}
    </div>
  );
}