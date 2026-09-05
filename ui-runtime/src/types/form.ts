/** Типы форм: канонический формат из json-descriptors.md §10 (fields — массив, cross-field validation, submit.target). */

export type FieldType = 'text' | 'email' | 'password' | 'number' | 'select' | 'checkbox' | 'textarea' | 'custom';

export interface ValidationRule {
  id: 'minLength' | 'maxLength' | 'min' | 'max' | 'pattern' | 'custom';
  value?: number | string | RegExp;
  message?: string;
}

export interface FieldSchema {
  name: string;
  type: FieldType;
  label?: string;
  required?: boolean;
  placeholder?: string;
  options?: string[];
  defaultValue?: unknown;
  rules?: ValidationRule[];
}

/** Кросс-полевое правило: e.g. `{ id: 'confirmMatch', fields: ['password','password2'] }`. */
export interface CrossFieldRule {
  id: string;
  fields: string[];
  message?: string;
}

/** submit.target: `endpoint.<id>` | `operation.<id>` | голый id (по умолчанию — endpoint). */
export interface FormSubmitConfig {
  target: string;
  providerId?: string;
}

export interface FormDefinition {
  id: string;
  name?: string;
  fields: FieldSchema[];
  validation?: CrossFieldRule[];
  submit: FormSubmitConfig;
}

export interface FieldValidationError {
  fieldId: string;
  ruleId: string;
  message?: string;
}

export interface ValidationResult {
  valid: boolean;
  errors: FieldValidationError[];
}

export type FormSubmitResult = { submissionId: string; status: 'ok' };

/** Снимок формы для UI. */
export interface FormRuntimeSnapshot {
  status: 'idle' | 'loading' | 'error' | 'submitting' | 'submitted';
  values: Record<string, unknown>;
  errors: Record<string, string[]>;
  definition?: FormDefinition;
}