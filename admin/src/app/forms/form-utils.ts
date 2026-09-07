import type { FormDefinition } from '../../runtime';

/** Человекочитаемое значение поля payload (скаляр/JSON). */
export function formatPayloadValue(value: unknown): string {
  if (value === null || value === undefined) return '';
  if (typeof value === 'object') return JSON.stringify(value);
  return String(value);
}

/** Скалярное значение для CSV (без переносов строк), иначе JSON. */
function csvValue(value: unknown): string {
  const s = formatPayloadValue(value);
  return s.replace(/\s+/g, ' ').trim();
}

/** Экранирование CSV-поля по RFC4180. */
function csvEscape(value: string): string {
  if (/[",\n]/.test(value)) return `"${value.replace(/"/g, '""')}"`;
  return value;
}

/** CSV-файл сабмитов: колонки — поля формы + дата + id. BOM добавляется при скачивании. */
export function buildSubmissionsCsv(
  fieldNames: string[],
  submissions: Array<{ id: string; createdAt: string; payload?: Record<string, unknown> | undefined }>,
  labels: { date: string; id: string },
): string {
  const header = [...fieldNames, labels.date, labels.id];
  const rows = submissions.map((s) => [
    ...fieldNames.map((name) => csvEscape(csvValue(s.payload?.[name]))),
    csvEscape(csvValue(s.createdAt)),
    csvEscape(s.id),
  ]);
  return [header.map(csvEscape).join(','), ...rows.map((r) => r.join(','))].join('\n');
}

/** Исполнители полей формы: subset контракта docs/ui-runtime/json-descriptors.md §10. */
const FIELD_TYPES = ['text', 'email', 'password', 'number', 'select', 'checkbox', 'textarea', 'custom'] as const;

/** «Contact Form» → form.contact; не-латиница/пусто → form.form. */
export function slugifyFormId(name: string): string {
  const slug = name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '.')
    .replace(/\.{2,}/g, '.')
    .replace(/^\.+|\.+$/g, '');
  return slug ? `form.${slug}` : 'form.form';
}

/** Дефолтное определение: пустые поля, отправка на endpoint формы (как в примере api-admin §Формы). */
export function createDefaultDefinition(id: string): FormDefinition {
  return { id, fields: [], submit: { endpoint: id } };
}

export type DefinitionValidation =
  | { ok: true; value: FormDefinition }
  | { ok: false; error: string };

/** Валидация raw-JSON определения формы по схеме §10 (поля не переусложняем — бэкенд хранит as-is). */
export function validateDefinition(text: string): DefinitionValidation {
  let raw: unknown;
  try {
    raw = JSON.parse(text);
  } catch (e) {
    return { ok: false, error: e instanceof Error ? e.message : String(e) };
  }
  if (typeof raw !== 'object' || raw === null || Array.isArray(raw)) {
    return { ok: false, error: 'определение должно быть объектом JSON' };
  }
  const def = raw as Record<string, unknown>;
  const id = def.id;
  if (typeof id !== 'string' || id.trim() === '') {
    return { ok: false, error: 'поле id — непустая строка' };
  }
  if (def.fields !== undefined && !Array.isArray(def.fields)) {
    return { ok: false, error: 'поле fields — массив' };
  }
  const fields = (def.fields ?? []) as unknown[];
  for (let i = 0; i < fields.length; i++) {
    const field = fields[i];
    if (typeof field !== 'object' || field === null) {
      return { ok: false, error: `fields[${i}] — объект` };
    }
    const f = field as Record<string, unknown>;
    if (typeof f.name !== 'string' || f.name.trim() === '') {
      return { ok: false, error: `fields[${i}].name — непустая строка` };
    }
    if (typeof f.type !== 'string' || !FIELD_TYPES.includes(f.type as (typeof FIELD_TYPES)[number])) {
      return { ok: false, error: `fields[${i}].type — одно из: ${FIELD_TYPES.join(', ')}` };
    }
  }
  const submit = def.submit;
  if (typeof submit !== 'object' || submit === null) {
    return { ok: false, error: 'поле submit — объект' };
  }
  const s = submit as Record<string, unknown>;
  const target = s.target ?? s.endpoint;
  if (typeof target !== 'string' || target.trim() === '') {
    return { ok: false, error: 'submit.target (или submit.endpoint) — непустая строка' };
  }
  return { ok: true, value: def as unknown as FormDefinition };
}