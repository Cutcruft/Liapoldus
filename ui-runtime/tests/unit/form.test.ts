import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { ApiClient } from '../../src/core/api-client';
import { registerBuiltin } from '../../src/core/builtin/descriptors';
import { FormRuntime } from '../../src/core/form';
import { createRuntimeStore } from '../../src/core/store';
import { RuntimeRegistry } from '../../src/core/registry';
import { TransportFactory } from '../../src/core/transport/factory';
import { FormValidationError } from '../../src/errors';
import { makeFakeFetch, resetFakes } from './helpers';

const BUILTIN = 'liapoldus.builtin';

const formFix = {
  id: 'form.contact',
  fields: [
    { name: 'email', type: 'email', required: true },
    { name: 'password', type: 'password', required: true, rules: [{ id: 'minLength', value: 5 }] },
    { name: 'password2', type: 'password', required: true },
    { name: 'theme', type: 'select', required: false, options: ['general', 'bug'] },
    { name: 'message', type: 'textarea', required: false, rules: [{ id: 'minLength', value: 10 }] },
  ],
  validation: [{ id: 'confirmMatch', fields: ['password', 'password2'] }],
  submit: { target: 'endpoint.form.submit' },
};

const validValues = {
  email: 'user@example.com',
  password: 'secret5',
  password2: 'secret5',
  theme: 'general',
  message: 'Достаточно длинное сообщение',
};

function buildBackend() {
  return makeFakeFetch((call) => {
    const url = new URL(call.url, 'http://runtime.local');
    const path = url.pathname;
    const method = call.init.method ?? 'GET';

    if (path === '/api/forms/form.contact' && method === 'GET') return formFix;

    if (path === '/api/forms/form.contact/submissions' && method === 'POST') {
      return { submissionId: 'sub-1', state: 'ok' };
    }

    return null;
  });
}

function serverApi(registry: RuntimeRegistry, fetch: ReturnType<typeof makeFakeFetch>['fetch']) {
  const transport = new TransportFactory({ fetch }).create(registry.getProvider(BUILTIN)!);
  return new ApiClient(registry, { transport, scope: 'server' });
}

afterEach(() => resetFakes());
beforeEach(() => resetFakes());

describe('form runtime', () => {
  function setup() {
    const registry = new RuntimeRegistry();
    registerBuiltin(registry);
    const { fetch, calls } = buildBackend();
    const store = createRuntimeStore();
    store.getState().setLocale('ru');
    const api = serverApi(registry, fetch);
    const runtime = new FormRuntime(store, api, { submittedAt: () => '2026-09-05T12:00:00Z' });
    return { store, api, runtime, calls };
  }

  it('load() → определение формы и кэш в store.forms', async () => {
    const { store, runtime } = setup();
    const def = await runtime.load('form.contact');

    expect(def.id).toBe('form.contact');
    expect(def.fields).toHaveLength(5);
    expect(store.getState().forms['form.contact'].definition).toBe(def);
  });

  it('fields() → FieldSchema[]', async () => {
    const { runtime } = setup();
    await runtime.load('form.contact');

    const fields = runtime.fields('form.contact');
    expect(fields.map((f) => f.name)).toEqual(['email', 'password', 'password2', 'theme', 'message']);
  });

  it('validate(\u2026) валидные значения → { valid: true, errors: [] }', async () => {
    const { runtime } = setup();
    await runtime.load('form.contact');

    expect(runtime.validate('form.contact', validValues)).toEqual({ valid: true, errors: [] });
  });

  it('validate: пустое required → { fieldId:"email", ruleId:"required" }', async () => {
    const { runtime } = setup();
    await runtime.load('form.contact');

    const res = runtime.validate('form.contact', { ...validValues, email: '' });
    expect(res.valid).toBe(false);
    expect(res.errors).toContainEqual(expect.objectContaining({ fieldId: 'email', ruleId: 'required' }));
  });

  it('validate: minLength → ruleId:"minLength"', async () => {
    const { runtime } = setup();
    await runtime.load('form.contact');

    const res = runtime.validate('form.contact', { ...validValues, password: '123', password2: '123' });
    expect(res.valid).toBe(false);
    expect(res.errors).toContainEqual(expect.objectContaining({ fieldId: 'password', ruleId: 'minLength' }));
  });

  it('validate: confirmMatch при расхождении паролей', async () => {
    const { runtime } = setup();
    await runtime.load('form.contact');

    const res = runtime.validate('form.contact', { ...validValues, password2: 'different' });
    expect(res.valid).toBe(false);
    expect(res.errors).toContainEqual(expect.objectContaining({ fieldId: 'password2', ruleId: 'confirmMatch' }));
  });

  it('validate: неверный email → ruleId:"email"', async () => {
    const { runtime } = setup();
    await runtime.load('form.contact');

    const res = runtime.validate('form.contact', { ...validValues, email: 'not-an-email' });
    expect(res.valid).toBe(false);
    expect(res.errors).toContainEqual(expect.objectContaining({ fieldId: 'email', ruleId: 'email' }));
  });

  it('submit с невалидными значениями → FormValidationError, запрос не уходит', async () => {
    const { runtime, calls } = setup();
    await runtime.load('form.contact');

    await expect(runtime.submit('form.contact', { ...validValues, email: '' })).rejects.toThrow(FormValidationError);
    expect(calls.filter((c) => c.url.endsWith('/submissions'))).toHaveLength(0);
  });

  it('submit → callEndpoint(form.submit) с payload {formId, locale, submittedAt, values} (raw JSON)', async () => {
    const { runtime, calls } = setup();
    await runtime.load('form.contact');

    await runtime.submit('form.contact', validValues);

    const subCalls = calls.filter((c) => c.url.endsWith('/submissions'));
    expect(subCalls).toHaveLength(1);
    const body = JSON.parse(String(subCalls[0].init.body)) as Record<string, unknown>;
    expect(Object.keys(body).sort()).toEqual(['formId', 'locale', 'submittedAt', 'values']);
    expect(body).toMatchObject({
      formId: 'form.contact',
      locale: 'ru',
      submittedAt: '2026-09-05T12:00:00Z',
    });
    expect((body.values as Record<string, unknown>).email).toBe('user@example.com');
  });

  it('submit успешный → { submissionId }, статус формы "submitted"', async () => {
    const { store, runtime } = setup();
    await runtime.load('form.contact');

    const res = await runtime.submit('form.contact', validValues);

    expect(res).toMatchObject({ submissionId: 'sub-1' });
    expect(store.getState().forms['form.contact'].status).toBe('submitted');
  });

  it('reset → status idle, пустые values/errors', async () => {
    const { store, runtime } = setup();
    await runtime.load('form.contact');
    await runtime.submit('form.contact', validValues);

    runtime.reset('form.contact');

    const snapshot = store.getState().forms['form.contact'];
    expect(snapshot.status).toBe('idle');
    expect(snapshot.values).toEqual({});
    expect(snapshot.errors).toEqual({});
  });

  it('form.get и form.submit зарегистрированы как builtin при boot', () => {
    const registry = new RuntimeRegistry();
    registerBuiltin(registry);

    expect(registry.hasOperation('form.get')).toBe(true);
    expect(registry.hasOperation('form.submit')).toBe(true);
    expect(registry.hasEndpoint('form.submit')).toBe(true);
  });
});