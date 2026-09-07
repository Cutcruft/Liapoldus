import { describe, expect, it } from 'vitest';
import { buildSubmissionsCsv, createDefaultDefinition, formatPayloadValue, slugifyFormId, validateDefinition } from './form-utils';

describe('slugifyFormId', () => {
  it('латиница → form.<slug>', () => {
    expect(slugifyFormId('Contact')).toBe('form.contact');
  });

  it('пробелы в доты', () => {
    expect(slugifyFormId('Contact Form')).toBe('form.contact.form');
  });

  it('лишние разделители схлопываются', () => {
    expect(slugifyFormId('  Feedback   Form  ')).toBe('form.feedback.form');
  });

  it('не-латиница/пусто → form.form', () => {
    expect(slugifyFormId('Подписка')).toBe('form.form');
    expect(slugifyFormId('   ')).toBe('form.form');
  });
});

describe('createDefaultDefinition', () => {
  it('пустые поля + submit.endpoint', () => {
    expect(createDefaultDefinition('form.contact')).toEqual({
      id: 'form.contact',
      fields: [],
      submit: { endpoint: 'form.contact' },
    });
  });
});

describe('validateDefinition', () => {
  const VALID = {
    id: 'form.contact',
    fields: [{ name: 'email', type: 'email', required: true }],
    submit: { target: 'endpoint.form.submit', providerId: 'liapoldus.builtin' },
  };

  it('принимает валидную схему §10', () => {
    const res = validateDefinition(JSON.stringify(VALID));
    expect(res.ok).toBe(true);
    if (res.ok) expect(res.value.id).toBe('form.contact');
  });

  it('принимает submit.endpoint (api-admin)', () => {
    const res = validateDefinition(JSON.stringify({ id: 'form.x', fields: [], submit: { endpoint: 'form.x' } }));
    expect(res.ok).toBe(true);
  });

  it('битый JSON → ошибка', () => {
    const res = validateDefinition('{id:');
    expect(res.ok).toBe(false);
    if (!res.ok) expect(res.error.length).toBeGreaterThan(0);
  });

  it('не-объект → ошибка', () => {
    expect(validateDefinition('[1,2]').ok).toBe(false);
    expect(validateDefinition('"str"').ok).toBe(false);
  });

  it('нет id → ошибка', () => {
    const res = validateDefinition(JSON.stringify({ fields: [] }));
    expect(res.ok).toBe(false);
  });

  it('неверный тип поля → ошибка', () => {
    const bad = { ...VALID, fields: [{ name: 'x', type: 'nope' }] };
    const res = validateDefinition(JSON.stringify(bad));
    expect(res.ok).toBe(false);
  });

  it('нет submit → ошибка', () => {
    const res = validateDefinition(JSON.stringify({ id: 'form.x', fields: [] }));
    expect(res.ok).toBe(false);
  });

  it('submit без target/endpoint → ошибка', () => {
    const res = validateDefinition(JSON.stringify({ id: 'form.x', fields: [], submit: {} }));
    expect(res.ok).toBe(false);
  });

  it('fields опционален (пусто = [])', () => {
    const res = validateDefinition(JSON.stringify({ id: 'form.x', submit: { target: 't' } }));
    expect(res.ok).toBe(true);
  });
});

describe('formatPayloadValue', () => {
  it('скаляры как есть, объекты/массивы — JSON', () => {
    expect(formatPayloadValue('text')).toBe('text');
    expect(formatPayloadValue(7)).toBe('7');
    expect(formatPayloadValue(true)).toBe('true');
    expect(formatPayloadValue(null)).toBe('');
    expect(formatPayloadValue(undefined)).toBe('');
    expect(formatPayloadValue({ a: 1 })).toBe('{"a":1}');
  });
});

describe('buildSubmissionsCsv', () => {
  const S = (payload: Record<string, unknown>, createdAt = '2026-01-01T00:00:00Z', id = 's1') => ({
    id,
    createdAt,
    payload,
  });

  it('заголовок: поля формы + дата + id', () => {
    const csv = buildSubmissionsCsv(['email', 'city'], [S({ email: 'a@b.c' })], { date: 'Дата', id: 'ID' });
    expect(csv.split('\n')[0]).toBe('email,city,Дата,ID');
    expect(csv.split('\n')[1]).toBe('a@b.c,,2026-01-01T00:00:00Z,s1');
  });

  it('экранирует запятые и кавычки по RFC4180', () => {
    const csv = buildSubmissionsCsv(['note'], [S({ note: 'a,"b"' })], { date: 'Дата', id: 'ID' });
    expect(csv.split('\n')[1]).toBe('"a,""b""",2026-01-01T00:00:00Z,s1');
  });

  it('объектные значения — JSON, экранированы как CSV-поле', () => {
    const csv = buildSubmissionsCsv(['meta'], [S({ meta: { x: 1 } })], { date: 'Дата', id: 'ID' });
    expect(csv.split('\n')[1]).toBe('"{""x"":1}",2026-01-01T00:00:00Z,s1');
  });
});