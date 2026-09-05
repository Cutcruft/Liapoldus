import { DescriptorValidationError, FormValidationError } from '../errors';
import type {
  FieldSchema,
  FieldValidationError,
  FormDefinition,
  FormSubmitResult,
  ValidationResult,
} from '../types/form';
import type { BuiltinApi } from './builtin/builtin-client';
import type { RuntimeStore } from './store';

export interface FormRuntimeOptions {
  localeProvider?: () => string;
  submittedAt?: () => string;
}

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

/**
 * Формы (спека §7d): определение из `form.get`, валидация только на клиенте,
 * submit — единственная запись на сервер (`form.submit`, raw JSON).
 */
export class FormRuntime {
  constructor(
    private readonly store: RuntimeStore,
    private readonly api: BuiltinApi,
    private readonly opts?: FormRuntimeOptions,
  ) {}

  async load(formId: string): Promise<FormDefinition> {
    const existing = this.store.getState().forms[formId]?.definition;
    if (existing) return existing;
    const definition = (await this.api.query('form.get', { formId })) as FormDefinition;
    this.store.getState().setFormState(formId, { definition });
    return definition;
  }

  fields(formId: string): FieldSchema[] {
    return this.definitionOf(formId).fields;
  }

  /** Клиентская валидация по схеме из `form.get`. */
  validate(formId: string, values: Record<string, unknown>): ValidationResult {
    const def = this.definitionOf(formId);
    const errors: FieldValidationError[] = [];
    for (const field of def.fields) {
      errors.push(...validateField(field, values[field.name]));
    }
    for (const cross of def.validation ?? []) {
      errors.push(...validateCrossField(cross, values));
    }
    return { valid: errors.length === 0, errors };
  }

  /** Submit: клиентская валидация → `callEndpoint(form.submit)` с raw JSON. */
  async submit(formId: string, values: Record<string, unknown>): Promise<FormSubmitResult> {
    let def = this.store.getState().forms[formId]?.definition;
    if (!def) def = await this.load(formId);

    const validated = this.validate(formId, values);
    const errorsMap = toMap(validated.errors);
    if (!validated.valid) {
      this.store.getState().setFormState(formId, { status: 'error', values, errors: errorsMap });
      throw new FormValidationError(`Форма '${formId}' не прошла клиентскую валидацию`);
    }

    this.store.getState().setFormState(formId, { status: 'submitting', values, errors: {} });
    const locale = this.opts?.localeProvider ? this.opts.localeProvider() : this.store.getState().locale;
    const submittedAt = this.opts?.submittedAt ? this.opts.submittedAt() : new Date().toISOString();
    const payload = { formId, locale, submittedAt, values };

    try {
      const res = (await this.api.callEndpoint('form.submit', payload)) as FormSubmitResult;
      this.store.getState().setFormState(formId, { status: 'submitted', values, errors: {} });
      return res;
    } catch (err) {
      this.store.getState().setFormState(formId, { status: 'error', values, errors: errorsMap });
      throw err;
    }
  }

  reset(formId: string): void {
    this.store.getState().setFormState(formId, { status: 'idle', values: {}, errors: {} });
  }

  private definitionOf(formId: string): FormDefinition {
    const def = this.store.getState().forms[formId]?.definition;
    if (!def) {
      throw new DescriptorValidationError(`Определение формы '${formId}' не загружено (form.get)`);
    }
    return def;
  }
}

function validateField(field: FieldSchema, value: unknown): FieldValidationError[] {
  const errors: FieldValidationError[] = [];
  const empty = value === undefined || value === null || value === '';

  if (field.required && empty) {
    errors.push({ fieldId: field.name, ruleId: 'required', message: `Поле '${field.name}' обязательно` });
    return errors;
  }
  if (empty) return errors;

  if (field.type === 'email' && typeof value === 'string' && !EMAIL_RE.test(value)) {
    errors.push({ fieldId: field.name, ruleId: 'email', message: `Неверный email в поле '${field.name}'` });
  }
  if (field.type === 'select' && field.options && field.options.length > 0 && !field.options.includes(String(value))) {
    errors.push({ fieldId: field.name, ruleId: 'option', message: `Недопустимое значение в поле '${field.name}'` });
  }

  for (const rule of field.rules ?? []) {
    const check = checkRule(rule.id, value, rule.value);
    if (check) {
      errors.push({
        fieldId: field.name,
        ruleId: rule.id,
        message: rule.message ?? `Поле '${field.name}' не прошло проверку '${rule.id}'`,
      });
    }
  }
  return errors;
}

function checkRule(id: string, value: unknown, expected: unknown): boolean {
  switch (id) {
    case 'minLength':
      return typeof value === 'string' && typeof expected === 'number' && value.length < expected;
    case 'maxLength':
      return typeof value === 'string' && typeof expected === 'number' && value.length > expected;
    case 'min':
      return typeof value === 'number' && typeof expected === 'number' && value < expected;
    case 'max':
      return typeof value === 'number' && typeof expected === 'number' && value > expected;
    case 'pattern': {
      if (typeof value !== 'string') return false;
      const re = expected instanceof RegExp ? expected : new RegExp(String(expected));
      return !re.test(value);
    }
    default:
      return false;
  }
}

function validateCrossField(rule: { id: string; fields: string[]; message?: string }, values: Record<string, unknown>): FieldValidationError[] {
  if (rule.id !== 'confirmMatch' || rule.fields.length < 2) return [];
  const [first, ...rest] = rule.fields;
  const base = String(values[first] ?? '');
  const differs = rest.some((f) => String(values[f] ?? '') !== base);
  if (!differs) return [];
  return [{ fieldId: rest[0], ruleId: 'confirmMatch', message: rule.message ?? 'Значения полей не совпадают' }];
}

function toMap(errors: FieldValidationError[]): Record<string, string[]> {
  const out: Record<string, string[]> = {};
  for (const e of errors) {
    (out[e.fieldId] ??= []).push(e.message ?? e.ruleId);
  }
  return out;
}