import { lazy, Suspense, useEffect, useRef, useState } from 'react';
import { Inline, Stack } from '@liapoldus/ui-kit';
import type { Translate } from '../../runtime';

const RichTextEditor = lazy(() =>
  import('../rich-text/RichTextEditor').then((m) => ({ default: m.RichTextEditor })),
);

const INPUT_CLASS =
  'rounded border border-neutral-300 px-2 py-1 text-sm focus:border-blue-500 focus:outline-none';

export type FieldRowType = 'string' | 'number' | 'boolean' | 'json' | 'richtext';

export type FieldRow = {
  key: string;
  type: FieldRowType;
  text: string;
  /** Последнее валидное значение (для json/number при недописанном вводе). */
  good?: unknown;
};

export function inferRowType(value: unknown): FieldRowType {
  if (typeof value === 'number') return 'number';
  if (typeof value === 'boolean') return 'boolean';
  if (typeof value === 'object' && value !== null) return 'json';
  return 'string';
}

function valueToText(type: FieldRowType, value: unknown): string {
  if (type === 'json') return JSON.stringify(value, null, 2);
  if (type === 'boolean') return value ? 'true' : 'false';
  return value == null ? '' : String(value);
}

function defaultValue(type: FieldRowType): unknown {
  if (type === 'number') return 0;
  if (type === 'boolean') return false;
  return '';
}

function parseRow(row: FieldRow): { ok: true; value: unknown } | { ok: false } {
  if (row.type === 'number') {
    if (row.text.trim() === '') return { ok: true, value: row.good ?? 0 };
    const n = Number(row.text);
    if (Number.isNaN(n)) return { ok: false };
    return { ok: true, value: n };
  }
  if (row.type === 'boolean') return { ok: true, value: row.text === 'true' };
  if (row.type === 'json') {
    if (row.text.trim() === '') return { ok: true, value: row.good ?? {} };
    try {
      return { ok: true, value: JSON.parse(row.text) };
    } catch {
      return { ok: false };
    }
  }
  return { ok: true, value: row.text };
}

function rowsToObject(rows: FieldRow[]): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const row of rows) {
    const key = row.key.trim();
    if (!key) continue;
    const parsed = parseRow(row);
    out[key] = parsed.ok ? parsed.value : row.good;
  }
  return out;
}

function toRows(value: Record<string, unknown>): FieldRow[] {
  return Object.entries(value).map(([key, v]) => ({
    key,
    type: inferRowType(v),
    text: valueToText(inferRowType(v), v),
    good: v,
  }));
}

/**
 * Редактор произвольного JSON-объекта: строки «ключ / тип / значение» (строка/число/bool/json).
 * Недовалидный ввод (NaN/битый JSON) в результат не попадает — держится предыдущее
 * валидное значение, ошибка показана инлайн.
 */
