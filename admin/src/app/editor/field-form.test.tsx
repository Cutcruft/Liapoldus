import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';

afterEach(cleanup);
import { validateValue, type ValidationCode } from './field-form';
import type { JSONSchemaProperty } from './schemas';
import { SchemaForm } from './field-form';

const TEXT_PROP: JSONSchemaProperty = { type: 'string', title: 'Текст', required: true };
const SIZE_PROP: JSONSchemaProperty = { type: 'string', enum: ['sm', 'md', 'lg'] };
const GAP_PROP: JSONSchemaProperty = { type: 'number', min: 0, max: 64 };
const ON_PROP: JSONSchemaProperty = { type: 'boolean' };

describe('validateValue', () => {
  it('required пустое → required', () => {
    expect(validateValue(TEXT_PROP, '')).toBe<ValidationCode>('required');
    expect(validateValue(TEXT_PROP, undefined)).toBe<ValidationCode>('required');
  });

  it('тип-проверки', () => {
    expect(validateValue(TEXT_PROP, 'ok')).toBeNull();
    expect(validateValue(GAP_PROP, 'abc')).toBe<ValidationCode>('type');
    expect(validateValue(ON_PROP, 'yes')).toBe<ValidationCode>('type');
    expect(validateValue(ON_PROP, false)).toBeNull();
  });

  it('enum', () => {
    expect(validateValue(SIZE_PROP, 'xl')).toBe<ValidationCode>('enum');
    expect(validateValue(SIZE_PROP, 'sm')).toBeNull();
  });

  it('диапазон чисел', () => {
    expect(validateValue(GAP_PROP, -1)).toBe<ValidationCode>('range');
    expect(validateValue(GAP_PROP, 100)).toBe<ValidationCode>('range');
    expect(validateValue(GAP_PROP, 8)).toBeNull();
  });
});

const SCHEMA = {
  type: 'object' as const,
  title: 'Текст',
  properties: {
    text: TEXT_PROP,
    size: SIZE_PROP,
    gap: GAP_PROP,
    active: ON_PROP,
  },
};

const sourceLabel = (s: string) => `src:${s}`;

function renderForm(props: Partial<Parameters<typeof SchemaForm>[0]> = {}) {
  const onChange = props.onChange ?? (() => {});
  const onBindingChange = props.onBindingChange ?? (() => {});
  return render(
    <SchemaForm
      schema={SCHEMA}
      values={{ text: 'Привет', size: 'md', gap: 8, active: true }}
      bindings={{}}
      sourceLabel={sourceLabel}
      pathLabel="Путь"
      errorLabel={(c) => `ERR:${c}`}
      onChange={onChange}
      onBindingChange={onBindingChange}
      {...props}
    />,
  );
}

describe('SchemaForm', () => {
  it('рендерит ожидаемые контролы по типам', () => {
    renderForm();
    expect(screen.getByDisplayValue('Привет')).toBeTruthy();
    expect(screen.getByDisplayValue('md')).toBeTruthy();
    expect(screen.getByDisplayValue('8')).toBeTruthy();
    expect(screen.getByRole('checkbox')).toBeTruthy();
  });

  it('показывает live-ошибку для невалидного литерала', () => {
    renderForm({ values: { text: '', size: 'xs', gap: 8, active: true } });
    expect(screen.getByText('ERR:required')).toBeTruthy();
    expect(screen.getByText('ERR:enum')).toBeTruthy();
  });

  it('переключение источника на binding показывает путь', () => {
    const onBindingChange = vi.fn();
    renderForm({ onBindingChange });
    fireEvent.change(screen.getAllByRole('combobox')[0]!, { target: { value: 'content' } });
    expect(onBindingChange).toHaveBeenCalledWith('text', { source: 'content', path: '' });
  });
});