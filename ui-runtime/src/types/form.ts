export interface FieldSchema {
  type: 'text' | 'email' | 'password' | 'number' | 'select' | 'checkbox' | 'textarea' | 'custom';
  label?: string;
  required?: boolean;
  placeholder?: string;
  values?: string[];
  defaultValue?: unknown;
  rules?: ValidationRule[];
}

export interface ValidationRule {
  id: 'minLength' | 'maxLength' | 'min' | 'max' | 'pattern' | 'custom';
  value?: number | string | RegExp;
  message?: string;
}

export interface FormDefinition {
  id: string;
  name?: string;
  submit: {
    operationId?: string;
    endpointId?: string;
  };
  fields: Record<string, FieldSchema>;
  /** кросс-полевые правила */
  rules?: CrossFieldRule[];
}

export interface CrossFieldRule {
  id: string;
  condition: { field: string; operator: 'eq' | 'notEq' | 'gt' | 'gte' | 'lt' | 'lte'; value: unknown };
  message?: string;
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

export interface FormRuntimeSnapshot {
  status: 'idle' | 'loading' | 'error' | 'submitting' | 'submitted';
  values: Record<string, unknown>;
  errors: FieldValidationError[];
}