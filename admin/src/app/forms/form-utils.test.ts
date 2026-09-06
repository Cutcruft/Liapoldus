import { describe, expect, it } from 'vitest';
import { createDefaultDefinition, slugifyFormId, validateDefinition } from './form-utils';

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