export function JsonFieldsEditor({
  value,
  onChange,
  t,
  siteId,
}: {
  value: Record<string, unknown>;
  onChange: (next: Record<string, unknown>) => void;
  t: Translate;
  /** Нужен для rich-text полей (AssetPicker); без него такие поля — textarea. */
  siteId?: string;
}) {
  const [rows, setRows] = useState<FieldRow[]>(() => toRows(value));
  const lastEmitted = useRef<Record<string, unknown> | null>(value);

  useEffect(() => {
    if (value === lastEmitted.current) return;
    lastEmitted.current = value;
    setRows(toRows(value));
  }, [value]);

  const update = (next: FieldRow[]): void => {
    setRows(next);
    const obj = rowsToObject(next);
    lastEmitted.current = obj;
    onChange(obj);
  };

  const patchRow = (index: number, patch: Partial<FieldRow>): void => {
    update(rows.map((r, i) => (i === index ? { ...r, ...patch } : r)));
  };

  const addRow = (): void => {
    update([...rows, { key: `field${rows.length + 1}`, type: 'string', text: '', good: '' }]);
  };

  const removeRow = (index: number): void => {
    update(rows.filter((_, i) => i !== index));
  };

  const changeType = (index: number, type: FieldRowType): void => {
    const good = defaultValue(type);
    patchRow(index, { type, text: valueToText(type, good), good });
  };

  const typeLabel = (type: FieldRowType): string =>
    type === 'string'
      ? t('content.type.string')
      : type === 'number'
        ? t('content.type.number')
        : type === 'boolean'
          ? t('content.type.boolean')
          : type === 'richtext'
            ? t('content.type.richtext')
            : t('content.type.json');

  const changeRichText = (index: number, html: string): void => {
    patchRow(index, { text: html });
  };

  return (
    <div className="rounded border border-neutral-200 p-3">
      <Stack gap={2}>
        {rows.map((row, i) => {
          const parsedOk = parseRow({ ...row, text: row.text }).ok;
          const invalid = (row.type === 'number' || row.type === 'json') && !parsedOk;
          const head = (
            <Inline gap={2} align="center">
              <input
                className={`${INPUT_CLASS} w-40`}
                value={row.key}
                aria-label={t('content.fieldKey')}
                onChange={(e) => patchRow(i, { key: e.target.value })}
              />
              <select
                className={INPUT_CLASS}
                value={row.type}
                aria-label={t('content.fieldType')}
                onChange={(e) => changeType(i, e.target.value as FieldRowType)}
              >
                {(['string', 'number', 'boolean', 'json', 'richtext'] as FieldRowType[]).map((type) => (
                  <option key={type} value={type}>
                    {typeLabel(type)}
                  </option>
                ))}
              </select>
              {row.type === 'boolean' ? (
                <input
                  type="checkbox"
                  checked={row.text === 'true'}
                  aria-label={t('content.fieldValue')}
                  onChange={(e) => patchRow(i, { text: String(e.target.checked), good: e.target.checked })}
                />
              ) : null}
              {invalid && (
                <span className="text-xs text-red-500" role="alert">
                  {row.type === 'number' ? 'NaN' : t('content.invalidJson', { key: row.key })}
                </span>
              )}
              <button
                type="button"
                onClick={() => removeRow(i)}
                className="rounded border border-neutral-300 px-2 py-1 text-xs text-neutral-500 hover:border-red-300 hover:text-red-600"
              >
                ✕
              </button>
            </Inline>
          );
          if (row.type === 'richtext') {
            return (
              <div key={`${i}-${row.key}`} className="flex flex-col gap-1">
                {head}
                {siteId ? (
                  <Suspense
                    fallback={
                      <textarea
                        className={`${INPUT_CLASS} min-h-24 w-full`}
                        value={row.text}
                        aria-label={t('content.fieldValue')}
                        onChange={(e) => patchRow(i, { text: e.target.value })}
                      />
                    }
                  >
                    <RichTextEditor value={row.text} onChange={(html) => changeRichText(i, html)} siteId={siteId} />
                  </Suspense>
                ) : (
                  <textarea
                    className={`${INPUT_CLASS} min-h-24 w-full`}
                    value={row.text}
                    aria-label={t('content.fieldValue')}
                    onChange={(e) => patchRow(i, { text: e.target.value })}
                  />
                )}
              </div>
            );
          }
          return (
            <Inline key={`${i}-${row.key}`} gap={2} align="center">
              {head}
              {row.type === 'boolean' ? null : (
                <input
                  className={`${INPUT_CLASS} min-w-0 flex-1 ${invalid ? 'border-red-400' : ''}`}
                  value={row.text}
                  aria-label={t('content.fieldValue')}
                  onChange={(e) => {
                    const parsedNow = parseRow({ ...row, text: e.target.value });
                    patchRow(i, { text: e.target.value, good: parsedNow.ok ? parsedNow.value : row.good });
                  }}
                />
              )}
            </Inline>
          );
        })}
      </Stack>
      <div className="mt-2">
        <button
          type="button"
          onClick={addRow}
          className="rounded border border-neutral-300 px-2 py-1 text-xs text-neutral-600 hover:border-blue-300 hover:text-blue-600"
        >
          + {t('content.addField')}
        </button>
      </div>
    </div>
  );
}