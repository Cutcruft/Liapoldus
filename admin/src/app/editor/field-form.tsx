import type { BindingSource, ElementProp } from '../../runtime';
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

/**
 * Источники биндинга (линейная модель §1.3). 'literal' — хранить значение как есть;
 * остальные указывают на runtime-источник. Значение опционально для routeGroup.
 */
export const BINDING_SOURCES = ['literal', 'content', 'form', 'operation', 'query', 'routeGroup'] as const;
export type BindingSourceName = (typeof BINDING_SOURCES)[number];

const SELECT_CLASS =
  'rounded border border-neutral-300 px-1.5 py-1 text-xs text-neutral-700 focus:border-blue-500 focus:outline-none';

const INPUT_CLASS =
  'w-full rounded border border-neutral-300 px-2 py-1 text-sm focus:border-blue-500 focus:outline-none';
const ERROR_CLASS = 'text-xs text-red-600';

/** Пустой источник для вида binding (data приходит из runtime-источника). */
function emptySource(kind: Exclude<BindingSourceName, 'literal'>): BindingSource {
  switch (kind) {
    case 'content':
      return { kind: 'content', contentId: '', field: '' };
    case 'form':
      return { kind: 'form', formId: '' };
    case 'operation':
      return { kind: 'operation', operationId: '' };
    case 'query':
      return { kind: 'query', param: '' };
    case 'routeGroup':
      return { kind: 'routeGroup', index: 1 };
  }
}

function sourceKind(source?: BindingSource): BindingSourceName {
  return source ? source.kind : 'literal';
}

function bindingLabel(source?: BindingSource): string {
  if (!source) return '';
  switch (source.kind) {
    case 'content':
      return `${source.contentId}.${source.field}`;
    case 'form':
      return source.formId;
    case 'operation':
      return source.operationId;
    case 'query':
      return source.param;
    case 'routeGroup':
      return String(source.index);
  }
}

/** Поле ввода для конкретного типа биндинг-источника. */
function BindingInput({
  source,
  onChange,
  placeholder,
}: {
  source: BindingSource;
  onChange: (source: BindingSource) => void;
  placeholder: string;
}) {
  const label = bindingLabel(source);
  return (
    <input
      aria-label={placeholder}
      className={INPUT_CLASS}
      value={label}
      placeholder={placeholder}
      onChange={(e) => {
        const raw = e.target.value;
        switch (source.kind) {
          case 'content': {
            // Поле вида "contentId.field"
            const dot = raw.lastIndexOf('.');
            onChange({
              kind: 'content',
              contentId: dot === -1 ? raw : raw.slice(0, dot),
              field: dot === -1 ? '' : raw.slice(dot + 1),
            });
            break;
          }
          case 'query':
            onChange({ kind: 'query', param: raw });
            break;
          case 'routeGroup':
            onChange({ kind: 'routeGroup', index: Number(raw) || 1 });
            break;
          default:
            onChange({ ...source, [source.kind === 'form' ? 'formId' : 'operationId']: raw } as BindingSource);
        }
      }}
    />
  );
}

/**
 * Форма свойств из JSON-Schema: литеральные значения + bindings к источнику.
 * В отличие от старой формы работает напрямую с props-картой ElementProp
 * (`{kind:'literal',value}` или `{kind:'binding',source}`).
 */
export function SchemaForm({
  schema,
  props,
  sourceLabel,
  pathLabel,
  errorLabel,
  onChange,
  onBindingChange,
  renderAssetField,
  renderRichTextField,
}: {
  schema: JSONSchema;
  props: Record<string, ElementProp>;
  sourceLabel: (source: BindingSourceName) => string;
  pathLabel: string;
  errorLabel: (code: ValidationCode) => string;
  onChange: (field: string, value: unknown) => void;
  onBindingChange: (field: string, source: BindingSource) => void;
  /** Поле с format:'asset' рендерится переданным контролом (asset-picker). */
  renderAssetField?: (value: unknown, onChange: (value: unknown) => void) => React.ReactNode;
  /** Поле с format:'richtext' рендерится переданным Tiptap-контролом. */
  renderRichTextField?: (value: unknown, onChange: (value: unknown) => void) => React.ReactNode;
}) {
  const fields = Object.entries(schema.properties);

  return (
    <div className="flex flex-col gap-3">
      {fields.length === 0 && <p className="text-xs text-neutral-400">—</p>}
      {fields.map(([field, prop]) => {
        const propDef = props[field] ?? { kind: 'literal' as const };
        const kind = sourceKind(propDef.kind === 'binding' ? propDef.source : undefined);
        const isBound = kind !== 'literal';
        const source = propDef.kind === 'binding' ? propDef.source : undefined;
        const literalValue = propDef.kind === 'literal' && propDef.value !== undefined ? propDef.value : prop.default;
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
                value={kind}
                onChange={(e) => {
                  const s = e.target.value as BindingSourceName;
                  if (s === 'literal') onChange(field, literalValue);
                  else onBindingChange(field, emptySource(s));
                }}
              >
                {BINDING_SOURCES.map((s) => (
                  <option key={s} value={s}>
                    {sourceLabel(s)}
                  </option>
                ))}
              </select>
            </div>

            {isBound && source ? (
              <BindingInput
                source={source}
                onChange={(next) => onBindingChange(field, next)}
                placeholder={pathLabel}
              />
            ) : prop.format === 'asset' && renderAssetField ? (
              <div>{renderAssetField(literalValue, (v) => onChange(field, v))}</div>
            ) : prop.format === 'richtext' && renderRichTextField ? (
              <div>{renderRichTextField(literalValue, (v) => onChange(field, v))}</div>
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