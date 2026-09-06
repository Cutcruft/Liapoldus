import type { BindingSource } from '../../runtime';
import type { JSONSchema, JSONSchemaProperty } from './schemas';

export type ValidationCode = 'required' | 'type' | 'enum' | 'range';

/** Live-валидация литерального значения по описанию свойства (R4-подмножество). */
export function validateValue(prop: JSONSchemaProperty, value: unknown): ValidationCode | null {
  const empty = value === undefined || value === null || value === '';
  if (prop.required && empty) return 'required';
  if (empty) return null;

  switch (prop.type) {
    case 'string':
      if (typeof value !== 'string') return 'type';
      if (prop.enum && !prop.enum.includes(value)) return 'enum';
      return null;
    case 'number': {
      if (typeof value !== 'number' || Number.isNaN(value)) return 'type';
      if (prop.min !== undefined && value < prop.min) return 'range';
      if (prop.max !== undefined && value > prop.max) return 'range';
      return null;
    }
    case 'boolean':
      return typeof value === 'boolean' ? null : 'type';
  }
}

export const BINDING_SOURCES = ['literal', 'content', 'route', 'query', 'operation', 'form'] as const;
export type BindingSourceName = (typeof BINDING_SOURCES)[number];

const SELECT_CLASS =
  'rounded border border-neutral-300 px-1.5 py-1 text-xs text-neutral-700 focus:border-blue-500 focus:outline-none';

const INPUT_CLASS =
  'w-full rounded border border-neutral-300 px-2 py-1 text-sm focus:border-blue-500 focus:outline-none';
const ERROR_CLASS = 'text-xs text-red-600';

/** Форма свойств из JSON-Schema: литеральные значения + bindings к источнику. */
export function SchemaForm({
  schema,
  values,
  bindings,
  sourceLabel,
  pathLabel,
  errorLabel,
  onChange,
  onBindingChange,
  renderAssetField,
}: {
  schema: JSONSchema;
  values: Record<string, unknown>;
  bindings: Record<string, BindingSource>;
  sourceLabel: (source: BindingSourceName) => string;
  pathLabel: string;
  errorLabel: (code: ValidationCode) => string;
  onChange: (field: string, value: unknown) => void;
  onBindingChange: (field: string, binding: BindingSource) => void;
  /** Поле с format:'asset' рендерится переданным контролом (asset-picker). */
  renderAssetField?: (value: unknown, onChange: (value: unknown) => void) => React.ReactNode;
}) {
  const fields = Object.entries(schema.properties);

  return (
    <div className="flex flex-col gap-3">
      {fields.length === 0 && <p className="text-xs text-neutral-400">—</p>}
      {fields.map(([field, prop]) => {
        const binding: BindingSource = bindings[field] ?? { source: 'literal' };
        const isBound = binding.source !== 'literal';
        const literalValue = values[field] ?? prop.default;
        const error = isBound ? null : validateValue(prop, literalValue);

        return (
          <div key={field} className="flex flex-col gap-1">
            <div className="flex items-center gap-2">
              <label className="flex-1 text-sm text-neutral-700">
                {prop.title ?? field}
                {prop.required && <span className="ml-0.5 text-red-600">*</span>}
              </label>
              <select
                aria-label={sourceLabel('literal')}
                className={SELECT_CLASS}
                value={binding.source}
                onChange={(e) => {
                  const source = e.target.value as BindingSourceName;
                  if (source === 'literal') onBindingChange(field, { source: 'literal' });
                  else onBindingChange(field, { source, path: binding.source === source ? binding.path ?? '' : '' });
                }}
              >
                {BINDING_SOURCES.map((s) => (
                  <option key={s} value={s}>
                    {sourceLabel(s)}
                  </option>
                ))}
              </select>
            </div>

            {isBound ? (
              <input
                aria-label={pathLabel}
                className={INPUT_CLASS}
                value={binding.path ?? ''}
                placeholder={pathLabel}
                onChange={(e) => onBindingChange(field, { source: binding.source, path: e.target.value })}
              />
            ) : prop.format === 'asset' && renderAssetField ? (
              <div>{renderAssetField(literalValue, (v) => onChange(field, v))}</div>
            ) : (
              <LiteralInput prop={prop} value={literalValue} onChange={(v) => onChange(field, v)} />
            )}

            {error && <p className={ERROR_CLASS}>{errorLabel(error)}</p>}
          </div>
        );
      })}
    </div>
  );
}

function LiteralInput({
  prop,
  value,
  onChange,
}: {
  prop: JSONSchemaProperty;
  value: unknown;
  onChange: (value: unknown) => void;
}) {
  if (prop.type === 'boolean') {
    return (
      <input
        type="checkbox"
        checked={value === true}
        onChange={(e) => onChange(e.target.checked)}
        className="h-4 w-4"
      />
    );
  }
  if (prop.type === 'string' && prop.enum) {
    return (
      <select className={INPUT_CLASS} value={String(value ?? '')} onChange={(e) => onChange(e.target.value)}>
        <option value="">—</option>
        {prop.enum.map((option) => (
          <option key={option} value={option}>
            {option}
          </option>
        ))}
      </select>
    );
  }
  if (prop.type === 'number') {
    return (
      <input
        type="number"
        className={INPUT_CLASS}
        value={value === undefined || value === null ? '' : Number(value)}
        onChange={(e) => {
          const raw = e.target.value;
          onChange(raw === '' ? undefined : Number(raw));
        }}
      />
    );
  }
  return (
    <input
      className={INPUT_CLASS}
      value={value === undefined || value === null ? '' : String(value)}
      onChange={(e) => onChange(e.target.value)}
    />
  );
}